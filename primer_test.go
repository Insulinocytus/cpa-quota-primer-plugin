package primer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	primer "github.com/Insulinocytus/cpa-quota-primer-plugin"
)

const (
	managementKey = "secret-management-key"
	accountToken  = "secret-account-token"
)

// reply is one api-call answer: outer management status, upstream status, upstream body.
type reply struct {
	outer, status int
	body          string
}

func ok(body string) reply { return reply{http.StatusOK, http.StatusOK, body} }

// fakeCPA imitates the CLIProxyAPI management API (field shapes from the
// pinned host's auth_files.go and api_tools.go).
type fakeCPA struct {
	mu       sync.Mutex
	accounts []map[string]any
	usage    map[string][]reply // by auth_index; the last reply repeats
	models   map[string][]string
	apiCalls []map[string]any
}

func newCPA(accounts ...map[string]any) *fakeCPA {
	return &fakeCPA{accounts: accounts, usage: map[string][]reply{}, models: map[string][]string{}}
}

func oauth(provider, name string) map[string]any {
	return map[string]any{
		"id": "id-" + name, "auth_index": "idx-" + name, "name": name + ".json", "email": name + "@example.com",
		"provider": provider, "type": provider, "account_type": "oauth", "disabled": false,
	}
}

func (f *fakeCPA) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+managementKey {
		http.Error(w, `{"error":"invalid management key"}`, http.StatusUnauthorized)
		return
	}
	switch r.URL.Path {
	case "/v0/management/auth-files":
		json.NewEncoder(w).Encode(map[string]any{"files": f.accounts})
	case "/v0/management/auth-files/models":
		models := []map[string]string{}
		for _, id := range f.models[r.URL.Query().Get("name")] {
			models = append(models, map[string]string{"id": id})
		}
		json.NewEncoder(w).Encode(map[string]any{"models": models})
	case "/v0/management/api-call":
		var call map[string]any
		json.NewDecoder(r.Body).Decode(&call)
		f.apiCalls = append(f.apiCalls, call)
		index, _ := call["auth_index"].(string)
		replies := f.usage[index]
		if len(replies) == 0 {
			http.Error(w, `{"error":"unknown auth"}`, http.StatusBadRequest)
			return
		}
		next := replies[0]
		if len(replies) > 1 {
			f.usage[index] = replies[1:]
		}
		if next.outer != http.StatusOK {
			http.Error(w, `{"error":"request failed"}`, next.outer)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status_code": next.status, "header": map[string]any{}, "body": next.body})
	default:
		http.NotFound(w, r)
	}
}

type logLine struct {
	Level, Message string
	Fields         map[string]any
}

type outcome struct {
	err    error
	warmed []primer.ModelRequest
	logs   []logLine
}

// account returns the result line logged for an account.
func (o outcome) account(t *testing.T, name string) map[string]any {
	t.Helper()
	for _, l := range o.logs {
		if l.Fields["account"] == name+"@example.com" {
			return l.Fields
		}
	}
	t.Fatalf("no log line for %s in %+v", name, o.logs)
	return nil
}

func (o outcome) level(name string) string {
	for _, l := range o.logs {
		if l.Fields["account"] == name+"@example.com" {
			return l.Level
		}
	}
	return ""
}

type executor func(primer.ModelRequest) (primer.ModelResponse, error)

func succeed(primer.ModelRequest) (primer.ModelResponse, error) {
	return primer.ModelResponse{StatusCode: http.StatusOK, Body: []byte(`{}`)}, nil
}

func run(t *testing.T, cpa *fakeCPA, providers string, exec executor) outcome {
	t.Helper()
	server := httptest.NewServer(cpa)
	defer server.Close()
	cfg, err := primer.ParseConfig([]byte(fmt.Sprintf("cron: \"30 8 * * *\"\nmanagement: {base_url: %q, key: %q}\nproviders: %s\n", server.URL, managementKey, providers)))
	if err != nil {
		t.Fatal(err)
	}
	var o outcome
	p := primer.Primer{
		Config: cfg,
		Client: server.Client(),
		Execute: func(_ context.Context, req primer.ModelRequest) (primer.ModelResponse, error) {
			o.warmed = append(o.warmed, req)
			return exec(req)
		},
		Log: func(level, message string, fields map[string]any) {
			o.logs = append(o.logs, logLine{level, message, fields})
		},
	}
	o.err = p.Run(context.Background())
	return o
}

