package usage

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

// Decision 7's two kinds this file keys on. `edit` is the edit-shaped call on
// both clients; `bash` is the shell call that is neither an edit nor read-shaped
// — on Codex that is exactly what the ported allowlist decided, because a shell
// call whose parsed commands are all read-only lands as `read` instead.
const (
	toolKindEdit = "edit"
	toolKindBash = "bash"
)

// SignalScope is the client and period a Decision 6 metric is computed over. A
// turn is in scope when its `classified` signal row's started_at falls in
// [From, To). A `pending` row is a turn that does not exist yet and is invisible
// to every aggregate here, which is why the scope is anchored on that table
// rather than on the tool calls or events a turn happens to carry.
type SignalScope struct {
	From     time.Time
	To       time.Time
	Client   string
	Session  string
	Activity string
}

// WorkflowMetrics carries Decision 6's five metrics for one scope.
//
// Every field is a pointer because each metric's unavailable condition is not
// zero and must not read as zero. A scope whose sessions reached an edit but
// never reworked a file reports `Retries` as a pointer to 0; a scope no session
// reached an edit in reports it as nil. Rendering the two the same way would
// claim a measurement that was never taken, which is what
// `requirements.md` acceptance 4 is about.
type WorkflowMetrics struct {
	FirstEditSeconds *int     `json:"first_edit_seconds"`
	FilesTouched     *int     `json:"files_touched"`
	Retries          *int     `json:"retries"`
	EditsPerSession  *float64 `json:"edits_per_session"`
	TopFile          *string  `json:"top_file"`
	TopFileEdits     *int     `json:"top_file_edits"`
}

// workflowCall is one tool call in scope, carrying only the written files of
// Decision 6. A call that both reads and writes contributes its written rows and
// nothing else, which is why `usage_tool_files.wrote` is filtered in the query
// rather than inspected here.
type workflowCall struct {
	sessionKey  string
	client      string
	toolName    string
	toolKind    string
	startedAt   string
	commandRead bool
	files       []workflowFile
}

type workflowFile struct {
	digest   string
	baseName string
}

// WorkflowMetrics computes Decision 6's five metrics over one scope.
func (s *Service) WorkflowMetrics(ctx context.Context, scope SignalScope) (WorkflowMetrics, error) {
	sessionStarts, err := s.workflowSessionStarts(ctx, scope)
	if err != nil {
		return WorkflowMetrics{}, err
	}
	calls, err := s.workflowCalls(ctx, scope)
	if err != nil {
		return WorkflowMetrics{}, err
	}

	writtenRows := 0
	editCounts := map[string]int{}
	baseNames := map[string]string{}
	firstEdit := map[string]string{}
	rework := 0
	reworkState := map[string]map[string]bool{}

	for _, call := range calls {
		if call.toolKind == toolKindEdit || len(call.files) > 0 {
			if at, seen := firstEdit[call.sessionKey]; !seen || call.startedAt < at {
				firstEdit[call.sessionKey] = call.startedAt
			}
		}
		if reworkState[call.sessionKey] == nil {
			reworkState[call.sessionKey] = map[string]bool{}
		}
		rework += observeRework(reworkState[call.sessionKey], call)
		for _, file := range call.files {
			writtenRows++
			editCounts[file.digest]++
			if _, known := baseNames[file.digest]; !known {
				baseNames[file.digest] = file.baseName
			}
		}
	}

	var metrics WorkflowMetrics
	if len(sessionStarts) > 0 {
		perSession := float64(writtenRows) / float64(len(sessionStarts))
		metrics.EditsPerSession = &perSession
	}
	if writtenRows > 0 {
		touched := len(editCounts)
		metrics.FilesTouched = &touched
		digest := topWrittenDigest(editCounts, baseNames)
		name, edits := baseNames[digest], editCounts[digest]
		metrics.TopFile, metrics.TopFileEdits = &name, &edits
	}
	if len(firstEdit) > 0 {
		// `retries` and `first_edit_seconds` share one unavailable condition —
		// no session in scope reached an edit — so they are decided together.
		// A scope that reached an edit and reworked nothing reports 0.
		reworkTotal := rework
		metrics.Retries = &reworkTotal
		if seconds, ok := medianFirstEditSeconds(sessionStarts, firstEdit); ok {
			metrics.FirstEditSeconds = &seconds
		}
	}
	return metrics, nil
}

