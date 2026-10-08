// Package primer runs warm-up rounds that start idle Codex and Claude usage
// windows through a CLIProxyAPI host.
package primer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata" // Windows hosts ship no zoneinfo database.

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

// PluginID is the plugin name and the dynamic library basename.
const PluginID = "cpa-quota-primer"

const defaultBaseURL = "http://127.0.0.1:8317"

// Config is the validated plugin configuration.
type Config struct {
	// Schedule fires in Schedule.Location.
	Schedule      *cron.SpecSchedule
	BaseURL       string
	Key           string
	Codex, Claude Provider
}

// Provider is the per-provider warm-up switch.
type Provider struct {
	Enabled bool
	Model   string
}

type rawConfig struct {
	Cron          string `yaml:"cron"`
	Timezone      string `yaml:"timezone"`
	ManagementURL string `yaml:"management_url"`
	ManagementKey string `yaml:"management_key"`
	CodexEnabled  bool   `yaml:"codex_enabled"`
	CodexModel    string `yaml:"codex_model"`
	ClaudeEnabled bool   `yaml:"claude_enabled"`
	ClaudeModel   string `yaml:"claude_model"`
	// The host writes these keys into every plugin config and only registers
	// enabled plugins, so they are accepted and ignored.
	Enabled  any `yaml:"enabled"`
	Priority any `yaml:"priority"`
	Store    any `yaml:"store"`
}

// ParseConfig decodes and validates the plugin config YAML from the host.
func ParseConfig(raw []byte) (Config, error) {
	var rc rawConfig
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&rc); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("invalid plugin config: %w", err)
	}
	location := time.Local // Go falls back to UTC when the host zone is unknown.
	if tz := strings.TrimSpace(rc.Timezone); tz != "" {
		loaded, err := time.LoadLocation(tz)
		if err != nil {
			return Config{}, fmt.Errorf("invalid timezone %q: expected an IANA name", tz)
		}
		location = loaded
	}
	schedule, err := parseCron(rc.Cron)
	if err != nil {
		return Config{}, err
	}
	schedule.Location = location
	if schedule.Next(time.Now().In(location)).IsZero() {
		return Config{}, errors.New("invalid cron: expression never fires")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(rc.ManagementURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if u, err := url.Parse(baseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("invalid management_url %q: expected http(s)://host:port", baseURL)
	}
	key := strings.TrimSpace(rc.ManagementKey)
	if key == "" {
		return Config{}, errors.New("management_key is required")
	}
	return Config{
		Schedule: schedule,
		BaseURL:  baseURL,
		Key:      key,
		Codex:    Provider{Enabled: rc.CodexEnabled, Model: strings.TrimSpace(rc.CodexModel)},
		Claude:   Provider{Enabled: rc.ClaudeEnabled, Model: strings.TrimSpace(rc.ClaudeModel)},
	}, nil
}

// parseCron accepts only standard five-field expressions: sub-minute or
// descriptor schedules could re-prime a window inside the 10s tolerance.
func parseCron(expr string) (*cron.SpecSchedule, error) {
	expr = strings.TrimSpace(expr)
	switch {
	case expr == "":
		return nil, errors.New("cron is required")
	case strings.HasPrefix(expr, "@"):
		return nil, fmt.Errorf("invalid cron %q: descriptors are not supported, use five fields", expr)
	case strings.HasPrefix(expr, "TZ=") || strings.HasPrefix(expr, "CRON_TZ="):
		return nil, fmt.Errorf("invalid cron %q: use the timezone setting instead of a prefix", expr)
	case len(strings.Fields(expr)) != 5:
		return nil, fmt.Errorf("invalid cron %q: expected 5 fields (minute hour day month weekday)", expr)
	}
	parsed, err := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron %q: %v", expr, err)
	}
	return parsed.(*cron.SpecSchedule), nil
}