const bothEnabled = "{codex: {enabled: true}, claude: {enabled: true}}"

func window(used, limit, resetAfter int) string {
	return fmt.Sprintf(`{"used_percent":%d,"limit_window_seconds":%d,"reset_after_seconds":%d,"reset_at":1791483563}`, used, limit, resetAfter)
}

func codexUsage(primary, secondary string) string {
	return fmt.Sprintf(`{"plan_type":"plus","rate_limit":{"allowed":true,"primary_window":%s,"secondary_window":%s}}`, primary, secondary)
}

func TestRoundCodexWindowJudgement(t *testing.T) {
	for _, tc := range []struct {
		name, usage, decision string
	}{
		{"five hour not started", codexUsage(window(0, 18000, 18000), window(20, 604800, 300000)), "not_started"},
		{"weekly not started", codexUsage(window(5, 18000, 12000), window(0, 604800, 604800)), "not_started"},
		{"started at zero percent", codexUsage(window(0, 18000, 16840), window(20, 604800, 300000)), "started"},
		{"tolerance edge 10s", codexUsage(window(0, 18000, 17990), window(20, 604800, 300000)), "not_started"},
		{"beyond tolerance 11s", codexUsage(window(0, 18000, 17989), window(20, 604800, 300000)), "started"},
		{"slot names ignored", codexUsage(window(0, 604800, 604800), window(30, 18000, 9000)), "not_started"},
		{"exhausted window blocks idle window", codexUsage(window(0, 18000, 18000), window(100, 604800, 7000)), "exhausted"},
		{"null slot ignored for pro plan", codexUsage("null", window(0, 604800, 604799)), "not_started"},
		{"unknown window length is not a target", codexUsage(window(0, 2592000, 2592000), "null"), "started"},
		{"unknown length still counts when exhausted", codexUsage(window(0, 18000, 18000), window(100, 2592000, 50)), "exhausted"},
		{"no window reported", codexUsage("null", "null"), "unknown"},
		{"window field missing", codexUsage(`{"used_percent":0,"limit_window_seconds":18000}`, "null"), "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpa := newCPA(oauth("codex", "a"))
			cpa.usage["idx-a"] = []reply{ok(tc.usage)}
			o := run(t, cpa, "{codex: {enabled: true, model: gpt-5.5}}", succeed)
			if o.err != nil {
				t.Fatal(o.err)
			}
			if got := o.account(t, "a")["decision"]; got != tc.decision {
				t.Fatalf("decision = %v, want %s", got, tc.decision)
			}
			if want := tc.decision == "not_started"; (len(o.warmed) == 1) != want {
				t.Fatalf("warm-up requests = %+v, want sent=%v", o.warmed, want)
			}
		})
	}
}

