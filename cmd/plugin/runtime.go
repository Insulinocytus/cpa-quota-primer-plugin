package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	primer "github.com/Insulinocytus/cpa-quota-primer-plugin"
)

// pluginVersion is set by release builds via -ldflags "-X main.pluginVersion=<version>".
var pluginVersion = "dev"

// startupRetries bounds how long the first round waits for the host's HTTP
// server: plugins register before the management API starts listening.
const startupRetries = 120

type runtime struct {
	mu      sync.Mutex
	raw     []byte // config YAML of the running generation
	cancel  context.CancelFunc
	done    chan struct{}
	client  *http.Client
	execute primer.ExecuteFunc
	log     primer.LogFunc
}

func newRuntime(execute primer.ExecuteFunc, log primer.LogFunc) *runtime {
	return &runtime{client: &http.Client{Timeout: 2 * time.Minute}, execute: execute, log: log}
}

// configure validates first so an invalid reconfigure leaves the running
// generation untouched; an unchanged config keeps its schedule.
func (r *runtime) configure(raw []byte) error {
	var request struct {
		ConfigYAML []byte `json:"config_yaml"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return fmt.Errorf("invalid lifecycle request: %w", err)
	}
	cfg, err := primer.ParseConfig(request.ConfigYAML)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil && bytes.Equal(r.raw, request.ConfigYAML) {
		return nil
	}
	r.stopLocked()
	ctx, cancel := context.WithCancel(context.Background())
	r.raw, r.cancel, r.done = request.ConfigYAML, cancel, make(chan struct{})
	go r.schedule(ctx, r.done, &primer.Primer{Config: cfg, Client: r.client, Execute: r.execute, Log: r.log})
	return nil
}

// schedule runs one round immediately, then one per cron occurrence. At most
// one round runs at a time; occurrences missed while stopped are not replayed.
func (r *runtime) schedule(ctx context.Context, done chan struct{}, p *primer.Primer) {
	var rounds sync.WaitGroup
	defer close(done)
	defer rounds.Wait()
	var busy atomic.Bool
	trigger := func(name string) {
		if !busy.CompareAndSwap(false, true) {
			r.log("warn", "quota primer trigger skipped: previous round still running", map[string]any{"trigger": name})
			return
		}
		rounds.Add(1)
		go func() {
			defer rounds.Done()
			defer busy.Store(false)
			r.round(ctx, p, name)
		}()
	}
	trigger("startup")
	// Advance from the previous occurrence, not from time.Now(): a timer that
	// fires slightly early (seen on Windows) would otherwise yield the same
	// occurrence again and trigger it twice.
	next := p.Config.Schedule.Next(time.Now())
	for {
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			trigger("cron")
			next = p.Config.Schedule.Next(next)
			if now := time.Now(); next.Before(now) { // host slept past occurrences: skip them
				next = p.Config.Schedule.Next(now)
			}
		}
	}
}

func (r *runtime) round(ctx context.Context, p *primer.Primer, trigger string) {
	r.log("info", "quota primer round started", map[string]any{"trigger": trigger})
	err := p.Run(ctx)
	var transport *url.Error
	for attempt := 0; trigger == "startup" && errors.As(err, &transport) && ctx.Err() == nil && attempt < startupRetries; attempt++ {
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
			err = p.Run(ctx)
		}
	}
	switch {
	case ctx.Err() != nil:
		r.log("info", "quota primer round canceled", map[string]any{"trigger": trigger})
	case err != nil:
		r.log("warn", "quota primer round failed", map[string]any{"trigger": trigger, "error": err.Error()})
	default:
		r.log("info", "quota primer round finished", map[string]any{"trigger": trigger})
	}
}

func (r *runtime) stopLocked() {
	if r.cancel != nil {
		r.cancel()
		<-r.done
		r.raw, r.cancel, r.done = nil, nil, nil
	}
}

func (r *runtime) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

func (r *runtime) handle(method string, raw []byte) []byte {
	switch method {
	case "plugin.register", "plugin.reconfigure":
		if err := r.configure(raw); err != nil {
			return rpcFailure("invalid_config", err.Error())
		}
		return rpcSuccess(registration())
	case "plugin.quiesce", "plugin.shutdown":
		r.stop()
		return rpcSuccess(struct{}{})
	case "management.register":
		// The host requires one capability; the plugin exposes no routes.
		return rpcSuccess(struct{}{})
	default:
		return rpcFailure("unknown_method", "unknown method: "+method)
	}
}

func rpcFailure(code, message string) []byte {
	raw, _ := json.Marshal(map[string]any{"ok": false, "error": map[string]string{"code": code, "message": message}})
	return raw
}

func rpcSuccess(value any) []byte {
	raw, _ := json.Marshal(map[string]any{"ok": true, "result": value})
	return raw
}

func registration() any {
	// Metadata/ConfigFields use the host's Go wire types (PascalCase keys);
	// the surrounding registration fields are snake_case.
	return map[string]any{
		"schema_version": 6,
		"metadata": map[string]any{
			"Name": primer.PluginID, "Version": pluginVersion, "Author": "Insulinocytus",
			"GitHubRepository": "https://github.com/Insulinocytus/cpa-quota-primer-plugin",
			"ConfigFields": []map[string]string{
				{"Name": "cron", "Type": "string", "Description": "Required standard five-field cron for warm-up rounds, e.g. 30 8 * * *."},
				{"Name": "timezone", "Type": "string", "Description": "IANA timezone for cron; default host local timezone, then UTC."},
				{"Name": "management", "Type": "object", "Description": "base_url (default http://127.0.0.1:8317) and key (required plaintext management key)."},
				{"Name": "providers", "Type": "object", "Description": "codex / claude: enabled switch and optional warm-up model (default: first non-image model of the account)."},
			},
		},
		"capabilities": map[string]bool{"management_api": true},
	}
}
