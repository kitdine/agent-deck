package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const registrationScope = "returned_host_urls_and_targeted_widget_matches"
const registrationOutputLimit = 256 * 1024
const registrationEntryLimit = 64

func registrationBudget(full bool) time.Duration {
	if full {
		return 1500 * time.Millisecond
	}
	return 500 * time.Millisecond
}

func boundedRegistrationCheck(ctx context.Context, full bool, probe func(context.Context, bool) RegistrationDetails) Check {
	start := time.Now()
	total, cancel := context.WithTimeout(ctx, registrationBudget(full))
	defer cancel()
	d := probe(total, full)
	c := registrationCheck(d)
	if c.RegistrationDetails.Applicable && (total.Err() != nil || time.Since(start) > registrationBudget(full)) {
		d = *c.RegistrationDetails
		reason := "timeout"
		if ctx.Err() != nil {
			reason = "cancelled"
		}
		sourceFailure(&d.Host, reason)
		sourceFailure(&d.Widget, reason)
		return registrationCheck(d)
	}
	return c
}

// RegistrationDetails is local doctor evidence. Desktop uses its own DTO.
type RegistrationDetails struct {
	Applicable          bool               `json:"applicable"`
	ApplicabilityReason string             `json:"applicability_reason,omitempty"`
	Scope               string             `json:"scope"`
	Complete            bool               `json:"complete"`
	Host                RegistrationSource `json:"host"`
	Widget              RegistrationSource `json:"widget"`
}

type RegistrationSource struct {
	Source   string              `json:"source"`
	Complete bool                `json:"complete"`
	Reason   string              `json:"reason,omitempty"`
	Entries  []RegistrationEntry `json:"entries"`
}

type RegistrationEntry struct {
	Path      string `json:"path"`
	Canonical bool   `json:"canonical"`
	State     string `json:"state"`
	Version   string `json:"version,omitempty"`
	Build     string `json:"build,omitempty"`
}

func registrationCheck(d RegistrationDetails) Check {
	d.Scope = registrationScope
	for _, src := range []*RegistrationSource{&d.Host, &d.Widget} {
		seen := map[string]bool{}
		entries := make([]RegistrationEntry, 0, len(src.Entries))
		for _, entry := range src.Entries {
			if !seen[entry.Path] {
				seen[entry.Path] = true
				entries = append(entries, entry)
			}
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
		src.Entries = entries
	}
	code := "launchservices_consistent"
	status := "ok"
	if !d.Applicable {
		code = "launchservices_not_applicable"
	} else {
		conflict, stale := false, false
		for _, src := range []*RegistrationSource{&d.Host, &d.Widget} {
			var canonical *RegistrationEntry
			for _, entry := range src.Entries {
				if entry.Canonical && entry.State == "canonical" {
					copy := entry
					canonical = &copy
				}
			}
			if len(src.Entries) >= registrationEntryLimit {
				sourceFailure(src, "entry_limit")
			}
			if len(src.Entries) == 0 {
				sourceFailure(src, "empty_result")
			}
			if canonical == nil {
				sourceFailure(src, "canonical_missing")
			}
			for i := range src.Entries {
				e := &src.Entries[i]
				switch e.State {
				case "missing":
					stale = true
				case "unreadable_metadata", "invalid_metadata":
					sourceFailure(src, e.State)
				case "canonical", "matching_build", "different_build":
					if e.Version == "" || e.Build == "" {
						e.State = "invalid_metadata"
						sourceFailure(src, e.State)
						continue
					}
					if !e.Canonical && canonical != nil {
						if e.Version == canonical.Version && e.Build == canonical.Build {
							e.State = "matching_build"
						} else {
							e.State = "different_build"
							conflict = true
						}
					}
				default:
					sourceFailure(src, "invalid_metadata")
				}
			}
		}
		d.Complete = d.Host.Complete && d.Widget.Complete
		switch {
		case !d.Complete:
			code = "launchservices_unknown"
		case conflict:
			code = "launchservices_conflict"
		case stale:
			code = "launchservices_stale"
		}
		if code != "launchservices_consistent" {
			status = "warning"
		}
	}
	return Check{Name: "launchservices", Status: status, Code: code, Resource: "launchservices_registration", Reason: code, RegistrationDetails: &d}
}

func sourceFailure(s *RegistrationSource, reason string) {
	s.Complete = false
	if s.Reason == "" {
		s.Reason = reason
	}
}

func probeRegistration(ctx context.Context, full bool) RegistrationDetails {
	budget := registrationBudget(full)
	total, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	deadline, _ := total.Deadline()
	acquire, stop := context.WithDeadline(total, deadline.Add(-25*time.Millisecond))
	defer stop()
	d := acquireRegistration(acquire)
	if total.Err() != nil && d.Applicable {
		reason := "timeout"
		if ctx.Err() != nil {
			reason = "cancelled"
		}
		sourceFailure(&d.Host, reason)
		sourceFailure(&d.Widget, reason)
	}
	return d
}

// Owned output pipes share a strict cap; overflow cancels the child. Run always
// waits/reaps, including startup failure, cancellation and output overflow.
type registrationOutput struct {
	sync.Mutex
	buffer   bytes.Buffer
	overflow bool
	limit    int
	cancel   context.CancelFunc
}

func (b *registrationOutput) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	if b.buffer.Len()+len(p) >= b.limit {
		b.overflow = true
		b.cancel()
		return 0, fmt.Errorf("registration output limit")
	}
	return b.buffer.Write(p)
}

