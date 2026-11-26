package main

// Synthetic native Codex sources for TestCJSISourceIssueAcceptance. P, C, S
// and G are the requirement's main, child, sibling and grandchild threads.
// Every value is synthetic; nothing is read from a real rollout.

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	cjsiAccP = "00000000-0000-4000-8000-00000000a001"
	cjsiAccC = "00000000-0000-4000-8000-00000000a002"
	cjsiAccS = "00000000-0000-4000-8000-00000000a003"
	cjsiAccG = "00000000-0000-4000-8000-00000000a004"
	// cjsiAccX names a canonical file whose valid head is cjsiAccY: a source
	// conflict that must produce no owner.
	cjsiAccX = "00000000-0000-4000-8000-00000000a005"
	cjsiAccY = "00000000-0000-4000-8000-00000000a006"

	cjsiAccDay         = "2026-01-02"
	cjsiAccParentToken = "zebraparent"
	cjsiAccChildToken  = "zebrachild"
)

// cjsiAccParentForm selects which of the two native parent fields a child
// head uses.
type cjsiAccParentForm int

const (
	cjsiAccTopLevelParent cjsiAccParentForm = iota
	cjsiAccThreadSpawnParent
)

type cjsiAccRequest struct {
	input, cached, output, reasoning int64
}

func (r cjsiAccRequest) total() int64 { return r.input + r.output }

func cjsiAccLine(format string, args ...any) string { return fmt.Sprintf(format, args...) + "\n" }

func cjsiAccActive(clock, id string) string {
	return filepath.Join(".codex", "sessions", "2026", "01", "02", "rollout-"+cjsiAccDay+"T"+clock+"-"+id+".jsonl")
}

func cjsiAccArchived(clock, id string) string {
	return filepath.Join(".codex", "archived_sessions", "rollout-"+cjsiAccDay+"T"+clock+"-"+id+".jsonl")
}

// cjsiAccParentHistory is P's own execution: one legacy token_count turn of
// 100 input + 20 output (P = 120). A child file may begin with a copy of it.
func cjsiAccParentHistory() string {
	return cjsiAccLine(`{"timestamp":"%sT08:00:00Z","type":"session_meta","payload":{"id":"%s","timestamp":"%sT08:00:00Z","cwd":"/synthetic-project"}}`, cjsiAccDay, cjsiAccP, cjsiAccDay) +
		cjsiAccLine(`{"timestamp":"%sT08:00:01Z","type":"turn_context","payload":{"turn_id":"parent-turn","model":"parent-model"}}`, cjsiAccDay) +
		cjsiAccLine(`{"timestamp":"%sT08:00:02Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"%s parent prompt synthetic"}]}}`, cjsiAccDay, cjsiAccParentToken) +
		cjsiAccLine(`{"timestamp":"%sT08:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"output_tokens":20},"total_token_usage":{"input_tokens":100,"output_tokens":20}}}}`, cjsiAccDay) +
		cjsiAccLine(`{"timestamp":"%sT08:00:04Z","type":"response_item","payload":{"type":"message","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"parent final synthetic"}]}}`, cjsiAccDay)
}

// cjsiAccChildSource is a native child file: its first complete head names
// the child, then (optionally) inherited parent history with no second child
// head, then the child's own turn with one native token_usage_record.
func cjsiAccChildSource(id, parent string, form cjsiAccParentForm, inherit bool, clock string, req cjsiAccRequest, token string) string {
	relation := fmt.Sprintf(`"parent_thread_id":"%s"`, parent)
	if form == cjsiAccThreadSpawnParent {
		relation = fmt.Sprintf(`"source":{"subagent":{"thread_spawn":{"parent_thread_id":"%s","depth":1}}}`, parent)
	}
	var b strings.Builder
	b.WriteString(cjsiAccLine(`{"timestamp":"%sT%s","type":"session_meta","payload":{"id":"%s","session_id":"%s",%s,"root_turn_id":"root-turn","timestamp":"%sT%s"}}`, cjsiAccDay, clock, id, cjsiAccP, relation, cjsiAccDay, clock))
	if inherit {
		b.WriteString(cjsiAccParentHistory())
		// The copied history also carries P's request in the native format,
		// explicitly naming P as its executing thread. Inside C's file it is
		// foreign evidence: neither C's nor counted again for P.
		b.WriteString(cjsiAccLine(`{"timestamp":"%sT08:00:03Z","type":"token_usage_record","payload":{"thread_id":"%s","turn_id":"parent-turn","session_id":"%s","response_id":"resp-parent","usage":{"input_tokens":100,"output_tokens":20,"total_tokens":120},"turn_token_usage":{"input_tokens":100,"output_tokens":20}}}`, cjsiAccDay, cjsiAccP, cjsiAccP))
	}
	turn := "turn-" + id[len(id)-4:]
	b.WriteString(cjsiAccLine(`{"type":"turn_context","payload":{"turn_id":"%s","model":"child-model"}}`, turn))
	b.WriteString(cjsiAccLine(`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"%s child prompt synthetic"}]}}`, token))
	b.WriteString(cjsiAccLine(`{"timestamp":"%sT%s","type":"token_usage_record","payload":{"thread_id":"%s","turn_id":"%s","session_id":"%s","root_turn_id":"root-turn","response_id":"resp-%s","usage":{"input_tokens":%d,"cached_input_tokens":%d,"cache_write_input_tokens":0,"output_tokens":%d,"reasoning_output_tokens":%d,"total_tokens":%d},"turn_token_usage":{"input_tokens":%d,"output_tokens":%d}}}`,
		cjsiAccDay, clock, id, turn, cjsiAccP, turn, req.input, req.cached, req.output, req.reasoning, req.total(), req.input, req.output))
	b.WriteString(cjsiAccLine(`{"type":"response_item","payload":{"type":"message","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"child final synthetic"}]}}`))
	b.WriteString(cjsiAccLine(`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"%s"}}`, turn))
	return b.String()
}

