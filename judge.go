package primer

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// tolerance must stay below the 60s minimum cron interval: by the next
// trigger a freshly primed Codex window already reads as started.
const tolerance = 10

const (
	fiveHourSeconds = 18000
	weekSeconds     = 604800
)

type codexWindow struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds *float64 `json:"limit_window_seconds"`
	ResetAfterSeconds  *float64 `json:"reset_after_seconds"`
}

// judgeCodex identifies windows by limit_window_seconds, not by slot name.
func judgeCodex(body []byte) verdict {
	var usage struct {
		RateLimit json.RawMessage `json:"rate_limit"`
	}
	if json.Unmarshal(body, &usage) != nil {
		return verdict{decision: decisionQueryFailed, reason: "upstream quota response could not be parsed"}
	}
	var slots struct {
		Primary   *codexWindow `json:"primary_window"`
		Secondary *codexWindow `json:"secondary_window"`
	}
	unknown := func(reason string) verdict {
		return verdict{decision: decisionUnknown, reason: reason, windows: string(usage.RateLimit)}
	}
	if len(usage.RateLimit) > 0 && json.Unmarshal(usage.RateLimit, &slots) != nil {
		return unknown("rate_limit could not be parsed")
	}
	var windows []codexWindow
	for _, w := range []*codexWindow{slots.Primary, slots.Secondary} {
		if w == nil {
			continue // null slot: upstream does not report this window
		}
		if w.UsedPercent == nil || w.LimitWindowSeconds == nil || w.ResetAfterSeconds == nil {
			return unknown("window lacks used_percent, limit_window_seconds or reset_after_seconds")
		}
		windows = append(windows, *w)
	}
	if len(windows) == 0 {
		return unknown("no usage window reported")
	}
	for _, w := range windows {
		if *w.UsedPercent >= 100 {
			return verdict{decision: decisionExhausted, reason: windowName(*w.LimitWindowSeconds) + " window exhausted"}
		}
	}
	var idle []string
	for _, w := range windows {
		limit := *w.LimitWindowSeconds
		if (limit == fiveHourSeconds || limit == weekSeconds) && *w.UsedPercent == 0 &&
			math.Abs(*w.ResetAfterSeconds-limit) <= tolerance {
			idle = append(idle, windowName(limit))
		}
	}
	if len(idle) > 0 {
		return verdict{decision: decisionNotStarted, reason: strings.Join(idle, " and ") + " window not started"}
	}
	return verdict{decision: decisionStarted, reason: "no target window is idle"}
}

func windowName(seconds float64) string {
	switch seconds {
	case fiveHourSeconds:
		return "5h"
	case weekSeconds:
		return "weekly"
	}
	return fmt.Sprintf("%gs", seconds)
}

type claudeWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

// judgeClaude targets five_hour only; seven_day resets at a fixed weekly
// time per account and only matters when exhausted.
func judgeClaude(body []byte) verdict {
	var usage struct {
		FiveHour *claudeWindow `json:"five_hour"`
		SevenDay *claudeWindow `json:"seven_day"`
	}
	var raw struct {
		FiveHour json.RawMessage `json:"five_hour"`
		SevenDay json.RawMessage `json:"seven_day"`
	}
	if json.Unmarshal(body, &raw) != nil {
		return verdict{decision: decisionQueryFailed, reason: "upstream quota response could not be parsed"}
	}
	unknown := func(reason string) verdict {
		windows, _ := json.Marshal(raw)
		return verdict{decision: decisionUnknown, reason: reason, windows: string(windows)}
	}
	if json.Unmarshal(body, &usage) != nil {
		return unknown("five_hour or seven_day could not be parsed")
	}
	five := usage.FiveHour
	if five == nil || five.Utilization == nil {
		return unknown("five_hour or five_hour.utilization missing")
	}
	if five.ResetsAt != nil {
		if _, err := time.Parse(time.RFC3339, *five.ResetsAt); err != nil {
			return unknown("five_hour.resets_at is not a timestamp")
		}
	}
	if usage.SevenDay != nil && usage.SevenDay.Utilization == nil {
		return unknown("seven_day.utilization missing")
	}
	if *five.Utilization >= 100 {
		return verdict{decision: decisionExhausted, reason: "five_hour window exhausted"}
	}
	if usage.SevenDay != nil && *usage.SevenDay.Utilization >= 100 {
		return verdict{decision: decisionExhausted, reason: "seven_day window exhausted"}
	}
	pending := five.ResetsAt == nil
	if *five.Utilization == 0 && pending {
		return verdict{decision: decisionNotStarted, reason: "five_hour window not started", resetPending: true}
	}
	return verdict{decision: decisionStarted, reason: "five_hour window already started", resetPending: pending}
}