func registrationCommand(ctx context.Context, executable string, args ...string) ([]byte, string) {
	return registrationCommandLimited(ctx, registrationOutputLimit, executable, args...)
}

func registrationCommandLimited(ctx context.Context, limit int, executable string, args ...string) ([]byte, string) {
	if limit <= 0 {
		return nil, "output_limit"
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(child, executable, args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "LANG=C")
	cmd.WaitDelay = 25 * time.Millisecond
	out := &registrationOutput{cancel: cancel, limit: limit}
	cmd.Stdout = out
	// Combined stdout/stderr cap, but any stderr makes the source uncertain.
	errOut := &registrationStderr{output: out}
	cmd.Stderr = errOut
	err := cmd.Run()
	if out.overflow {
		return nil, "output_limit"
	}
	if ctx.Err() != nil {
		if ctx.Err() == context.Canceled {
			return nil, "cancelled"
		}
		return nil, "timeout"
	}
	if err != nil || errOut.wrote {
		return nil, "enumeration_failed"
	}
	return out.buffer.Bytes(), ""
}

type registrationStderr struct {
	output *registrationOutput
	wrote  bool
}

func (w *registrationStderr) Write(p []byte) (int, error) { w.wrote = true; return w.output.Write(p) }

var widgetHeading = regexp.MustCompile(`^[+\-! =]*com\.kitdine\.agentdeck\.widget\([^()]+\)$`)
var widgetCount = regexp.MustCompile(`^\(([0-9]+) plug-ins?\)$`)

func parseWidgetPaths(output []byte) ([]string, string) {
	paths := []string{}
	heading := false
	pathSeen := false
	countSeen := false
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimLeft(line, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if countSeen {
			return paths, "unknown_format"
		}
		if widgetHeading.MatchString(trimmed) {
			if heading && !pathSeen {
				return paths, "unknown_format"
			}
			heading = true
			pathSeen = false
			continue
		}
		if m := widgetCount.FindStringSubmatch(trimmed); m != nil {
			var count int
			if _, err := fmt.Sscanf(m[1], "%d", &count); err != nil || count != len(paths) || heading && !pathSeen {
				return paths, "unknown_format"
			}
			countSeen = true
			continue
		}
		key, value, ok := strings.Cut(line, " = ")
		if !ok || !heading {
			return paths, "unknown_format"
		}
		switch strings.TrimSpace(key) {
		case "Path":
			if pathSeen || !strings.HasPrefix(value, "/") {
				return paths, "unknown_format"
			}
			paths = append(paths, value)
			pathSeen = true
		case "UUID", "Timestamp", "SDK", "Parent Bundle", "Display Name", "Short Name", "Parent Name", "Platform":
		default:
			return paths, "unknown_format"
		}
	}
	if len(paths) == 0 {
		return paths, "empty_result"
	}
	if !countSeen {
		return paths, "unknown_format"
	}
	return paths, ""
}

// Keep identities already parsed from incomplete PlugInKit output. Metadata
// remains deadline/output bounded; the earlier parser failure is authoritative.
func widgetSourceFromOutput(ctx context.Context, output []byte, metadata func(context.Context, []string, int) RegistrationSource) RegistrationSource {
	paths, reason := parseWidgetPaths(output)
	if len(paths) == 0 {
		src := RegistrationSource{Source: "widget_pluginkit", Entries: []RegistrationEntry{}}
		sourceFailure(&src, reason)
		return src
	}
	src := metadata(ctx, paths, registrationOutputLimit-len(output))
	if len(src.Entries) == 0 {
		// A killed or failed metadata child cannot erase identities acquired by
		// enumeration. Keep raw paths as uncertain evidence, without claiming
		// normalization, canonical membership, existence, versions or builds.
		seen := make(map[string]bool)
		for _, path := range paths {
			if seen[path] {
				continue
			}
			seen[path] = true
			src.Entries = append(src.Entries, RegistrationEntry{Path: path, State: "unreadable_metadata"})
			if len(src.Entries) >= registrationEntryLimit {
				break
			}
		}
		sourceFailure(&src, "unreadable_metadata")
	}
	if reason != "" {
		src.Complete = false
		src.Reason = reason
	}
	return src
}

func decodeRegistration(data []byte, target any) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}

