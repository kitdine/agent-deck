package quota

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/buildinfo"
)

const codexIdentityTimeout = 2 * time.Second

// ProbeCodexCycle returns only due sides. The scheduler supplies trigger and
// retry timestamps on Failure before CommitCodexCycle, as for other probes.
func ProbeCodexCycle(ctx context.Context, observedAt time.Time, identityDue, usageDue bool) CodexCycleResult {
	result, _ := probeCodexCycle(ctx, observedAt, identityDue, usageDue, DefaultCodexTimeout, codexIdentityTimeout, nil)
	return result
}

type codexReply struct {
	raw     json.RawMessage
	failure *CodexFailureError
	at      time.Time
}

// Only registered requests receive replies. The scanner never blocks on a
// consumer that timed out, and never decodes unattended result payloads.
type codexDispatch struct {
	mu       sync.Mutex
	wait     map[int]chan codexReply
	terminal *CodexFailureError
}

func (d *codexDispatch) fail(kind CodexFailureKind) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.terminal = &CodexFailureError{Kind: kind}
	for id, ch := range d.wait {
		ch <- codexReply{failure: d.terminal}
		delete(d.wait, id)
	}
}

func (d *codexDispatch) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e struct {
			ID, Method, Result, Error json.RawMessage
		}
		if line[0] != '{' || json.Unmarshal(line, &e) != nil {
			d.fail(CodexFailureMalformed)
			return
		}
		if len(e.Method) != 0 || len(e.ID) == 0 {
			continue
		}
		var id *int
		if json.Unmarshal(e.ID, &id) != nil {
			d.fail(CodexFailureMalformed)
			return
		}
		if id == nil {
			continue
		}
		d.mu.Lock()
		if ch := d.wait[*id]; ch != nil {
			reply := codexReply{raw: e.Result, at: time.Now()}
			if len(e.Error) > 0 && !bytes.Equal(e.Error, []byte("null")) {
				reply.raw = nil
				reply.failure = &CodexFailureError{Kind: CodexFailureServerError}
			}
			delete(d.wait, *id)
			ch <- reply
		}
		d.mu.Unlock()
	}
	d.fail(CodexFailureExit)
}

func (d *codexDispatch) request(ctx context.Context, stdin io.Writer, id int, method string, params any, budget time.Duration) codexReply {
	ch := make(chan codexReply, 1)
	d.mu.Lock()
	if d.terminal != nil {
		failure := d.terminal
		d.mu.Unlock()
		return codexReply{failure: failure}
	}
	d.wait[id] = ch
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.wait, id); d.mu.Unlock() }()
	if json.NewEncoder(stdin).Encode(struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params,omitempty"`
	}{id, method, params}) != nil {
		return codexReply{failure: &CodexFailureError{Kind: CodexFailureExit}}
	}
	if budget > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}
	select {
	case reply := <-ch:
		deadline, limited := ctx.Deadline()
		if ctx.Err() == nil && (!limited || reply.at.IsZero() || reply.at.Before(deadline)) {
			return reply
		}
	case <-ctx.Done():
	}
	return codexReply{failure: &CodexFailureError{Kind: CodexFailureDeadline}}
}

// keyReader is nil for the certified production schema: userId is not the
// rate-limits accountId. Only a schema with a certified equivalent may
// supply an extractor. Its bytes never leave this adapter.
type codexKeyReader func(json.RawMessage) []byte

