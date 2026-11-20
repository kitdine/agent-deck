package quota

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeInitialize = `IFS= read -r line || exit 1
case "$line" in *initialize*) ;; *) exit 1;; esac
echo '{"id":1,"result":{}}'
IFS= read -r line || exit 1
case "$line" in *initialized*) ;; *) exit 1;; esac
`

const fakeAccountRequest = `IFS= read -r line || exit 1
case "$line" in *account/read*'"refreshToken":false'*) ;; *) exit 1;; esac
`

const fakeUsageRequest = `IFS= read -r line || exit 1
case "$line" in *account/rateLimits/read*) ;; *) exit 1;; esac
`

const fakeLimits = `echo '{"id":2,"result":{"accountId":"acct-private","rateLimits":{"planType":"prolite","limitName":"workspace-private", "primary":{"usedPercent":33}}}}'
`

func TestCodexIdentitySchemaClassification(t *testing.T) {
	for _, tc := range []struct {
		name, account string
		state         IdentityState
		failure       IdentityFailure
	}{
		{"signed out", `null`, IdentitySignedOut, ""},
		{"API key", `{"type":"apiKey"}`, IdentityNonSubscription, ""},
		{"new auth kind", `{"type":"gcp"}`, "", IdentityIncomparable},
		{"unknown kind", `{"type":"novel"}`, "", IdentityIncomparable},
		{"invalid email", `{"type":"chatgpt","email":"private"}`, "", IdentityIncomparable},
		{"wrong type", `{"type":"chatgpt","email":7}`, "", IdentityParseFailed},
		{"masked", `{"type":"chatgpt","email":"tester@example.test","planType":"prolite","workspaceName":" Private   workspace "}`, IdentityIdentified, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withFakeCodex(t, fakeInitialize+fakeAccountRequest+`echo '{"id":3,"result":{"requiresOpenaiAuth":true,"account":`+tc.account+`}}'
if read -r unexpected; then exit 1; fi`)
			v := ProbeCodexCycle(context.Background(), time.Now(), true, false)
			if v.Identity == nil || v.Identity.State != tc.state || v.Identity.Failure != tc.failure || v.Usage != nil || v.Failure != nil || v.proof.matched {
				t.Fatal("classification or due-side mismatch")
			}
			if tc.state == IdentityIdentified {
				d := v.Identity.Display
				p := ProjectPlan(PlanSourceCodexAccountRead, d.Plan)
				if d.EmailMasked != "t***r@example.test" || d.OrganizationName != "Private workspace" || p.Name != "ChatGPT Pro 100" || p.ShortName != "Pro 100" || p.Raw[0].Value != "prolite" {
					t.Fatal("unsafe or incorrect identity projection", d, p)
				}
			}
		})
	}
}

func TestCodexIdentityCertifiedAssociationAndScreening(t *testing.T) {
	const rpc = `echo '{"id":3,"result":{"requiresOpenaiAuth":true,"userId":"workspace-private","account":{"type":"chatgpt","email":"tester@example.test","planType":"acct-private","workspaceName":"tester group","syntheticKey":"acct-private"}}}'
`
	for _, mode := range []string{"schema-without-key", "certified", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			reply := rpc
			var certified codexKeyReader
			if mode != "schema-without-key" {
				certified = func(raw json.RawMessage) []byte {
					var a struct{ Account struct{ SyntheticKey string } }
					if json.Unmarshal(raw, &a) != nil {
						return nil
					}
					return []byte(a.Account.SyntheticKey)
				}
			}
			if mode == "conflict" {
				reply = strings.Replace(reply, `"syntheticKey":"acct-private"`, `"syntheticKey":"Acct-private"`, 1)
			}
			withFakeCodex(t, fakeInitialize+fakeAccountRequest+reply+fakeUsageRequest+fakeLimits)
			v, e := probeCodexCycle(context.Background(), time.Now(), true, true, time.Second, time.Second, certified)
			if e != nil || v.Usage == nil || v.Identity == nil || v.Identity.State != IdentityIdentified || v.proof.matched != (mode == "certified") {
				t.Fatal("unexpected joint result", e)
			}
			if v.Usage.Plan != "prolite" || v.Usage.Windows[0].Label != "" || !v.Identity.Display.Plan.planType.rejected || v.Identity.Display.OrganizationName != "" {
				t.Fatal("cross-response screening failed")
			}
		})
	}
	for _, key := range []string{"safe", "first@example.test", "acct-private"} {
		raw := strings.ReplaceAll(`{"accountId":"acct-private","rateLimits":{"planType":"acct-private"},"rateLimitsByLimitId":{"KEY":{"limitName":"acct-private","primary":{"usedPercent":40}}},"rateLimitResetCredits":{"credits":[{"title":"acct-private bonus","status":"first@example.test"}]}}`, "KEY", key)
		v, unsafe, err := parseScreenedLimits([]byte(raw), time.Now(), nil)
		if key != "safe" {
			if err == nil || strings.Contains(err.Error(), key) || len(v.Windows) != 0 {
				t.Fatal("unsafe key escaped")
			}
			continue
		}
		if err != nil || !unsafe || v.Plan != "" || v.PlanReason != ReasonNotReported || v.Windows[0].Label != "" || v.ResetAllowance.Credits[0].Title != "" || v.ResetAllowance.Credits[0].Status != "" {
			t.Fatal("legacy screening failed", err)
		}
	}
}