// WriteRegistrationText renders only local diagnostic details, never a shell
// command. Quoted entries preserve identities and escape terminal controls.
func WriteRegistrationText(w io.Writer, d *RegistrationDetails, code string) error {
	var b strings.Builder
	if !d.Applicable {
		fmt.Fprintf(&b, "  applicability: %s\n", d.ApplicabilityReason)
	} else {
		b.WriteString("  scope: Returned host application URLs and targeted Widget PlugInKit matches only.\n")
		for _, item := range []struct {
			name string
			src  RegistrationSource
		}{{"host", d.Host}, {"widget", d.Widget}} {
			fmt.Fprintf(&b, "  %s: ", item.name)
			if item.src.Complete {
				b.WriteString("complete\n")
			} else {
				fmt.Fprintf(&b, "incomplete (%s)\n", item.src.Reason)
			}
			for _, e := range item.src.Entries {
				fmt.Fprintf(&b, "  %s entry: %q; state=%s", item.name, e.Path, e.State)
				if e.Version != "" {
					fmt.Fprintf(&b, "; version=%q; build=%q", e.Version, e.Build)
				}
				b.WriteByte('\n')
			}
		}
		switch code {
		case "launchservices_conflict", "launchservices_stale":
			b.WriteString("  next: Verify which copy is obsolete and confirm the canonical app and Widget. Quit obsolete app copies normally. Only then manually unregister or remove the verified obsolete registration using its exact path; protect the canonical app and Widget.\n")
		case "launchservices_unknown":
			b.WriteString("  next: Inspect the canonical app and Widget and the incomplete source before choosing any obsolete copy. Run agentdeck doctor --full again.\n")
		default:
			b.WriteString("  next: Matching copies are not a version conflict within the returned-source scope.\n")
		}
		b.WriteString("  limit: A later diagnosis confirms observed registrations only. Check the Widget separately.\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
