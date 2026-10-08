package primer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ModelRequest is the subset of the host's HostModelExecutionRequest the
// plugin sets; JSON tags match the host wire format.
type ModelRequest struct {
	EntryProtocol string `json:"entry_protocol"`
	ExitProtocol  string `json:"exit_protocol"`
	Model         string `json:"model"`
	Stream        bool   `json:"stream"`
	Body          []byte `json:"body"`
	// AuthID pins execution to one credential; the host does not fail over.
	AuthID string `json:"auth_id,omitempty"`
}

// ModelResponse is the host's HostModelExecutionResponse without headers.
type ModelResponse struct {
	StatusCode int    `json:"status_code"`
	Body       []byte `json:"body"`
}

// ExecuteFunc runs a model request through the host executor. On failure it
// returns the error and, when the host reports one, the HTTP status.
type ExecuteFunc func(context.Context, ModelRequest) (ModelResponse, error)

// LogFunc writes one structured log line; level is debug, info or warn.
type LogFunc func(level, message string, fields map[string]any)

// Primer runs warm-up rounds. It keeps no state between rounds: every
// decision comes from the latest upstream quota query.
type Primer struct {
	Config  Config
	Client  *http.Client
	Execute ExecuteFunc
	Log     LogFunc
}

const (
	decisionSkipped     = "skipped"
	decisionQueryFailed = "query_failed"
	decisionUnknown     = "unknown"
	decisionExhausted   = "exhausted"
	decisionStarted     = "started"
	decisionNotStarted  = "not_started"
)

type account struct {
	ID          string `json:"id"`
	AuthIndex   string `json:"auth_index"`
	Provider    string `json:"provider"`
	AccountType string `json:"account_type"`
	Disabled    bool   `json:"disabled"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	IDToken     struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"id_token"`
}

// verdict is the judgement of one quota query.
type verdict struct {
	decision string
	reason   string
	// windows holds the raw window fields, logged only for unknown verdicts.
	windows string
	// resetPending marks a Claude five_hour window whose resets_at is null.
	resetPending bool
}

// Run executes one warm-up round: list accounts, then query, judge and
// warm up each eligible account serially. It returns an error only when the
// account list cannot be read; per-account failures are logged and skipped.
func (p *Primer) Run(ctx context.Context) error {
	var list struct {
		Files []account `json:"files"`
	}
	if err := p.management(ctx, http.MethodGet, "/v0/management/auth-files", nil, &list); err != nil {
		return err
	}
	for _, a := range list.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		p.prime(ctx, a)
	}
	return nil
}

func (p *Primer) prime(ctx context.Context, a account) {
	fields := map[string]any{"provider": a.Provider, "account": a.Email}
	if a.Email == "" {
		fields["account"] = a.Name
	}
	result := func(level string, v verdict) {
		fields["decision"], fields["reason"] = v.decision, v.reason
		if v.decision == decisionUnknown {
			fields["windows"] = v.windows
		}
		p.Log(level, "quota primer account result", fields)
	}

	var settings Provider
	switch a.Provider {
	case "codex":
		settings = p.Config.Codex
	case "claude":
		settings = p.Config.Claude
	default:
		result("debug", verdict{decision: decisionSkipped, reason: "unsupported provider"})
		return
	}
	switch {
	case !settings.Enabled:
		result("info", verdict{decision: decisionSkipped, reason: "provider disabled in plugin config"})
		return
	case a.AccountType != "oauth":
		result("info", verdict{decision: decisionSkipped, reason: "not an OAuth subscription account"})
		return
	case a.Disabled:
		result("info", verdict{decision: decisionSkipped, reason: "account disabled"})
		return
	}

	v := p.check(ctx, a)
	switch v.decision {
	case decisionNotStarted:
	case decisionUnknown, decisionQueryFailed:
		result("warn", v)
		return
	default:
		result("info", v)
		return
	}

	model := settings.Model
	if model == "" {
		var err error
		if model, err = p.firstModel(ctx, a.Name); err != nil {
			fields["warmup_error"] = err.Error()
			result("warn", v)
			return
		}
	}
	fields["model"] = model
	resp, err := p.Execute(ctx, warmupRequest(a, model))
	fields["warmup_status"] = resp.StatusCode
	if err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
		if err != nil {
			fields["warmup_error"] = truncate(err.Error())
		} else {
			fields["warmup_error"] = truncate(string(resp.Body))
		}
		result("warn", v)
		return
	}
	if a.Provider == "claude" {
		// Codex is not rechecked: reset_after_seconds stays at the full window
		// for 1-3 seconds after a request and would report a false failure.
		if after := p.check(ctx, a); after.decision == decisionQueryFailed {
			fields["recheck"] = "quota recheck failed: " + after.reason
			result("warn", v)
			return
		} else if after.resetPending {
			fields["recheck"] = "five_hour.resets_at still null after warm-up; next round retries"
			result("warn", v)
			return
		}
		fields["recheck"] = "five_hour window started"
	}
	result("info", v)
}

