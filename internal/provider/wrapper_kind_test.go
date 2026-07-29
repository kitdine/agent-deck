package provider

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

// newWrapperKindService builds a service holding one custom provider, which is
// the smallest state in which a wrapper can be declared.
func newWrapperKindService(t *testing.T) (context.Context, Service, *store.Store) {
	t.Helper()
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	service := Service{Store: database, Vault: testCredentialVault(t)}
	if _, err = service.Add(ctx, Definition{Name: "example", Endpoint: "https://provider.example", Clients: []Client{ClientCodex}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	return ctx, service, database
}

// TestNormalizeWrapperKindResolvesTheDefaultAndRejectsUnknownProtocols pins the
// vocabulary. An empty value must resolve rather than fail, because it is what
// both an omitted flag and a row written before the field existed carry.
func TestNormalizeWrapperKindResolvesTheDefaultAndRejectsUnknownProtocols(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "omitted resolves to the default", value: "", want: WrapperKindPlain},
		{name: "explicit default", value: "plain", want: WrapperKindPlain},
		{name: "headroom", value: "headroom", want: WrapperKindHeadroom},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeWrapperKind(test.value)
			if err != nil || got != test.want {
				t.Fatalf("NormalizeWrapperKind(%q) = %q, %v, want %q", test.value, got, err, test.want)
			}
		})
	}

	for _, value := range []string{"Headroom", "HEADROOM", "litellm", "plain ", "true", "1"} {
		if _, err := NormalizeWrapperKind(value); !errors.Is(err, ErrInvalidProvider) {
			t.Fatalf("NormalizeWrapperKind(%q) error = %v, want ErrInvalidProvider", value, err)
		}
	}
}

// TestReportedWrapperKindHidesTheDefault pins the rule that keeps this field
// additive: a wrapper that declared nothing, and one that declared the default,
// report identically to a build that had no such field at all.
func TestReportedWrapperKindHidesTheDefault(t *testing.T) {
	for _, stored := range []string{"", WrapperKindPlain} {
		if got := reportedWrapperKind(stored); got != "" {
			t.Fatalf("reportedWrapperKind(%q) = %q, want it hidden", stored, got)
		}
	}
	if got := reportedWrapperKind(WrapperKindHeadroom); got != WrapperKindHeadroom {
		t.Fatalf("reportedWrapperKind(headroom) = %q, want it reported", got)
	}
}

// TestSetWrapperRejectsAnUnknownKindBeforeAnyWrite pins that a bad declaration
// never reaches storage: the provider must be left exactly as it was.
func TestSetWrapperRejectsAnUnknownKindBeforeAnyWrite(t *testing.T) {
	ctx, service, _ := newWrapperKindService(t)

	if _, _, err := service.SetWrapper(ctx, "example", "https://wrapper.example", "litellm", false); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("SetWrapper with unknown kind error = %v, want ErrInvalidProvider", err)
	}

	definition, err := service.Show(ctx, "example")
	if err != nil {
		t.Fatal(err)
	}
	if definition.Definition.WrapperURL != "" || definition.Definition.WrapperKind != "" {
		t.Fatalf("rejected declaration still stored something: %#v", definition.Definition)
	}
}

// TestSetWrapperStoresAndClearsTheDeclaredKind covers the service-level
// round-trip for a stored provider, including that --clear takes the
// declaration with the URL.
func TestSetWrapperStoresAndClearsTheDeclaredKind(t *testing.T) {
	ctx, service, _ := newWrapperKindService(t)

	result, _, err := service.SetWrapper(ctx, "example", "https://wrapper.example", "headroom", false)
	if err != nil || result.Definition.WrapperKind != WrapperKindHeadroom {
		t.Fatalf("SetWrapper = %#v, %v", result.Definition, err)
	}

	cleared, _, err := service.SetWrapper(ctx, "example", "", "", true)
	if err != nil || cleared.Definition.WrapperURL != "" || cleared.Definition.WrapperKind != "" {
		t.Fatalf("cleared definition = %#v, %v", cleared.Definition, err)
	}
}

