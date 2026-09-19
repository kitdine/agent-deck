package quota

// Allowed is architecture.md C1's gate: a client is probed only when quota
// reading is on and its current provider is official, in that order —
// probeEnabled off suppresses every probe regardless of provider, and the
// provider check runs only once probeEnabled has already passed.
//
// recordedOfficial is whether provider.Service.Current's recorded selection
// for this client is the official provider; it is false both when the
// current selection is some other provider and when there has never been a
// completed selection at all, since neither establishes official use, and
// both carry the same reason.
//
// observedKnown and observedOfficial carry C1's best-effort cross-check
// against a Hook-delivered *observed* provider (distinct from AgentDeck's
// own *recorded* selection — see architecture.md C1, "a recorded selection
// is not an observed one"). observedKnown is false whenever no Hook delivery
// has ever reported an observed provider for this client — the ordinary case
// for a client with no Hook integration — and in that case it must never by
// itself suppress a probe. When observedKnown is true, a disagreeing
// observed provider suppresses the probe even though the recorded selection
// says official: suppressing on disagreement is the safe direction, since it
// never probes an account the product is not sure it should.
//
// This function takes plain booleans rather than provider.Service or
// usage.Service directly so it stays a pure, dependency-free predicate; the
// caller (the scheduler, or its own caller) resolves those services and
// reduces their results to these four booleans.
func Allowed(probeEnabled, recordedOfficial, observedKnown, observedOfficial bool) (bool, Reason) {
	if !probeEnabled {
		return false, ReasonProbeDisabled
	}
	if !recordedOfficial {
		return false, ReasonNotOfficial
	}
	if observedKnown && !observedOfficial {
		return false, ReasonNotOfficial
	}
	return true, ""
}
