package main

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

var (
	codexModels  = []string{"gpt-5.6", "gpt-5.6-codex", "gpt-5.5"}
	claudeModels = []string{"claude-opus-4-7", "claude-sonnet-4-6"}
	vocabulary   = strings.Fields("scan source usage session ledger window budget cursor batch commit " +
		"render popover publication snapshot reduce writer decode record token model turn tool " +
		"partition coverage fixture anchor sample marker frame layout signal metric profile helper " +
		"queue lease receipt inventory derive project folder branch review repair verify build")
)

// padMin is the shortest padding line either client can emit; a record is
// only written when it leaves either zero bytes or at least padMin bytes.
const padMin = 128

type contentWriter struct {
	w         *bufio.Writer
	rng       *rand.Rand
	spec      fileSpec
	remaining int64
	written   int64
	last      time.Time
	done      bool
	err       error

	turn       int
	modelIndex int
	cumulative [4]int64 // input, cached, output, reasoning
	parentUUID string
}

// generateContent writes exactly spec.size bytes of JSONL and returns the
// timestamp of the last record, which becomes the file mtime.
func generateContent(w *bufio.Writer, seed uint64, spec fileSpec) (time.Time, error) {
	c := &contentWriter{
		w: w, rng: rand.New(rand.NewPCG(seed^uint64(spec.index+1)*0x9e3779b97f4a7c15, streamContent)),
		spec: spec, remaining: spec.size, last: spec.start,
	}
	if spec.overlong {
		c.emitPad(min(overlongLine, c.remaining))
	}
	if spec.client == "codex" {
		c.emit(c.codexSessionMeta())
	}
	for !c.done && c.err == nil {
		if spec.client == "codex" {
			c.codexTurn()
		} else {
			c.claudeTurn()
		}
	}
	if c.err == nil && c.written != spec.size {
		c.err = fmt.Errorf("fixture: wrote %d bytes, want %d", c.written, spec.size)
	}
	return c.last, c.err
}

// now maps the byte position to a timestamp, so record times follow file
// order and a continued session moves to today for its final 30%.
func (c *contentWriter) now() string {
	f := float64(c.written) / float64(c.spec.size)
	start, end := c.spec.start, c.spec.end
	if c.spec.continued {
		if f < continuedSplit {
			f /= continuedSplit
		} else {
			start, end = c.spec.todayStart, c.spec.todayEnd
			f = (f - continuedSplit) / (1 - continuedSplit)
		}
	}
	t := start.Add(time.Duration(f * float64(end.Sub(start)))).Truncate(time.Millisecond)
	if t.Before(c.last) {
		t = c.last
	}
	c.last = t
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func (c *contentWriter) emit(line string) {
	if c.done || c.err != nil {
		return
	}
	size := int64(len(line)) + 1
	if left := c.remaining - size; left != 0 && left < padMin {
		c.emitPad(c.remaining)
		return
	}
	c.raw(line)
	if c.done {
		return
	}
	// Malformed lines are inserted, never substituted, so they do not change
	// the usage a parser can recover from the well-formed records.
	if c.rng.Float64() < malformedRate {
		c.emit(`{"type":"event_msg","payload":{"type":`)
	}
}

func (c *contentWriter) raw(line string) {
	if _, err := c.w.WriteString(line); err != nil {
		c.err = err
		return
	}
	if err := c.w.WriteByte('\n'); err != nil {
		c.err = err
		return
	}
	c.written += int64(len(line)) + 1
	c.remaining -= int64(len(line)) + 1
	c.done = c.remaining == 0
}

// emitPad writes one record of exactly size bytes including LF. Parsers
// ignore both padding shapes for usage.
func (c *contentWriter) emitPad(size int64) {
	var prefix, suffix string
	if c.spec.client == "codex" {
		prefix = `{"timestamp":"` + c.now() + `","type":"event_msg","payload":{"type":"agent_reasoning","text":"`
		suffix = `"}}`
	} else {
		prefix = `{"type":"summary","summary":"`
		suffix = `","leafUuid":"` + c.spec.session + `"}`
	}
	fill := size - 1 - int64(len(prefix)+len(suffix))
	if fill < 0 {
		c.err = errors.New("fixture: padding budget below the padding record size")
		return
	}
	c.raw(prefix + strings.Repeat("x", int(fill)) + suffix)
}

func (c *contentWriter) words(minWords, maxWords int) string {
	n := minWords + c.rng.IntN(maxWords-minWords+1)
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(vocabulary[c.rng.IntN(len(vocabulary))])
	}
	return b.String()
}

func (c *contentWriter) hex(n int) string {
	const digits = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = digits[c.rng.IntN(16)]
	}
	return string(b)
}

func (c *contentWriter) cwd() string {
	return fmt.Sprintf("/perf/workspace/p%02d", c.spec.project)
}

func (c *contentWriter) codexSessionMeta() string {
	return `{"timestamp":"` + c.now() + `","type":"session_meta","payload":{"id":"` + c.spec.session +
		`","timestamp":"` + c.now() + `","cwd":"` + c.cwd() +
		`","originator":"codex_cli_rs","cli_version":"0.99.0","source":"cli","model_provider":"openai"}}`
}