func TestCodexIdentityDeadlinesAndFatalEvents(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		identity     IdentityFailure
		usage        Reason
	}{
		{"late", fakeInitialize + fakeAccountRequest + fakeUsageRequest + `echo '{"id":3,"result":{"invalid":"late-private"}}'
sleep 0.09
` + fakeLimits, IdentityProbeFailed, ""},
		{"silent", fakeInitialize + fakeAccountRequest + fakeUsageRequest + fakeLimits, IdentityProbeFailed, ""},
		{"before handshake", "exit 0", IdentityProbeFailed, ReasonProbeFailed},
		{"malformed handshake", "echo 'private !'", IdentityParseFailed, ReasonParseFailed},
		{"malformed identity", fakeInitialize + fakeAccountRequest + `echo '{"id":3,"result":[]}'
` + fakeUsageRequest + fakeLimits, IdentityParseFailed, ""},
		{"RPC refusal", fakeInitialize + fakeAccountRequest + `echo '{"id":3,"error":{"message":"private!"}}'
` + fakeUsageRequest + fakeLimits, IdentityProbeFailed, ""},
		{"crash after replies", fakeInitialize + fakeAccountRequest + `echo '{"id":3,"result":{"requiresOpenaiAuth":true,"account":null}}'
` + fakeUsageRequest + fakeLimits + "exit 7", IdentityProbeFailed, ReasonProbeFailed},
		{"signal after replies", fakeInitialize + fakeAccountRequest + `echo '{"id":3,"result":{"requiresOpenaiAuth":true,"account":null}}'
` + fakeUsageRequest + fakeLimits + `kill -TERM "$$"`, IdentityProbeFailed, ReasonProbeFailed},
		{"second side malformed", fakeInitialize + fakeAccountRequest + `echo '{"id":3,"result":{"requiresOpenaiAuth":true,"account":null}}'
` + fakeUsageRequest + "echo 'private malformed'", "", ReasonParseFailed},
		{"server request shares ID", fakeInitialize + fakeAccountRequest + `echo '{"id":3,"method":"server/request","params":{}}'
echo '{"id":3,"result":{"requiresOpenaiAuth":true,"account":null}}'
` + fakeUsageRequest + fakeLimits, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withFakeCodex(t, tc.script)
			v, err := probeCodexCycle(context.Background(), time.Now(), true, true, 150*time.Millisecond, 100*time.Millisecond, nil)
			if v.Identity == nil || v.Identity.Failure != tc.identity {
				t.Fatal("identity mismatch", v.Identity)
			}
			if tc.identity == "" && v.Identity.State != IdentitySignedOut {
				t.Fatal("lost independent successful identity")
			}
			if tc.usage == "" {
				if err != nil || v.Usage == nil || v.Failure != nil {
					t.Fatal("usage should succeed", err)
				}
			} else if err == nil || strings.Contains(err.Error(), "private") || v.Usage != nil || v.Failure.Reason != tc.usage {
				t.Fatal("unexpected failure", err)
			}
		})
	}
}

func TestCodexIdentitySurvivesUsageDeadline(t *testing.T) {
	withFakeCodex(t, fakeInitialize+fakeAccountRequest+`echo '{"id":3,"result":{"requiresOpenaiAuth":true,"account":{"type":"chatgpt","email":"tester@example.test"}}}'
`+fakeUsageRequest+"exec sleep 10")

	cycle, err := probeCodexCycle(context.Background(), time.Now(), true, true, 700*time.Millisecond, 300*time.Millisecond, nil)
	if err == nil || err.Kind != CodexFailureDeadline || cycle.Usage != nil || cycle.Failure == nil || cycle.Failure.Reason != ReasonProbeFailed {
		t.Fatalf("usage timeout = %#v, %v", cycle.Failure, err)
	}
	if cycle.Identity == nil || cycle.Identity.State != IdentityIdentified || cycle.Identity.Failure != "" {
		t.Fatalf("usage timeout lost the completed identity: %#v", cycle.Identity)
	}
	if cycle.Identity.Display.EmailMasked != "t***r@example.test" || cycle.proof.matched {
		t.Fatal("unexpected partial identity projection")
	}
}