func probeCodexCycle(parent context.Context, at time.Time, identity, usage bool, base, sub time.Duration, keyReader codexKeyReader) (CodexCycleResult, *CodexFailureError) {
	if !identity && !usage {
		return CodexCycleResult{}, nil
	}
	errReply := func(k CodexFailureKind) codexReply { return codexReply{failure: &CodexFailureError{Kind: k}} }
	var ir, ur codexReply
	total := base
	if identity && usage {
		total += sub
	}
	ctx, cancel := context.WithTimeout(parent, total)
	defer cancel()
	cmd := newCodexCommand(ctx)
	cmd.WaitDelay = 500 * time.Millisecond
	var killedAtDeadline atomic.Bool
	cmd.Cancel = func() error {
		err := cmd.Process.Kill()
		if err == nil && ctx.Err() == context.DeadlineExceeded {
			killedAtDeadline.Store(true)
		}
		return err
	}
	// Own the read descriptor so Wait and the scanner can run independently;
	// closing it after Wait also wakes a reader if a descendant inherited stdout.
	transport := func() CodexFailureKind {
		in, err := cmd.StdinPipe()
		if err != nil {
			return CodexFailureSpawn
		}
		defer in.Close()
		r, w, err := os.Pipe()
		if err != nil {
			return CodexFailureSpawn
		}
		defer r.Close()
		cmd.Stdout = w
		err = cmd.Start()
		w.Close()
		if err != nil {
			return CodexFailureSpawn
		}
		d := codexDispatch{wait: map[int]chan codexReply{}}
		done := make(chan struct{})
		go func() { defer close(done); d.read(r) }()
		init := d.request(ctx, in, 1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "agentdeck", "version": buildinfo.Version}}, 0)
		if init.failure != nil {
			ir, ur = init, init
		} else if len(init.raw) == 0 || init.raw[0] != '{' {
			ir, ur = errReply(CodexFailureMalformed), errReply(CodexFailureMalformed)
		} else if _, err = io.WriteString(in, "{\"method\":\"initialized\"}\n"); err != nil {
			ir, ur = errReply(CodexFailureExit), errReply(CodexFailureExit)
		} else {
			if identity {
				ir = d.request(ctx, in, 3, "account/read", map[string]bool{"refreshToken": false}, sub)
			}
			if usage {
				ur = d.request(ctx, in, 2, "account/rateLimits/read", nil, 0)
			}
		}
		in.Close()
		err = cmd.Wait()
		r.Close()
		<-done
		if err != nil {
			// An expired context alone does not establish why the process
			// exited. A deadline kill must also match its observed status.
			if killedAtDeadline.Load() && cmd.ProcessState != nil {
				if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signal() == syscall.SIGKILL {
					return CodexFailureDeadline
				}
			}
			return CodexFailureExit
		}
		return 0
	}()
	if transport == CodexFailureDeadline && usage && ur.at.IsZero() {
		// A deadline while waiting for usage leaves the earlier identity
		// outcome intact. EOF can race the deadline after the child is killed.
		if ur.failure == nil || ur.failure.Kind == CodexFailureExit {
			ur = errReply(transport)
		}
	} else if transport != 0 {
		ir, ur = errReply(transport), errReply(transport)
	}
	if parent.Err() != nil {
		ir, ur = errReply(CodexFailureDeadline), errReply(CodexFailureDeadline)
	}
	var result CodexCycleResult
	var account struct {
		Account            *struct{ Type, Email, WorkspaceName, PlanType string }
		RequiresOpenaiAuth *bool
		UserID             string // deny-list only; never an association key
	}
	var key []byte
	decodeErr := json.Unmarshal(ir.raw, &account)
	if ir.failure == nil && keyReader != nil {
		key = keyReader(ir.raw)
	}
	var limits struct{ AccountID string }
	_ = json.Unmarshal(ur.raw, &limits)
	var email, local string
	if account.Account != nil {
		email = account.Account.Email
		local, _, _ = strings.Cut(email, "@")
	}
	forbidden := newIdentityForbiddenSet(local, email, account.UserID, limits.AccountID, string(key))
	if identity {
		obs := IdentityObservation{Failure: IdentityIncomparable}
		switch {
		case ir.failure != nil:
			obs.Failure = IdentityProbeFailed
			if ir.failure.Kind == CodexFailureMalformed {
				obs.Failure = IdentityParseFailed
			}
		case decodeErr != nil || account.RequiresOpenaiAuth == nil:
			obs.Failure = IdentityParseFailed
		case account.Account == nil:
			if *account.RequiresOpenaiAuth {
				obs = IdentityObservation{State: IdentitySignedOut}
			}
		case account.Account.Type == "apiKey":
			obs = IdentityObservation{State: IdentityNonSubscription}
		case account.Account.Type == "chatgpt":
			if _, _, masked, ok := maskedIdentityEmail(email); ok {
				a := account.Account
				org := OrgUnknown
				if a.WorkspaceName != "" {
					org = OrgWorkspace
				}
				obs = IdentityObservation{State: IdentityIdentified, accountKind: "chatgpt", Display: IdentityDisplay{EmailMasked: masked, OrganizationKind: org, OrganizationName: sanitizeIdentityName(a.WorkspaceName, forbidden), Plan: PlanSources{planType: sanitizeIdentityPlan(a.PlanType, forbidden)}}}
			}
		}
		result.Identity = &obs
	}
	if usage {
		if ur.failure == nil {
			value, unsafe, err := parseScreenedLimits(ur.raw, at, forbidden)
			if err != nil {
				ur = errReply(CodexFailureMalformed)
			} else {
				result.Usage = &value
				result.planUnsafe = unsafe
				result.proof.matched = result.Identity != nil && result.Identity.State == IdentityIdentified && len(key) > 0 && bytes.Equal(key, []byte(value.AccountID))
			}
		}
		if ur.failure != nil {
			result.Failure = &UsageFailure{Reason: ur.failure.Kind.Reason(), FailureObservedAt: at}
		}
	}
	return result, ur.failure
}