func (c *contentWriter) codexTurn() {
	c.turn++
	if c.rng.IntN(12) == 0 {
		c.modelIndex = c.rng.IntN(len(codexModels))
	}
	turnID := c.spec.session[:8] + "-turn-" + strconv.Itoa(c.turn)
	c.emit(`{"timestamp":"` + c.now() + `","type":"turn_context","payload":{"turn_id":"` + turnID + `","cwd":"` + c.cwd() +
		`","model":"` + codexModels[c.modelIndex] + `","approval_policy":"on-request","sandbox_policy":{"type":"workspace-write"}}}`)
	c.emit(`{"timestamp":"` + c.now() + `","type":"event_msg","payload":{"type":"user_message","message":"` + c.words(4, 40) + `"}}`)
	for range c.rng.IntN(4) {
		call := "call_" + c.hex(24)
		c.emit(`{"timestamp":"` + c.now() + `","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"rg -n ` +
			vocabulary[c.rng.IntN(len(vocabulary))] + ` internal\"}","call_id":"` + call + `"}}`)
		c.emit(`{"timestamp":"` + c.now() + `","type":"response_item","payload":{"type":"function_call_output","call_id":"` + call +
			`","output":"` + c.words(20, 400) + `"}}`)
	}
	c.emit(`{"timestamp":"` + c.now() + `","type":"event_msg","payload":{"type":"agent_message","message":"` + c.words(10, 120) + `"}}`)
	input := int64(2000 + c.rng.IntN(38000))
	cached := input * int64(c.rng.IntN(80)) / 100
	output := int64(50 + c.rng.IntN(3950))
	reasoning := int64(c.rng.IntN(2000))
	c.cumulative[0] += input
	c.cumulative[1] += cached
	c.cumulative[2] += output
	c.cumulative[3] += reasoning
	usage := func(in, ca, out, re int64) string {
		return `{"input_tokens":` + strconv.FormatInt(in, 10) + `,"cached_input_tokens":` + strconv.FormatInt(ca, 10) +
			`,"output_tokens":` + strconv.FormatInt(out, 10) + `,"reasoning_output_tokens":` + strconv.FormatInt(re, 10) +
			`,"total_tokens":` + strconv.FormatInt(in+out, 10) + `}`
	}
	c.emit(`{"timestamp":"` + c.now() + `","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":` +
		usage(c.cumulative[0], c.cumulative[1], c.cumulative[2], c.cumulative[3]) + `,"last_token_usage":` +
		usage(input, cached, output, reasoning) + `,"model_context_window":272000},"rate_limits":null}}`)
}

func (c *contentWriter) claudeLine(kind, message string) string {
	id := c.hex(8) + "-" + c.hex(4) + "-4" + c.hex(3) + "-a" + c.hex(3) + "-" + c.hex(12)
	parent := "null"
	if c.parentUUID != "" {
		parent = `"` + c.parentUUID + `"`
	}
	c.parentUUID = id
	sidechain := "false"
	if c.spec.subagent {
		sidechain = "true"
	}
	return `{"parentUuid":` + parent + `,"isSidechain":` + sidechain + `,"userType":"external","cwd":"` + c.cwd() +
		`","sessionId":"` + c.spec.session + `","version":"2.1.0","type":"` + kind + `","message":` + message +
		`,"uuid":"` + id + `","timestamp":"` + c.now() + `"}`
}

func (c *contentWriter) claudeTurn() {
	c.turn++
	if c.rng.IntN(15) == 0 {
		c.modelIndex = c.rng.IntN(len(claudeModels))
	}
	c.emit(c.claudeLine("user", `{"role":"user","content":"`+c.words(4, 40)+`"}`))
	for range c.rng.IntN(4) {
		tool := "toolu_" + c.hex(24)
		c.emit(c.claudeLine("assistant", c.claudeAssistant(`{"type":"tool_use","id":"`+tool+
			`","name":"Bash","input":{"command":"rg -n `+vocabulary[c.rng.IntN(len(vocabulary))]+` internal"}}`)))
		c.emit(c.claudeLine("user", `{"role":"user","content":[{"type":"tool_result","tool_use_id":"`+tool+
			`","content":"`+c.words(20, 400)+`"}]}`))
	}
	c.emit(c.claudeLine("assistant", c.claudeAssistant(`{"type":"text","text":"`+c.words(10, 120)+`"}`)))
}

func (c *contentWriter) claudeAssistant(content string) string {
	input := 1 + c.rng.IntN(400)
	created := c.rng.IntN(8000)
	read := c.rng.IntN(60000)
	output := 20 + c.rng.IntN(2500)
	return `{"id":"msg_` + c.hex(24) + `","type":"message","role":"assistant","model":"` + claudeModels[c.modelIndex] +
		`","content":[` + content + `],"stop_reason":null,"usage":{"input_tokens":` + strconv.Itoa(input) +
		`,"cache_creation_input_tokens":` + strconv.Itoa(created) + `,"cache_read_input_tokens":` + strconv.Itoa(read) +
		`,"output_tokens":` + strconv.Itoa(output) + `}}`
}