func (p *Primer) check(ctx context.Context, a account) verdict {
	if a.Provider == "codex" {
		header := map[string]string{
			"Authorization": "Bearer $TOKEN$",
			"Accept":        "application/json",
			"User-Agent":    "codex_cli_rs/0.76.0",
		}
		if a.IDToken.ChatGPTAccountID != "" {
			header["Chatgpt-Account-Id"] = a.IDToken.ChatGPTAccountID
		}
		body, err := p.upstream(ctx, a.AuthIndex, "https://chatgpt.com/backend-api/wham/usage", header)
		if err != nil {
			return verdict{decision: decisionQueryFailed, reason: err.Error()}
		}
		return judgeCodex(body)
	}
	body, err := p.upstream(ctx, a.AuthIndex, "https://api.anthropic.com/api/oauth/usage", map[string]string{
		"Authorization":  "Bearer $TOKEN$",
		"anthropic-beta": "oauth-2025-04-20",
		"Accept":         "application/json",
		"User-Agent":     "claude-cli/2.1.0 (external, cli)",
	})
	if err != nil {
		return verdict{decision: decisionQueryFailed, reason: err.Error()}
	}
	return judgeClaude(body)
}

// upstream performs a read-only GET as the account through the host's
// api-call bridge, which substitutes $TOKEN$; the token never reaches the plugin.
func (p *Primer) upstream(ctx context.Context, authIndex, target string, header map[string]string) ([]byte, error) {
	request := map[string]any{"auth_index": authIndex, "method": http.MethodGet, "url": target, "header": header}
	var response struct {
		StatusCode int    `json:"status_code"`
		Body       string `json:"body"`
	}
	if err := p.management(ctx, http.MethodPost, "/v0/management/api-call", request, &response); err != nil {
		return nil, err
	}
	// Upstream bodies may carry account details; never log them.
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream quota query returned HTTP %d", response.StatusCode)
	}
	if !json.Valid([]byte(response.Body)) {
		return nil, fmt.Errorf("upstream quota response is not JSON")
	}
	return []byte(response.Body), nil
}

func (p *Primer) firstModel(ctx context.Context, name string) (string, error) {
	var list struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if err := p.management(ctx, http.MethodGet, "/v0/management/auth-files/models?name="+url.QueryEscape(name), nil, &list); err != nil {
		return "", fmt.Errorf("list account models: %w", err)
	}
	for _, m := range list.Models {
		if m.ID != "" && !strings.Contains(strings.ToLower(m.ID), "image") {
			return m.ID, nil
		}
	}
	return "", fmt.Errorf("account has no non-image model; set providers.<provider>.model")
}

// management calls the host management API. Transport failures are returned
// unwrapped (*url.Error) so callers can tell "not listening yet" apart.
func (p *Primer) management(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.Config.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.Config.Key)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	endpoint, _, _ := strings.Cut(path, "?")
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("management %s %s returned HTTP %d", method, endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("management %s %s returned an unreadable body", method, endpoint)
	}
	return nil
}

func warmupRequest(a account, model string) ModelRequest {
	req := ModelRequest{Model: model, AuthID: a.ID}
	if a.Provider == "codex" {
		req.EntryProtocol = "openai-response"
		// The host strips max_output_tokens for Codex, so none is sent.
		req.Body, _ = json.Marshal(map[string]any{"model": model, "input": "hi", "stream": false, "store": false})
	} else {
		req.EntryProtocol = "claude"
		req.Body, _ = json.Marshal(map[string]any{
			"model": model, "max_tokens": 16, "stream": false,
			"messages": []map[string]string{{"role": "user", "content": "hi"}},
		})
	}
	return req
}

func truncate(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