// TestStoredWrapperKindGivesTheDefaultOneEncoding pins the write-side half of
// the pair. Storing the literal "plain" would give one logical state two on-disk
// encodings — absent for a wrapper written before this field existed, "plain"
// for one written after — for no gain, since both report identically.
func TestStoredWrapperKindGivesTheDefaultOneEncoding(t *testing.T) {
	if got := storedWrapperKind(WrapperKindPlain); got != "" {
		t.Fatalf("storedWrapperKind(plain) = %q, want the default stored as absence", got)
	}
	if got := storedWrapperKind(WrapperKindHeadroom); got != WrapperKindHeadroom {
		t.Fatalf("storedWrapperKind(headroom) = %q, want it persisted", got)
	}
}

// TestSetWrapperReplacesRatherThanPatchesAndNamesWhatItDropped pins the
// semantics the review settled on: set-wrapper sets the whole wrapper, so
// omitting --kind returns the declaration to the default. The dropped value is
// reported so the change is visible, because a user moving a proxy to a new
// address would otherwise get no sign that attribution stopped.
func TestSetWrapperReplacesRatherThanPatchesAndNamesWhatItDropped(t *testing.T) {
	ctx, service, _ := newWrapperKindService(t)
	if _, dropped, err := service.SetWrapper(ctx, "example", "https://old.example", "headroom", false); err != nil || dropped != "" {
		t.Fatalf("first declaration dropped %q, %v, want nothing dropped", dropped, err)
	}

	result, dropped, err := service.SetWrapper(ctx, "example", "https://new.example", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Definition.WrapperKind != "" {
		t.Fatalf("declaration survived a URL-only set: %#v", result.Definition)
	}
	if dropped != WrapperKindHeadroom {
		t.Fatalf("dropped = %q, want the replaced declaration named", dropped)
	}
}

// TestSetWrapperReportsNothingDroppedWhenNothingWasLost covers every case that
// must stay silent, so the advisory means something when it does appear.
func TestSetWrapperReportsNothingDroppedWhenNothingWasLost(t *testing.T) {
	ctx, service, _ := newWrapperKindService(t)

	if _, dropped, err := service.SetWrapper(ctx, "example", "https://a.example", "", false); err != nil || dropped != "" {
		t.Fatalf("first undeclared set dropped %q, %v", dropped, err)
	}
	if _, dropped, err := service.SetWrapper(ctx, "example", "https://b.example", "plain", false); err != nil || dropped != "" {
		t.Fatalf("plain to plain dropped %q, %v", dropped, err)
	}
	if _, _, err := service.SetWrapper(ctx, "example", "https://c.example", "headroom", false); err != nil {
		t.Fatal(err)
	}
	if _, dropped, err := service.SetWrapper(ctx, "example", "https://d.example", "headroom", false); err != nil || dropped != "" {
		t.Fatalf("headroom to headroom dropped %q, %v", dropped, err)
	}
	// Clearing removes the wrapper outright, which is what the user asked for,
	// so it reports nothing.
	if _, dropped, err := service.SetWrapper(ctx, "example", "", "", true); err != nil || dropped != "" {
		t.Fatalf("clear dropped %q, %v", dropped, err)
	}
}

// TestSetWrapperWithoutAKindReportsNothingNew is the additive check at the
// service boundary: the common invocation must leave the reported definition
// exactly as it was before this field existed.
func TestSetWrapperWithoutAKindReportsNothingNew(t *testing.T) {
	ctx, service, _ := newWrapperKindService(t)

	result, _, err := service.SetWrapper(ctx, "example", "https://wrapper.example", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Definition.WrapperURL != "https://wrapper.example" || result.Definition.WrapperKind != "" {
		t.Fatalf("undeclared wrapper = %#v, want the URL alone", result.Definition)
	}
}