func TestCodexIdentityCancellationAndZeroDue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_FIXTURE", dir)
	withFakeCodex(t, `echo started > "$CODEX_FIXTURE/started"
exec sleep 10`)
	if r := ProbeCodexCycle(context.Background(), time.Now(), false, false); r != (CodexCycleResult{}) {
		t.Fatal("unexpected result")
	}
	if _, err := os.Stat(filepath.Join(dir, "started")); !os.IsNotExist(err) {
		t.Fatal("spawned with neither side due")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan CodexCycleResult, 1)
	go func() { done <- ProbeCodexCycle(ctx, time.Now(), true, true) }()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture not started")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case r := <-done:
		if r.Identity.Failure != IdentityProbeFailed || r.Failure.Reason != ReasonProbeFailed || r.Usage != nil {
			t.Fatal("unexpected cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled process did not settle")
	}
}

func TestCodexIdentityInheritedOutputDoesNotDelayWait(t *testing.T) {
	// A descendant retains stdout and waits for explicit release from the
	// fixture. Waiting for EOF before reaping the direct child would deadlock.
	dir := t.TempDir()
	t.Setenv("CODEX_FIXTURE", dir)
	withFakeCodex(t, fakeInitialize+fakeUsageRequest+fakeLimits+`
(while [ ! -e "$CODEX_FIXTURE/release" ]; do sleep .02; done
touch "$CODEX_FIXTURE/done") &
exit 0`)
	defer func() {
		if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0600); err != nil {
			t.Fatal(err)
		}
		end := time.Now().Add(2 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(dir, "done")); err == nil {
				return
			}
			if time.Now().After(end) {
				t.Fatal("descendant did not settle")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	start := time.Now()
	value, err := ProbeCodex(context.Background(), start, 2*time.Second)
	if err != nil || value.Plan != "prolite" || time.Since(start) >= time.Second {
		t.Fatal("inherited stdout delayed probe", err)
	}
}

func TestCodexIdentityPrivateValuesNotCommitted(t *testing.T) {
	ctx, s, lock, _ := newIdentityStore(t)
	at := time.Now().UTC()
	cycle := startIdentityCycle(t, ctx, s, lock, at)
	withFakeCodex(t, fakeInitialize+fakeAccountRequest+`echo '{"id":3,"result":{"requiresOpenaiAuth":true,"account":{"type":"chatgpt","email":"privateperson@example.test","workspaceName":"privateperson team","planType":"privateacct"}}}'
`+fakeUsageRequest+`echo '{"id":2,"result":{"accountId":"privateacct","rateLimits":{"planType":"privateacct","primary":{"usedPercent":33},"limitName":"privateperson"},"rateLimitResetCredits":{"credits":[{"title":"privateperson bonus","status":"privateacct"}]}}}'`)
	result := ProbeCodexCycle(ctx, at, true, true)
	out, err := s.CommitCodexCycle(ctx, cycle, result, time.Time{}, time.Time{})
	requireQuotaOutcome(t, out, err, WriteCommitted)
	var account, plan, reason, credits, label, windowAccount, masked, name, sources, association string
	var unsafe bool
	if err := s.db.QueryRow(`SELECT account_id,plan,plan_reason,plan_unsafe,reset_credits_json FROM quota_envelopes_v2 WHERE client='codex'`).Scan(&account, &plan, &reason, &unsafe, &credits); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT account_id,label FROM quota_windows_v2 WHERE client='codex'`).Scan(&windowAccount, &label); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT email_masked,org_name,plan_sources_json,association FROM quota_identities WHERE client='codex'`).Scan(&masked, &name, &sources, &association); err != nil {
		t.Fatal(err)
	}
	if plan != "" || reason != "not_reported" || !unsafe || label != "" || name != "" || association != "" || masked != "p***n@example.test" || windowAccount != account || account == "" {
		t.Fatal("unsafe persisted projection")
	}
	for _, v := range []string{account, credits, sources} {
		if strings.Contains(v, "privateacct") || strings.Contains(v, "privateperson") {
			t.Fatal("private sentinel persisted")
		}
	}
}
