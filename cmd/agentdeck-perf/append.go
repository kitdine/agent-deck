package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/perftrace"
)

const streamAppend = 0x617070656e64 // "append"

// appendResult tells the oracle what the append added.
type appendResult struct {
	Files       int   `json:"files"`
	Records     int   `json:"records"`
	TokensDelta int64 `json:"tokens_delta"`
	CodexFiles  int   `json:"codex_files"`
	ClaudeFiles int   `json:"claude_files"`
}

func runAppend(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("append", flag.ContinueOnError)
	flags.SetOutput(stderr)
	home := flags.String("home", "", "fixture HOME")
	anchor := flags.String("anchor", "", "anchor date YYYY-MM-DD in --tz")
	tz := flags.String("tz", "America/New_York", "IANA timezone of the anchor date")
	seed := flags.Uint64("seed", 1, "fixture seed")
	sequence := flags.Int("n", 1, "append sequence number; selects the target file")
	all := flags.Bool("all-today", false, "append one record to every today-related file (P08)")
	replay := flags.String("replay", "", "write the records of a planned append (P10) instead of new ones")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	var result appendResult
	var err error
	if *replay != "" {
		result, err = replayAppend(*home, *replay)
	} else {
		var location *time.Location
		if location, err = time.LoadLocation(*tz); err == nil {
			var day time.Time
			if day, err = time.ParseInLocation("2006-01-02", *anchor, location); err == nil {
				result, err = appendRecords(*home, day, *seed, *sequence, *all)
			}
		}
	}
	if err == nil {
		err = json.NewEncoder(stdout).Encode(result)
	}
	if err != nil {
		fmt.Fprintln(stderr, "agentdeck-perf append:", err)
		return 1
	}
	return 0
}

