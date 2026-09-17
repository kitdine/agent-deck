package quota

import (
	"context"
	"reflect"
	"testing"
	"time"

	agentdeckstore "github.com/kitdine/agent-deck/internal/store"
)

var _ SettingStore = (*agentdeckstore.Store)(nil)

type mapSettings map[string]string

func (m mapSettings) Setting(_ context.Context, key string) (string, bool, error) {
	value, ok := m[key]
	return value, ok, nil
}

func (m mapSettings) SetSetting(_ context.Context, key, value string) error {
	m[key] = value
	return nil
}

func (m mapSettings) SetSettings(_ context.Context, values map[string]string) error {
	for key, value := range values {
		m[key] = value
	}
	return nil
}

func TestLoadSettingsAppliesProductDefaultsWhenAbsent(t *testing.T) {
	got, err := LoadSettings(context.Background(), mapSettings{})
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got.ProbeEnabled || got.AlertsEnabled || got.StatusLineConsent {
		t.Fatalf("defaults = %+v, want every switch off (requirements.md clause 1)", got)
	}
	if !reflect.DeepEqual(got, DefaultSettings()) {
		t.Fatalf("defaults = %+v, want %+v", got, DefaultSettings())
	}
}

func TestSaveSettingsRoundTrips(t *testing.T) {
	stored := mapSettings{}
	want := Settings{
		ProbeEnabled: true, ProbeInterval: 15 * time.Minute, AlertsEnabled: true,
		AlertThresholds: []float64{90}, ResetNotice: false, StatusLineConsent: true,
	}
	if err := SaveSettings(context.Background(), stored, want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := LoadSettings(context.Background(), stored)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestLoadSettingsRejectsAMalformedStoredValue(t *testing.T) {
	// The reading switch is the kill switch; a value that does not parse must
	// not be guessed.
	if _, err := LoadSettings(context.Background(), mapSettings{settingProbeEnabled: "yes"}); err == nil {
		t.Fatal("LoadSettings accepted quota.probe=yes")
	}
}

func TestSettingsValidateRejectsUnreachableStates(t *testing.T) {
	for name, mutate := range map[string]func(*Settings){
		"consent while reading off": func(s *Settings) { s.StatusLineConsent = true },
		"interval outside choices":  func(s *Settings) { s.ProbeEnabled = true; s.ProbeInterval = 10 * time.Minute },
		"threshold outside choices": func(s *Settings) { s.AlertThresholds = []float64{80} },
		"no thresholds":             func(s *Settings) { s.AlertThresholds = nil },
	} {
		s := DefaultSettings()
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: Validate accepted %+v", name, s)
		}
		if err := SaveSettings(context.Background(), mapSettings{}, s); err == nil {
			t.Errorf("%s: SaveSettings accepted %+v", name, s)
		}
	}
}

func TestParseAlertThresholds(t *testing.T) {
	got, err := ParseAlertThresholds("90, 75,75")
	if err != nil || !reflect.DeepEqual(got, []float64{75, 90}) {
		t.Fatalf("ParseAlertThresholds = (%v, %v), want [75 90]", got, err)
	}
	if _, err := ParseAlertThresholds("80"); err == nil {
		t.Fatal("ParseAlertThresholds accepted 80")
	}
}

func TestParseProbeInterval(t *testing.T) {
	for _, value := range []string{"5m", "15m", "30m"} {
		if _, err := ParseProbeInterval(value); err != nil {
			t.Errorf("ParseProbeInterval(%q): %v", value, err)
		}
	}
	if _, err := ParseProbeInterval("1m"); err == nil {
		t.Fatal("ParseProbeInterval accepted 1m")
	}
}

func TestSettingsAgainstTheRealCoreStore(t *testing.T) {
	core, err := agentdeckstore.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { core.Close() })
	want := DefaultSettings()
	want.ProbeEnabled = true
	if err := SaveSettings(context.Background(), core, want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := LoadSettings(context.Background(), core)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadSettings = (%+v, %v), want %+v", got, err, want)
	}
}