func TestRoundCodexWarmupIsPinnedMinimalRequest(t *testing.T) {
	cpa := newCPA(oauth("codex", "a"))
	cpa.accounts[0]["id_token"] = map[string]any{"chatgpt_account_id": "acct-1", "plan_type": "plus"}
	cpa.usage["idx-a"] = []reply{ok(codexUsage(window(0, 18000, 18000), "null"))}
	o := run(t, cpa, "{codex: {enabled: true, model: gpt-5.5}}", succeed)
	if len(o.warmed) != 1 {
		t.Fatalf("warm-ups = %+v", o.warmed)
	}
	req := o.warmed[0]
	if req.AuthID != "id-a" || req.Model != "gpt-5.5" || req.EntryProtocol != "openai-response" || req.Stream {
		t.Fatalf("request = %+v", req)
	}
	var body map[string]any
	if json.Unmarshal(req.Body, &body) != nil || body["input"] != "hi" || body["stream"] != false {
		t.Fatalf("body = %s", req.Body)
	}
	header := cpa.apiCalls[0]["header"].(map[string]any)
	if header["Chatgpt-Account-Id"] != "acct-1" || header["Authorization"] != "Bearer $TOKEN$" {
		t.Fatalf("usage query header = %v", header)
	}
	// Codex is not rechecked after a warm-up.
	if len(cpa.apiCalls) != 1 {
		t.Fatalf("api calls = %d, want 1", len(cpa.apiCalls))
	}
	if o.level("a") != "info" || o.account(t, "a")["warmup_status"] != 200 {
		t.Fatalf("log = %s %v", o.level("a"), o.account(t, "a"))
	}
}

func TestRoundUnknownLogsRawWindows(t *testing.T) {
	cpa := newCPA(oauth("codex", "a"), oauth("claude", "b"))
	cpa.usage["idx-a"] = []reply{ok(codexUsage(`{"used_percent":0,"limit_window_seconds":18000}`, "null"))}
	cpa.usage["idx-b"] = []reply{ok(`{"seven_day":{"utilization":3,"resets_at":"2026-10-12T00:00:00+00:00"}}`)}
	o := run(t, cpa, bothEnabled, succeed)
	codex, _ := o.account(t, "a")["windows"].(string)
	if !strings.Contains(codex, `"limit_window_seconds":18000`) || o.level("a") != "warn" {
		t.Fatalf("codex windows = %q", codex)
	}
	claude, _ := o.account(t, "b")["windows"].(string)
	if !strings.Contains(claude, `"utilization":3`) || o.level("b") != "warn" {
		t.Fatalf("claude windows = %q", claude)
	}
}

func claudeUsage(fiveHour, sevenDay string) string {
	return fmt.Sprintf(`{"five_hour":%s,"seven_day":%s,"extra_usage":{"is_enabled":false}}`, fiveHour, sevenDay)
}

const (
	claudeIdle    = `{"utilization":0.0,"resets_at":null}`
	claudeRunning = `{"utilization":0.0,"resets_at":"2026-10-08T17:30:00.288669+00:00"}`
	claudeWeek    = `{"utilization":12.0,"resets_at":"2026-10-12T03:00:00.288669+00:00"}`
)

func TestRoundClaudeWindowJudgement(t *testing.T) {
	for _, tc := range []struct {
		name, usage, decision string
	}{
		{"idle five hour", claudeUsage(claudeIdle, claudeWeek), "not_started"},
		{"future resets_at", claudeUsage(claudeRunning, claudeWeek), "started"},
		{"used with null resets_at", claudeUsage(`{"utilization":4.0,"resets_at":null}`, claudeWeek), "started"},
		{"seven day exhausted", claudeUsage(claudeIdle, `{"utilization":100.0,"resets_at":"2026-10-12T03:00:00+00:00"}`), "exhausted"},
		{"five hour exhausted", claudeUsage(`{"utilization":100.0,"resets_at":"2026-10-08T17:30:00+00:00"}`, claudeWeek), "exhausted"},
		{"seven day absent", claudeUsage(claudeIdle, "null"), "not_started"},
		{"five hour missing", `{"seven_day":` + claudeWeek + `}`, "unknown"},
		{"five hour null", claudeUsage("null", claudeWeek), "unknown"},
		{"resets_at unparsable", claudeUsage(`{"utilization":0.0,"resets_at":"soon"}`, claudeWeek), "unknown"},
		{"resets_at missing", claudeUsage(`{"utilization":0.0}`, claudeWeek), "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpa := newCPA(oauth("claude", "a"))
			cpa.usage["idx-a"] = []reply{ok(tc.usage), ok(claudeUsage(claudeRunning, claudeWeek))}
			o := run(t, cpa, "{claude: {enabled: true, model: claude-haiku-4-5}}", succeed)
			if got := o.account(t, "a")["decision"]; got != tc.decision {
				t.Fatalf("decision = %v, want %s", got, tc.decision)
			}
			if want := tc.decision == "not_started"; (len(o.warmed) == 1) != want {
				t.Fatalf("warm-up requests = %+v, want sent=%v", o.warmed, want)
			}
		})
	}
}

