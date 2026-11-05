package sessionorigin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Publication reasons for a selection view that is stale or unavailable.
const (
	PublicationReasonStale                     = "selection_stale"
	PublicationReasonSessionsUnavailable       = "sessions_unavailable"
	PublicationReasonClassificationUnavailable = "classification_unavailable"
	PublicationReasonGenerationUnavailable     = "generation_unavailable"
)

// Publication is one complete, same-generation selection result. Its ID is a
// digest of what it answers and the generation it was read at, so the same
// selection read at the same vector always has the same identity.
type Publication struct {
	ID          string
	GeneratedAt string
	Snapshot    Snapshot
}

// NewPublication identifies a verified snapshot.
func NewPublication(snapshot Snapshot, now time.Time) Publication {
	lines := []string{"session-scope-publication-v1", "reducer=" + ReducerVersion, "rules=" + strings.Join(ruleVersions, ","),
		"selection=" + snapshot.Selection.key(),
		"core_epoch=" + strconv.FormatInt(snapshot.Vector.CoreEpoch, 10),
		"usage_revision=" + strconv.FormatInt(snapshot.Vector.UsageRevision, 10),
		"session_epoch=" + strconv.FormatInt(snapshot.Vector.SessionEpoch, 10),
		"session_revision=" + strconv.FormatInt(snapshot.Vector.SessionRevision, 10)}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	return Publication{ID: hex.EncodeToString(sum[:]), GeneratedAt: now.UTC().Format(time.RFC3339Nano), Snapshot: snapshot}
}

// View is what a consumer shows for one selection: the publication it may
// render, whether that publication is an older complete result kept after a
// failed rebuild, and the safe reason when it is stale or unavailable.
type View struct {
	Available   bool
	Stale       bool
	Reason      string
	Publication *Publication
}

// Publish decides which complete result a selection shows after one build.
//
// observed is the current serving vector read at the publication boundary:
// the only trusted identity of which epochs are live. A candidate belongs to
// the live state only when its epochs equal observed's; a candidate read in
// any other epoch is a late result of a retired identity (for example a
// request that finished before a restore or rebuild) and never replaces the
// current publication, however its revisions compare. Epochs are identities,
// not ordered counters, and revisions of different epochs are never compared.
//
// Within the live epochs a candidate replaces the previous publication unless
// both were read in the same epochs and the candidate's vector is older in
// either revision. A previous publication of a retired epoch is replaced by a
// live candidate and is never kept as a fallback. Without observed there is
// no trusted identity and no candidate is shown as fresh: a complete previous
// publication of the candidate's epoch stays visible as stale, and with no
// previous, or one of another epoch, the view is unavailable.
//
// A failed build keeps the previous complete publication as stale only while
// its epochs are live; with nothing to keep the view is unavailable. A
// previous publication for another selection is never used, and a candidate
// for another selection is not a result for this one. The returned
// publication is the one to retain.
func Publish(selection Selection, previous, candidate *Publication, buildErr error, observed *Vector) (View, *Publication) {
	if previous != nil && previous.Snapshot.Selection.key() != selection.key() {
		previous = nil
	}
	if previous != nil && observed != nil && !previous.Snapshot.Vector.sameEpochs(*observed) {
		previous = nil
	}
	if candidate != nil && candidate.Snapshot.Selection.key() != selection.key() {
		candidate = nil
		if buildErr == nil {
			buildErr = errMismatchedCandidate
		}
	}
	if buildErr == nil && candidate != nil {
		vector := candidate.Snapshot.Vector
		switch {
		case observed != nil && !vector.sameEpochs(*observed):
			// A retired identity's late result: keep the live publication.
			if previous != nil {
				return View{Available: true, Publication: previous}, previous
			}
			return View{Reason: PublicationReasonGenerationUnavailable}, nil
		case observed == nil:
			// No trusted live identity: a successful build only proves the
			// epoch it read, not that this epoch survived. It is never shown
			// as fresh. A complete previous of the same epoch stays visible
			// as stale, pending a verified identity; any other case is
			// unavailable.
			if previous != nil && previous.Snapshot.Vector.sameEpochs(vector) {
				return View{Available: true, Stale: true, Reason: PublicationReasonGenerationUnavailable, Publication: previous}, previous
			}
			return View{Reason: PublicationReasonGenerationUnavailable}, previous
		case previous != nil && previous.Snapshot.Vector.sameEpochs(vector) && !notOlder(vector, previous.Snapshot.Vector):
			return View{Available: true, Publication: previous}, previous
		}
		return View{Available: true, Publication: candidate}, candidate
	}
	reason := publicationReason(buildErr)
	if previous != nil {
		return View{Available: true, Stale: true, Reason: reason, Publication: previous}, previous
	}
	return View{Reason: reason}, nil
}

var errMismatchedCandidate = errors.New("candidate answers another selection")

func notOlder(candidate, previous Vector) bool {
	return candidate.UsageRevision >= previous.UsageRevision && candidate.SessionRevision >= previous.SessionRevision
}

func publicationReason(err error) string {
	switch {
	case errors.Is(err, ErrSessionsUnavailable):
		return PublicationReasonSessionsUnavailable
	case errors.Is(err, ErrClassificationUnavailable):
		return PublicationReasonClassificationUnavailable
	case errors.Is(err, ErrGenerationUnavailable):
		return PublicationReasonGenerationUnavailable
	}
	return PublicationReasonStale
}