var (
	cjsiAccChildRequest      = cjsiAccRequest{input: 30, cached: 10, output: 5, reasoning: 2} // C = 35
	cjsiAccSiblingRequest    = cjsiAccRequest{input: 9, output: 4}                            // S = 13
	cjsiAccGrandchildRequest = cjsiAccRequest{input: 8, output: 3}                            // G = 11
)

// cjsiAccFamily is the P/C/S/G corpus: P active; C active with inherited P
// history and the top-level parent field; S archived with the thread_spawn
// parent field; G active as C's child. relative path -> content.
func cjsiAccFamily() map[string]string {
	return map[string]string{
		cjsiAccActive("08-00-00", cjsiAccP):   cjsiAccParentHistory(),
		cjsiAccActive("09-01-00", cjsiAccC):   cjsiAccChildSource(cjsiAccC, cjsiAccP, cjsiAccTopLevelParent, true, "09:01:00Z", cjsiAccChildRequest, cjsiAccChildToken),
		cjsiAccArchived("09-02-00", cjsiAccS): cjsiAccChildSource(cjsiAccS, cjsiAccP, cjsiAccThreadSpawnParent, false, "09:02:00Z", cjsiAccSiblingRequest, "zebrasibling"),
		cjsiAccActive("09-03-00", cjsiAccG):   cjsiAccChildSource(cjsiAccG, cjsiAccC, cjsiAccTopLevelParent, false, "09:03:00Z", cjsiAccGrandchildRequest, "zebragrandchild"),
	}
}

// cjsiAccConflict is a canonical file for X whose first complete head is Y.
func cjsiAccConflict() (string, string) {
	return cjsiAccActive("09-05-00", cjsiAccX), cjsiAccChildSource(cjsiAccY, cjsiAccP, cjsiAccTopLevelParent, false, "09:05:00Z", cjsiAccRequest{input: 1, output: 1}, "zebraconflict")
}

// cjsiAccOTelExport is one synthetic OTLP JSON log export for a Codex
// conversation; no credential, prompt or path is included.
func cjsiAccOTelExport(conversation string, input, output int64) string {
	attr := func(key, value string) string {
		return fmt.Sprintf(`{"key":%q,"value":{"stringValue":%q}}`, key, value)
	}
	count := func(key string, value int64) string {
		return fmt.Sprintf(`{"key":%q,"value":{"intValue":"%d"}}`, key, value)
	}
	resource := "[" + attr("service.name", "codex-app-server") + "," + attr("service.version", "0.165.0") + "]"
	record := "[" + strings.Join([]string{
		attr("event.name", "codex.sse_event"), attr("event.kind", "response.completed"), attr("model", "child-model"),
		attr("originator", "codex-tui"), attr("conversation.id", conversation),
		count("input_token_count", input), count("output_token_count", output), count("cached_token_count", 0),
	}, ",") + "]"
	return fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":%s},"scopeLogs":[{"logRecords":[{"timeUnixNano":"1767344640000000000","body":{"stringValue":"x"},"attributes":%s}]}]}]}`, resource, record)
}

// cjsiAccCoreCopy opens a private copy of the core database (main and WAL)
// so an observation never adds a connection, WAL or shared-memory file to the
// fixture root.
func cjsiAccCoreCopy(t *testing.T, state string) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"agentdeck.sqlite3", "agentdeck.sqlite3-wal"} {
		content, _, err := cjsiAccReadRegular(filepath.Join(state, name), cjsiAccStateReadLimit)
		if err != nil {
			if os.IsNotExist(err) && name != "agentdeck.sqlite3" {
				continue
			}
			t.Fatalf("core copy: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	uri := url.URL{Scheme: "file", Path: filepath.Join(dir, "agentdeck.sqlite3")}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// cjsiAccCoreQuery runs one single-column query on a private core copy.
func cjsiAccCoreQuery(t *testing.T, state, query string) []string {
	t.Helper()
	db := cjsiAccCoreCopy(t, state)
	defer db.Close()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value sql.NullString
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		values = append(values, value.String)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return values
}
