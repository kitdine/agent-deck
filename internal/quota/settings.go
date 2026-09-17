package quota

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The quota preferences live in the core state's settings table (wire-and-cli
// task, operator-approved), so `agentdeck quota`, the desktop snapshot, and the
// desktop's quota refresh read one authority. An absent key means the product
// default.
const (
	settingProbeEnabled      = "quota.probe"
	settingProbeInterval     = "quota.interval"
	settingAlertsEnabled     = "quota.alerts"
	settingAlertThresholds   = "quota.thresholds"
	settingResetNotice       = "quota.reset_notice"
	settingStatusLineConsent = "quota.statusline"
)

// ProbeIntervals are C9's three selectable background cadences.
var ProbeIntervals = []time.Duration{5 * time.Minute, 15 * time.Minute, 30 * time.Minute}

// AlertThresholdChoices are the threshold values ux/settings-quota.md offers.
var AlertThresholdChoices = []float64{75, 90}

// Settings is the subscription-quota settings group.
type Settings struct {
	ProbeEnabled      bool
	ProbeInterval     time.Duration
	AlertsEnabled     bool
	AlertThresholds   []float64
	ResetNotice       bool
	StatusLineConsent bool
}

// DefaultSettings is the product default: the three switches off
// (requirements.md clause 1, ux/settings-quota.md), the 5m interval, both
// thresholds, and the reset notice on beneath the still-off alerts switch, as
// the prototype's defaults have it.
func DefaultSettings() Settings {
	return Settings{ProbeInterval: 5 * time.Minute, AlertThresholds: []float64{75, 90}, ResetNotice: true}
}

// AlertConfig projects the alert half for DueAlerts.
func (s Settings) AlertConfig() AlertConfig {
	return AlertConfig{Enabled: s.AlertsEnabled, Thresholds: slices.Clone(s.AlertThresholds), ResetNotice: s.ResetNotice}
}

// Validate rejects states the settings group cannot reach: status-line consent
// while reading is off (requirements.md clause 14), an interval outside C9's
// three values, and thresholds outside the offered choices.
func (s Settings) Validate() error {
	if s.StatusLineConsent && !s.ProbeEnabled {
		return errors.New("status-line consent requires quota reading to be on")
	}
	if !slices.Contains(ProbeIntervals, s.ProbeInterval) {
		return fmt.Errorf("quota interval %s is not one of 5m, 15m, 30m", s.ProbeInterval)
	}
	if len(s.AlertThresholds) == 0 {
		return errors.New("at least one alert threshold is required")
	}
	for _, threshold := range s.AlertThresholds {
		if !slices.Contains(AlertThresholdChoices, threshold) {
			return fmt.Errorf("alert threshold %s is not one of 75, 90", strconv.FormatFloat(threshold, 'f', -1, 64))
		}
	}
	return nil
}

// SettingReader is the read half of the core state's settings table.
type SettingReader interface {
	Setting(ctx context.Context, key string) (string, bool, error)
}

// SettingStore adds the write half. SetSettings must write its whole batch
// in one transaction (SaveSettings relies on this for the settings group's
// atomicity), while SetSetting remains for a genuinely single-key write.
type SettingStore interface {
	SettingReader
	SetSetting(ctx context.Context, key, value string) error
	SetSettings(ctx context.Context, values map[string]string) error
}

// LoadSettings reads the settings group, applying the product default for any
// absent key. A stored value that does not parse is an error rather than a
// silent default: the reading switch is the topic's kill switch, and guessing
// its state is not a decision this function may make.
func LoadSettings(ctx context.Context, r SettingReader) (Settings, error) {
	s := DefaultSettings()
	load := func(key string, apply func(string) error) error {
		value, ok, err := r.Setting(ctx, key)
		if err != nil || !ok {
			return err
		}
		if err := apply(value); err != nil {
			return fmt.Errorf("stored %s: %w", key, err)
		}
		return nil
	}
	switchInto := func(target *bool) func(string) error {
		return func(value string) (err error) {
			*target, err = ParseSwitch(value)
			return err
		}
	}
	for _, step := range []struct {
		key   string
		apply func(string) error
	}{
		{settingProbeEnabled, switchInto(&s.ProbeEnabled)},
		{settingProbeInterval, func(value string) (err error) { s.ProbeInterval, err = ParseProbeInterval(value); return err }},
		{settingAlertsEnabled, switchInto(&s.AlertsEnabled)},
		{settingAlertThresholds, func(value string) (err error) { s.AlertThresholds, err = ParseAlertThresholds(value); return err }},
		{settingResetNotice, switchInto(&s.ResetNotice)},
		{settingStatusLineConsent, switchInto(&s.StatusLineConsent)},
	} {
		if err := load(step.key, step.apply); err != nil {
			return Settings{}, err
		}
	}
	return s, nil
}

// SaveSettings validates and writes every key of the settings group in one
// transaction (SettingStore.SetSettings), so a write error or a concurrent
// reader can never observe only part of the validated group -- map iteration
// order made that partial state nondeterministic before this was atomic.
func SaveSettings(ctx context.Context, w SettingStore, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	return w.SetSettings(ctx, map[string]string{
		settingProbeEnabled:      FormatSwitch(s.ProbeEnabled),
		settingProbeInterval:     s.ProbeInterval.String(),
		settingAlertsEnabled:     FormatSwitch(s.AlertsEnabled),
		settingAlertThresholds:   FormatAlertThresholds(s.AlertThresholds),
		settingResetNotice:       FormatSwitch(s.ResetNotice),
		settingStatusLineConsent: FormatSwitch(s.StatusLineConsent),
	})
}

// ParseSwitch accepts "on" or "off".
func ParseSwitch(value string) (bool, error) {
	switch strings.TrimSpace(value) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	default:
		return false, fmt.Errorf("%q is not on or off", value)
	}
}

func FormatSwitch(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// ParseProbeInterval accepts one of C9's three cadences, written as 5m, 15m,
// or 30m.
func ParseProbeInterval(value string) (time.Duration, error) {
	interval, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || !slices.Contains(ProbeIntervals, interval) {
		return 0, fmt.Errorf("%q is not one of 5m, 15m, 30m", value)
	}
	return interval, nil
}

// ParseAlertThresholds accepts "75", "90", or both comma-separated.
func ParseAlertThresholds(value string) ([]float64, error) {
	var thresholds []float64
	for _, part := range strings.Split(value, ",") {
		threshold, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || !slices.Contains(AlertThresholdChoices, threshold) {
			return nil, fmt.Errorf("%q is not 75, 90, or 75,90", value)
		}
		if !slices.Contains(thresholds, threshold) {
			thresholds = append(thresholds, threshold)
		}
	}
	slices.Sort(thresholds)
	return thresholds, nil
}

func FormatAlertThresholds(thresholds []float64) string {
	parts := make([]string, 0, len(thresholds))
	for _, threshold := range thresholds {
		parts = append(parts, strconv.FormatFloat(threshold, 'f', -1, 64))
	}
	return strings.Join(parts, ",")
}
