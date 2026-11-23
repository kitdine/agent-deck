package quota

import (
	"context"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

const identitySchemaVersion = 36

type CycleSeq int64
type Generation int64

type RefreshCycle struct {
	owner *Store
	lock  *store.Lock
	seq   CycleSeq
	at    time.Time
}

type IdentityState string

const (
	IdentityIdentified      IdentityState = "identified"
	IdentitySignedOut       IdentityState = "signed_out"
	IdentityNonSubscription IdentityState = "non_subscription"
)

type IdentityFailure string

const (
	IdentityProbeFailed  IdentityFailure = "probe_failed"
	IdentityParseFailed  IdentityFailure = "parse_failed"
	IdentityIncomparable IdentityFailure = "incomparable"
)

type OrganizationKind string

const (
	OrgOrganization OrganizationKind = "organization"
	OrgWorkspace    OrganizationKind = "workspace"
	OrgPersonal     OrganizationKind = "personal"
	OrgUnknown      OrganizationKind = "unknown"
)

type PlanMissingReason string

const (
	PlanNotReported PlanMissingReason = "not_reported"
	PlanUnsafeValue PlanMissingReason = "unsafe_value"
)

type planField struct {
	value    string
	rejected bool
}

type PlanSources struct {
	planType, planTier, subscriptionType planField
}

type PlanSource string
type PlanState string
type PlanBasis string

type PlanRaw struct {
	Field string
	Value string
}

type PlanDisplay struct {
	State           PlanState
	Name, ShortName string
	Basis           PlanBasis
	Source          PlanSource
	Raw             []PlanRaw
	Reason          PlanMissingReason
}

type IdentityDisplay struct {
	EmailMasked      string
	OrganizationKind OrganizationKind
	OrganizationName string
	Plan             PlanSources
}

type claudeComparison struct {
	correlation [32]byte
	keyID       [8]byte
}

type IdentityObservation struct {
	accountKind string
	State       IdentityState
	Failure     IdentityFailure
	Display     IdentityDisplay
	claude      *claudeComparison
}

type IdentityRecord struct {
	AttemptSeq, SuccessSeq, FailureSeq CycleSeq
	AttemptedAt, ObservedAt            time.Time
	BackoffBaseAt, BackoffUntil        time.Time
	Failure                            IdentityFailure
	SuccessState                       IdentityState
	Display                            IdentityDisplay
	AssociationSeq                     CycleSeq
}

type FailureDetail string

const (
	DetailMissingLine        FailureDetail = "missing_line"
	DetailShapeInvalid       FailureDetail = "shape_invalid"
	DetailTimezoneUnresolved FailureDetail = "timezone_unresolved"
)

type UsageFailure struct {
	Trigger                                    Trigger
	Reason                                     Reason
	Detail                                     FailureDetail
	FailureObservedAt, FailureAt, BackoffUntil time.Time
}

type ContextInvalidation string

const (
	InvalidatedContextChanged   ContextInvalidation = "context_changed"
	InvalidatedSignedOut        ContextInvalidation = "signed_out"
	InvalidatedNonSubscription  ContextInvalidation = "non_subscription"
	InvalidatedIdentityKeyReset ContextInvalidation = "identity_key_reset"
)

type CaptureGate string

const (
	CaptureOpenInitial CaptureGate = "open_initial"
	CaptureBlocked     CaptureGate = "blocked"
	CaptureOpen        CaptureGate = "open"
)

type BootStamp struct {
	Boot string
	Mono int64
}

type ClaudeContext struct {
	Generation, ConfirmedGeneration Generation
	Gate                            CaptureGate
	OpenedAt                        BootStamp
	InvalidatedAt                   time.Time
	Invalidation                    ContextInvalidation
}

type UsageRecord struct {
	Envelope          EnvelopeRecord
	Plan              PlanSources
	present           bool
	Seq               CycleSeq
	FailureOrderKnown bool
	Detail            FailureDetail
	Claude            ClaudeContext
}

type ReadVersion struct {
	Schema        int
	CoreEpoch     int64
	Revision      int64
	RevalidateSeq CycleSeq
}

type WriteOutcome uint8

const (
	WriteUnresolved WriteOutcome = iota
	WriteDropped
	WriteCommitted
)

type BeforeClaudeInvalidationCommit func(context.Context, Generation) error

type ClaudeIdentityResult struct {
	Outcome      WriteOutcome
	Generation   Generation
	Invalidation ContextInvalidation
	proof        *claudeReconfirmProof
}

type claudeReconfirmProof struct {
	owner       *Store
	seq         CycleSeq
	correlation [32]byte
}

type codexAssociationProof struct {
	matched bool
}

type CodexCycleResult struct {
	Identity   *IdentityObservation
	Usage      *CodexResult
	Failure    *UsageFailure
	proof      codexAssociationProof
	planUnsafe bool
}