// observeRework advances one session's rework state for a single call and
// returns how many reworks it closed.
//
// The pattern is Decision 6's: a file is edited, a non-read-shaped shell command
// runs, and the same file is edited again. The window is the session rather than
// the turn, so `state` is carried across every call in the session and a fix
// spanning two turns is counted.
//
// The barrier deliberately skips the files this same call wrote. A command that
// itself produced the file is not an independent step between two edits of it,
// and letting it be one would turn a two-event sequence into a three-event
// pattern.
func observeRework(state map[string]bool, call workflowCall) int {
	if call.isNonReadShell() {
		written := map[string]bool{}
		for _, file := range call.files {
			written[file.digest] = true
		}
		for digest, verified := range state {
			if !verified && !written[digest] {
				state[digest] = true
			}
		}
	}
	closed := 0
	for _, file := range call.files {
		if state[file.digest] {
			closed++
		}
		// The second edit opens the next candidate cycle rather than ending the
		// file's history, so a file reworked twice counts twice.
		state[file.digest] = false
	}
	return closed
}

func (call workflowCall) isNonReadShell() bool {
	if call.commandRead {
		return false
	}
	if call.client == "claude" {
		return call.toolName == "Bash"
	}
	switch call.toolName {
	case "exec_command", "exec", "js", "write_stdin":
		return true
	default:
		return false
	}
}

// topWrittenDigest picks Decision 6's most frequent written path, breaking ties
// by base name and then by digest. Both tiebreaks are needed: two files can
// share a base name, and without a total order the value would move between runs
// on a map iteration order. This is Decision 7's rule for the top MCP server
// applied to the same problem.
func topWrittenDigest(counts map[string]int, baseNames map[string]string) string {
	digests := make([]string, 0, len(counts))
	for digest := range counts {
		digests = append(digests, digest)
	}
	sort.Slice(digests, func(i, j int) bool {
		left, right := digests[i], digests[j]
		if counts[left] != counts[right] {
			return counts[left] > counts[right]
		}
		if baseNames[left] != baseNames[right] {
			return baseNames[left] < baseNames[right]
		}
		return left < right
	})
	if len(digests) == 0 {
		return ""
	}
	return digests[0]
}

// medianFirstEditSeconds reduces the per-session first-edit delays to Decision
// 6's median. A session whose timestamps do not parse contributes nothing rather
// than a synthesized value, on the same reasoning that keeps a signal row from
// being written with a synthesized `started_at`.
func medianFirstEditSeconds(sessionStarts map[string]string, firstEdit map[string]string) (int, bool) {
	var values []int
	for sessionKey, editAt := range firstEdit {
		startText, known := sessionStarts[sessionKey]
		if !known {
			continue
		}
		start, startErr := time.Parse(time.RFC3339Nano, startText)
		edit, editErr := time.Parse(time.RFC3339Nano, editAt)
		if startErr != nil || editErr != nil {
			continue
		}
		seconds := int(edit.Sub(start).Seconds())
		if seconds < 0 {
			// The two timestamps come from different log entries and nothing
			// guarantees a monotonic clock across them. A negative delay is
			// reported as an immediate edit, never as a negative duration.
			seconds = 0
		}
		values = append(values, seconds)
	}
	if len(values) == 0 {
		return 0, false
	}
	sort.Ints(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle], true
	}
	// An even count averages the two middle values and rounds half up, so the
	// metric stays an integer number of seconds on both surfaces.
	return (values[middle-1] + values[middle] + 1) / 2, true
}

func workflowSessionKey(client, session string) string {
	return client + "\x00" + session
}

