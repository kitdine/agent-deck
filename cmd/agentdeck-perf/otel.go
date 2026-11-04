package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
	"github.com/kitdine/agent-deck/internal/telemetry"
)

// mockProducer names the mock in every event, so the product never mistakes
// a measurement event for a supported producer: it stays capture_only.
const mockProducer = "agentdeck-perf-mock"

type otelResult struct {
	Sent     int     `json:"sent"`
	Accepted int     `json:"accepted"`
	Statuses []int   `json:"statuses"`
	SendMS   []int64 `json:"send_ms"`
}

// runOTel is the P05 mock producer: it sends seeded, metered OTLP/HTTP JSON
// log events, one per request, to the loopback receiver configured in a state
// root. otel.send is marked before each request and records its sequence.
func runOTel(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("otel", flag.ContinueOnError)
	flags.SetOutput(stderr)
	stateDir := flags.String("state-dir", "", "AgentDeck state root with an enabled receiver")
	seed := flags.Uint64("seed", 1, "event seed")
	first := flags.Int("first", 1, "sequence number of the first event")
	count := flags.Int("n", 1, "number of events")
	interval := flags.Duration("interval", 0, "pause between events")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	result, err := sendOTel(*stateDir, *seed, *first, *count, *interval)
	if err == nil {
		err = json.NewEncoder(stdout).Encode(result)
	}
	if err != nil {
		fmt.Fprintln(stderr, "agentdeck-perf otel:", err)
		return 1
	}
	return 0
}

func sendOTel(stateDir string, seed uint64, first, count int, interval time.Duration) (otelResult, error) {
	var result otelResult
	if stateDir == "" || count < 1 || first < 1 {
		return result, errors.New("--state-dir, --first >= 1 and --n >= 1 are required")
	}
	config, err := telemetry.LoadConfig(stateDir)
	if err != nil {
		return result, err
	}
	if !config.Enabled {
		return result, errors.New("the receiver is not enabled in --state-dir")
	}
	token, err := telemetry.ReadToken(stateDir)
	if err != nil {
		return result, err
	}
	if perftrace.Start(perftrace.RoleDriver) {
		defer perftrace.Close()
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for n := first; n < first+count; n++ {
		if n > first && interval > 0 {
			time.Sleep(interval)
		}
		body := mockEvent(seed, n)
		request, err := http.NewRequest(http.MethodPost, telemetry.Endpoint(config.Port), bytes.NewReader(body))
		if err != nil {
			return result, err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(telemetry.TokenHeader, token)
		perftrace.Mark(perftrace.OTelSend, perftrace.Count{Key: "seq", Value: int64(n)})
		started := time.Now()
		response, err := client.Do(request)
		result.Sent++
		if err != nil {
			return result, err
		}
		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		result.Statuses = append(result.Statuses, response.StatusCode)
		result.SendMS = append(result.SendMS, time.Since(started).Milliseconds())
		if response.StatusCode == http.StatusOK {
			result.Accepted++
		}
	}
	return result, nil
}

// mockEvent is one deterministic codex.sse_event response.completed record.
// The same seed and sequence always produce the same bytes.
func mockEvent(seed uint64, n int) []byte {
	rng := rand.New(rand.NewPCG(seed, uint64(n)))
	input := 1000 + rng.Int64N(50000)
	output := 10 + rng.Int64N(4000)
	cached := rng.Int64N(input + 1)
	reasoning := rng.Int64N(output + 1)
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(n) * time.Second).UnixNano()
	attribute := func(key, kind string, value any) map[string]any {
		return map[string]any{"key": key, "value": map[string]any{kind: value}}
	}
	request := map[string]any{"resourceLogs": []any{map[string]any{
		"resource": map[string]any{"attributes": []any{
			attribute("service.name", "stringValue", mockProducer),
			attribute("service.version", "stringValue", "1"),
			attribute("service.instance.id", "stringValue", fmt.Sprintf("seed-%d", seed)),
		}},
		"scopeLogs": []any{map[string]any{"logRecords": []any{map[string]any{
			"timeUnixNano": fmt.Sprint(stamp),
			"attributes": []any{
				attribute("event.name", "stringValue", "codex.sse_event"),
				attribute("event.kind", "stringValue", "response.completed"),
				attribute("model", "stringValue", "mock-model"),
				attribute("response.id", "stringValue", fmt.Sprintf("mock-%d-%d", seed, n)),
				attribute("input_token_count", "intValue", fmt.Sprint(input)),
				attribute("output_token_count", "intValue", fmt.Sprint(output)),
				attribute("cached_token_count", "intValue", fmt.Sprint(cached)),
				attribute("reasoning_token_count", "intValue", fmt.Sprint(reasoning)),
			},
		}}}},
	}}}
	data, _ := json.Marshal(request)
	return data
}