// todayFiles lists the fixture's today-related sources, sorted by path.
func todayFiles(home string, midnight time.Time) ([]string, error) {
	var out []string
	for _, root := range []string{".codex/sessions", ".claude/projects"} {
		err := filepath.WalkDir(filepath.Join(home, root), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".jsonl") {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.ModTime().Before(midnight) {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// plannedAppend is one append's records, by path relative to HOME, so the
// same records can be written to another copy of the sources (P10's oracle
// mirror and then the measured working copy).
type plannedAppend struct {
	Sequence int           `json:"sequence"`
	Records  []plannedLine `json:"records"`
}

type plannedLine struct {
	Path   string `json:"path"`
	Line   string `json:"line"`
	Tokens int64  `json:"tokens"`
	Codex  bool   `json:"codex"`
}

func appendRecords(home string, midnight time.Time, seed uint64, sequence int, all bool) (appendResult, error) {
	plan, err := planAppend(home, midnight, seed, sequence, all)
	if err != nil {
		return appendResult{}, err
	}
	return writeAppend(home, plan, true)
}

// planAppend builds the next records for the files of home, without writing.
func planAppend(home string, midnight time.Time, seed uint64, sequence int, all bool) (plannedAppend, error) {
	if !filepath.IsAbs(home) {
		return plannedAppend{}, errors.New("--home must be absolute")
	}
	files, err := todayFiles(home, midnight)
	if err != nil {
		return plannedAppend{}, err
	}
	if len(files) == 0 {
		return plannedAppend{}, errors.New("fixture has no today-related files")
	}
	targets := files
	if !all {
		targets = files[(sequence-1+len(files))%len(files) : (sequence-1+len(files))%len(files)+1]
	}
	rng := rand.New(rand.NewPCG(seed, streamAppend+uint64(sequence)))
	plan := plannedAppend{Sequence: sequence}
	for _, path := range targets {
		line, tokens, codex, err := nextRecord(path, rng)
		if err != nil {
			return plannedAppend{}, err
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return plannedAppend{}, err
		}
		plan.Records = append(plan.Records, plannedLine{Path: relative, Line: line, Tokens: tokens, Codex: codex})
	}
	return plan, nil
}

// replayAppend writes a planned append saved as JSON to home's sources.
func replayAppend(home, path string) (appendResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return appendResult{}, err
	}
	var plan plannedAppend
	if err := json.Unmarshal(raw, &plan); err != nil {
		return appendResult{}, err
	}
	if !filepath.IsAbs(home) {
		return appendResult{}, errors.New("--home must be absolute")
	}
	return writeAppend(home, plan, true)
}

// writeAppend writes each record durably; with traced set, source.written is
// recorded once all are on disk, carrying the append's sequence number.
func writeAppend(home string, plan plannedAppend, traced bool) (appendResult, error) {
	if traced && perftrace.Start(perftrace.RoleDriver) {
		defer perftrace.Close()
	}
	var result appendResult
	for _, record := range plan.Records {
		if !filepath.IsLocal(record.Path) {
			return appendResult{}, fmt.Errorf("planned record path %q leaves HOME", record.Path)
		}
		if err := appendDurably(filepath.Join(home, record.Path), record.Line); err != nil {
			return appendResult{}, err
		}
		result.Files++
		result.Records++
		result.TokensDelta += record.Tokens
		if record.Codex {
			result.CodexFiles++
		} else {
			result.ClaudeFiles++
		}
	}
	if traced {
		perftrace.Mark(perftrace.SourceWritten, perftrace.Count{Key: "records", Value: int64(result.Records)},
			perftrace.Count{Key: "tokens", Value: result.TokensDelta}, perftrace.Count{Key: "seq", Value: int64(plan.Sequence)})
	}
	return result, nil
}

// nextRecord builds one complete record continuing the file's session.
func nextRecord(path string, rng *rand.Rand) (string, int64, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", 0, false, err
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	input := int64(1000 + rng.IntN(9000))
	output := int64(50 + rng.IntN(950))
	if strings.Contains(path, string(filepath.Separator)+".codex"+string(filepath.Separator)) {
		total, ok := lastCodexTotal(raw)
		if !ok {
			return "", 0, false, fmt.Errorf("%s: no prior token_count to continue", filepath.Base(path))
		}
		usage := func(in, cached, out, reasoning int64) string {
			return `{"input_tokens":` + strconv.FormatInt(in, 10) + `,"cached_input_tokens":` + strconv.FormatInt(cached, 10) +
				`,"output_tokens":` + strconv.FormatInt(out, 10) + `,"reasoning_output_tokens":` + strconv.FormatInt(reasoning, 10) +
				`,"total_tokens":` + strconv.FormatInt(in+out, 10) + `}`
		}
		// Every cumulative field continues, so the parser takes the delta.
		line := `{"timestamp":"` + now + `","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":` +
			usage(total[0]+input, total[1], total[2]+output, total[3]) + `,"last_token_usage":` + usage(input, 0, output, 0) +
			`,"model_context_window":272000},"rate_limits":null}}`
		return line, input + output, true, nil
	}
	session, ok := lastClaudeSession(raw)
	if !ok {
		return "", 0, false, fmt.Errorf("%s: no prior Claude record to continue", filepath.Base(path))
	}
	hex := func(n int) string {
		var b strings.Builder
		for range n {
			b.WriteByte("0123456789abcdef"[rng.IntN(16)])
		}
		return b.String()
	}
	line := `{"parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/perf/workspace/append","sessionId":"` + session +
		`","version":"2.1.0","type":"assistant","message":{"id":"msg_` + hex(24) + `","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"append"}],"stop_reason":null,"usage":{"input_tokens":` +
		strconv.FormatInt(input, 10) + `,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":` +
		strconv.FormatInt(output, 10) + `}},"uuid":"` + hex(8) + "-" + hex(4) + "-4" + hex(3) + "-a" + hex(3) + "-" + hex(12) +
		`","timestamp":"` + now + `"}`
	return line, input + output, false, nil
}

func lastCodexTotal(raw []byte) ([4]int64, bool) {
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		var record struct {
			Payload struct {
				Type string `json:"type"`
				Info struct {
					Total struct {
						Input     int64 `json:"input_tokens"`
						Cached    int64 `json:"cached_input_tokens"`
						Output    int64 `json:"output_tokens"`
						Reasoning int64 `json:"reasoning_output_tokens"`
					} `json:"total_token_usage"`
				} `json:"info"`
			} `json:"payload"`
		}
		if json.Unmarshal(lines[i], &record) == nil && record.Payload.Type == "token_count" {
			total := record.Payload.Info.Total
			return [4]int64{total.Input, total.Cached, total.Output, total.Reasoning}, true
		}
	}
	return [4]int64{}, false
}

func lastClaudeSession(raw []byte) (string, bool) {
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		var record struct {
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(lines[i], &record) == nil && record.SessionID != "" {
			return record.SessionID, true
		}
	}
	return "", false
}

// appendDurably writes one line and fsyncs the file and its directory before
// source.written is recorded.
func appendDurably(path, line string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	if _, err = file.WriteString(line + "\n"); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