// workflowSessionStarts returns each in-scope session's first classified turn
// time, keyed by client and session. It is both the denominator of
// `edits_per_session` and the origin `first_edit_seconds` measures from.
func (s *Service) workflowSessionStarts(ctx context.Context, scope SignalScope) (map[string]string, error) {
	query := `SELECT client,session_id,MIN(started_at) FROM usage_work_signals WHERE state=? AND started_at>=? AND started_at<?`
	args := []any{signalStateClassified, scope.From.UTC().Format(time.RFC3339Nano), scope.To.UTC().Format(time.RFC3339Nano)}
	if scope.Client != "" {
		query += ` AND client=?`
		args = append(args, scope.Client)
	}
	if scope.Session != "" {
		query += ` AND session_id=?`
		args = append(args, scope.Session)
	}
	if scope.Activity != "" {
		query += ` AND (activity_kind=? OR activity_sub=?)`
		args = append(args, scope.Activity, scope.Activity)
	}
	query += ` GROUP BY client,session_id`
	rows, err := s.Store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	starts := map[string]string{}
	for rows.Next() {
		var client, session, startedAt string
		if err = rows.Scan(&client, &session, &startedAt); err != nil {
			return nil, err
		}
		starts[workflowSessionKey(client, session)] = startedAt
	}
	return starts, rows.Err()
}

// workflowCalls streams every tool call belonging to an in-scope classified
// turn, in the order it ran, each carrying its written files.
//
// The join to `usage_work_signals` is what makes `pending` rows invisible: a
// turn with no classified row contributes no call, however many tool rows it
// already has. The left join to `usage_tool_files` keeps calls that wrote
// nothing in the stream, because they can still be the barrier a rework needs.
func (s *Service) workflowCalls(ctx context.Context, scope SignalScope) ([]workflowCall, error) {
	query := `SELECT c.client,c.session_id,c.activity_key,c.tool_name,c.tool_kind,c.started_at,c.command_read,f.path_digest,f.base_name
FROM usage_tool_calls c
JOIN usage_work_signals w ON w.client=c.client AND w.session_id=c.session_id AND w.turn_index=c.turn_index
LEFT JOIN usage_tool_files f ON f.activity_key=c.activity_key AND f.wrote=1
WHERE w.state=? AND w.started_at>=? AND w.started_at<?`
	args := []any{signalStateClassified, scope.From.UTC().Format(time.RFC3339Nano), scope.To.UTC().Format(time.RFC3339Nano)}
	if scope.Client != "" {
		query += ` AND w.client=?`
		args = append(args, scope.Client)
	}
	if scope.Session != "" {
		query += ` AND w.session_id=?`
		args = append(args, scope.Session)
	}
	if scope.Activity != "" {
		query += ` AND (w.activity_kind=? OR w.activity_sub=?)`
		args = append(args, scope.Activity, scope.Activity)
	}
	query += ` ORDER BY c.started_at,c.source_offset,c.activity_key,f.path_digest`
	rows, err := s.Store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var calls []workflowCall
	byKey := map[string]int{}
	for rows.Next() {
		var client, session, activityKey, toolName, toolKind, startedAt string
		var commandRead bool
		var digest, baseName sql.NullString
		if err = rows.Scan(&client, &session, &activityKey, &toolName, &toolKind, &startedAt, &commandRead, &digest, &baseName); err != nil {
			return nil, err
		}
		index, seen := byKey[activityKey]
		if !seen {
			calls = append(calls, workflowCall{
				sessionKey:  workflowSessionKey(client, session),
				client:      client,
				toolName:    toolName,
				toolKind:    toolKind,
				startedAt:   startedAt,
				commandRead: commandRead,
			})
			index = len(calls) - 1
			byKey[activityKey] = index
		}
		if digest.Valid && digest.String != "" {
			calls[index].files = append(calls[index].files, workflowFile{digest: digest.String, baseName: baseName.String})
		}
	}
	return calls, rows.Err()
}
