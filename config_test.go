package primer_test

import (
	"strings"
	"testing"
	"time"

	primer "github.com/Insulinocytus/cpa-quota-primer-plugin"
)

const validConfig = `
cron: "30 8 * * *"
management_key: secret-management-key
codex_enabled: true
`

func TestParseConfigRejectsInvalid(t *testing.T) {
	for _, tc := range []struct {
		name, yaml, want string
	}{
		{"six field cron", `cron: "0 30 8 * * *"` + "\nmanagement_key: k", "expected 5 fields"},
		{"every descriptor", `cron: "@every 1m"` + "\nmanagement_key: k", "descriptors"},
		{"daily descriptor", `cron: "@daily"` + "\nmanagement_key: k", "descriptors"},
		{"CRON_TZ prefix", `cron: "CRON_TZ=Asia/Tokyo 30 8 * * *"` + "\nmanagement_key: k", "timezone setting"},
		{"TZ prefix", `cron: "TZ=UTC 30 8 * * *"` + "\nmanagement_key: k", "timezone setting"},
		{"bad field", `cron: "61 8 * * *"` + "\nmanagement_key: k", "invalid cron"},
		{"missing cron", "management_key: k", "cron is required"},
		{"missing key", `cron: "30 8 * * *"`, "management_key is required"},
		{"bad timezone", `cron: "30 8 * * *"` + "\ntimezone: Mars/Base\nmanagement_key: k", "invalid timezone"},
		{"bad management_url", `cron: "30 8 * * *"` + "\nmanagement_key: k\nmanagement_url: 127.0.0.1:8317", "management_url"},
		{"unknown key", validConfig + "cronn: x\n", "field cronn not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := primer.ParseConfig([]byte(tc.yaml))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := primer.ParseConfig([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://127.0.0.1:8317" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Schedule.Location != time.Local {
		t.Fatalf("Location = %v, want host local", cfg.Schedule.Location)
	}
	if !cfg.Codex.Enabled || cfg.Claude.Enabled {
		t.Fatalf("switches = %+v", cfg)
	}
}

func TestParseConfigTimezoneDrivesSchedule(t *testing.T) {
	cfg, err := primer.ParseConfig([]byte(validConfig + "timezone: Asia/Tokyo\n"))
	if err != nil {
		t.Fatal(err)
	}
	// 08:30 Tokyo is 23:30 UTC the previous day.
	next := cfg.Schedule.Next(time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC))
	if want := time.Date(2026, 10, 8, 23, 30, 0, 0, time.UTC); !next.Equal(want) {
		t.Fatalf("next = %v, want %v", next, want)
	}
}

func TestParseConfigAcceptsHostOwnedKeys(t *testing.T) {
	_, err := primer.ParseConfig([]byte(validConfig + `
enabled: true
priority: 3
store:
  id: cpa-quota-primer
  version: 0.1.0
`))
	if err != nil {
		t.Fatal(err)
	}
}