func TestRoundClaudeRecheck(t *testing.T) {
	for _, tc := range []struct {
		name, after, level, recheck string
	}{
		{"window started", claudeUsage(claudeRunning, claudeWeek), "info", "five_hour window started"},
		{"still null", claudeUsage(claudeIdle, claudeWeek), "warn", "still null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpa := newCPA(oauth("claude", "a"))
			cpa.usage["idx-a"] = []reply{ok(claudeUsage(claudeIdle, claudeWeek)), ok(tc.after)}
			o := run(t, cpa, "{claude: {enabled: true, model: claude-haiku-4-5}}", succeed)
			if len(o.warmed) != 1 {
				t.Fatalf("warm-ups = %+v", o.warmed)
			}
			req := o.warmed[0]
			var body struct {
				MaxTokens int `json:"max_tokens"`
			}
			if req.AuthID != "id-a" || req.EntryProtocol != "claude" || req.Stream || json.Unmarshal(req.Body, &body) != nil || body.MaxTokens <= 0 || body.MaxTokens > 64 {
				t.Fatalf("request = %+v body=%s", req, req.Body)
			}
			fields := o.account(t, "a")
			if recheck, _ := fields["recheck"].(string); !strings.Contains(recheck, tc.recheck) || o.level("a") != tc.level {
				t.Fatalf("log = %s %v", o.level("a"), fields)
			}
		})
	}
}

func TestRoundSkipsIneligibleAccounts(t *testing.T) {
	apiKey := oauth("codex", "apikey")
	apiKey["account_type"] = "api_key"
	disabled := oauth("codex", "disabled")
	disabled["disabled"] = true
	cpa := newCPA(apiKey, disabled, oauth("claude", "switched-off"), oauth("gemini", "other"), oauth("codex", "healthy"))
	idle := ok(codexUsage(window(0, 18000, 18000), "null"))
	for _, name := range []string{"apikey", "disabled", "healthy"} {
		cpa.usage["idx-"+name] = []reply{idle}
	}
	cpa.usage["idx-switched-off"] = []reply{ok(claudeUsage(claudeIdle, claudeWeek))}
	o := run(t, cpa, "{codex: {enabled: true, model: gpt-5.5}, claude: {enabled: false}}", succeed)
	if len(o.warmed) != 1 || o.warmed[0].AuthID != "id-healthy" {
		t.Fatalf("warm-ups = %+v", o.warmed)
	}
	for _, name := range []string{"apikey", "disabled", "switched-off", "other"} {
		if fields := o.account(t, name); fields["decision"] != "skipped" || fields["reason"] == "" {
			t.Fatalf("%s: %v", name, fields)
		}
	}
	if len(cpa.apiCalls) != 1 {
		t.Fatalf("quota queries = %d, want only the healthy account", len(cpa.apiCalls))
	}
}

func TestRoundQueryFailuresSkipAccount(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply reply
	}{
		{"api-call outer failure", reply{http.StatusBadGateway, 0, ""}},
		{"upstream non-200", reply{http.StatusOK, http.StatusUnauthorized, `{"error":"token ` + accountToken + ` expired"}`}},
		{"body not JSON", reply{http.StatusOK, http.StatusOK, "<html>" + accountToken + "</html>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpa := newCPA(oauth("codex", "a"), oauth("codex", "healthy"))
			cpa.usage["idx-a"] = []reply{tc.reply}
			cpa.usage["idx-healthy"] = []reply{ok(codexUsage(window(0, 18000, 18000), "null"))}
			o := run(t, cpa, "{codex: {enabled: true, model: gpt-5.5}}", succeed)
			if o.err != nil {
				t.Fatal(o.err)
			}
			if fields := o.account(t, "a"); fields["decision"] != "query_failed" || o.level("a") != "warn" {
				t.Fatalf("log = %v", fields)
			}
			if len(o.warmed) != 1 || o.warmed[0].AuthID != "id-healthy" {
				t.Fatalf("warm-ups = %+v", o.warmed)
			}
			raw, _ := json.Marshal(o.logs)
			if strings.Contains(string(raw), accountToken) || strings.Contains(string(raw), managementKey) {
				t.Fatalf("secret leaked into logs: %s", raw)
			}
		})
	}
}

func TestRoundModelSelection(t *testing.T) {
	for _, tc := range []struct {
		name, providers string
		models          []string
		want            string
	}{
		{"configured model wins", "{codex: {enabled: true, model: gpt-5.4-mini}}", []string{"gpt-5.5"}, "gpt-5.4-mini"},
		{"first non-image model", "{codex: {enabled: true}}", []string{"gpt-image-2", "GPT-Image-1", "gpt-5.5", "gpt-5.4"}, "gpt-5.5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpa := newCPA(oauth("codex", "a"))
			cpa.usage["idx-a"] = []reply{ok(codexUsage(window(0, 18000, 18000), "null"))}
			cpa.models["a.json"] = tc.models
			o := run(t, cpa, tc.providers, succeed)
			if len(o.warmed) != 1 || o.warmed[0].Model != tc.want {
				t.Fatalf("warm-ups = %+v, want model %s", o.warmed, tc.want)
			}
		})
	}
}

func TestRoundNoUsableModelSkipsWarmup(t *testing.T) {
	cpa := newCPA(oauth("codex", "a"))
	cpa.usage["idx-a"] = []reply{ok(codexUsage(window(0, 18000, 18000), "null"))}
	cpa.models["a.json"] = []string{"gpt-image-2"}
	o := run(t, cpa, "{codex: {enabled: true}}", succeed)
	if len(o.warmed) != 0 || o.level("a") != "warn" || o.account(t, "a")["warmup_error"] == nil {
		t.Fatalf("warm-ups = %+v log = %v", o.warmed, o.logs)
	}
}

func TestRoundWarmupFailureIsLogged(t *testing.T) {
	cpa := newCPA(oauth("claude", "a"), oauth("claude", "b"))
	cpa.usage["idx-a"] = []reply{ok(claudeUsage(claudeIdle, claudeWeek))}
	cpa.usage["idx-b"] = []reply{ok(claudeUsage(claudeIdle, claudeWeek)), ok(claudeUsage(claudeRunning, claudeWeek))}
	o := run(t, cpa, "{claude: {enabled: true, model: claude-haiku-4-5}}", func(req primer.ModelRequest) (primer.ModelResponse, error) {
		if req.AuthID == "id-a" {
			return primer.ModelResponse{StatusCode: http.StatusTooManyRequests}, errors.New("rate limited")
		}
		return succeed(req)
	})
	fields := o.account(t, "a")
	if fields["warmup_status"] != http.StatusTooManyRequests || fields["warmup_error"] != "rate limited" || o.level("a") != "warn" {
		t.Fatalf("log = %v", fields)
	}
	if len(o.warmed) != 2 || o.level("b") != "info" {
		t.Fatalf("failure was not isolated: %+v", o.logs)
	}
}

func TestRoundFailsWhenAccountsCannotBeListed(t *testing.T) {
	server := httptest.NewServer(newCPA())
	defer server.Close()
	cfg, err := primer.ParseConfig([]byte(fmt.Sprintf("cron: \"30 8 * * *\"\nmanagement: {base_url: %q, key: wrong-key}", server.URL)))
	if err != nil {
		t.Fatal(err)
	}
	p := primer.Primer{Config: cfg, Client: server.Client()}
	if err := p.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "wrong-key") {
		t.Fatalf("err = %v", err)
	}
}
