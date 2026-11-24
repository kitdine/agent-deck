package cjsiledger

import (
	"cmp"
	"slices"
	"time"
)

type witnessedPoint struct {
	counts Counts
	at     time.Time
	ref    Reference
}

// Subdivide only at observed cumulative endpoints with ordered source times.
// Split portions cannot supply an invocation identity. An independently
// witnessed original call for exactly a portion can still supply its own.
func splitLegacyRanges(rows []observedMeter) []observedMeter {
	endpoints := map[[32]byte][]witnessedPoint{}
	for _, r := range rows {
		if r.bounded && !r.start.IsZero() && !r.At.Before(r.start) {
			endpoints[r.epoch] = append(endpoints[r.epoch], witnessedPoint{r.lo, r.start, r.begin}, witnessedPoint{r.hi, r.At, r.Proof[0]})
		}
	}
	for k, pts := range endpoints {
		slices.SortStableFunc(pts, func(a, b witnessedPoint) int {
			return cmp.Or(cmp.Compare(a.counts.Input, b.counts.Input), cmp.Compare(a.counts.Output, b.counts.Output), a.at.Compare(b.at),
				// Keep identical optional evidence adjacent even when several copies
				// witness the same primary point at the same time.
				cmp.Compare(a.counts.Known, b.counts.Known), cmp.Compare(a.counts.Cached, b.counts.Cached),
				cmp.Compare(a.counts.Write, b.counts.Write), cmp.Compare(a.counts.Reasoning, b.counts.Reasoning))
		})
		endpoints[k] = slices.CompactFunc(pts, func(a, b witnessedPoint) bool { return a.counts == b.counts && a.at.Equal(b.at) })
	}
	var out []observedMeter
	for _, r := range rows {
		if !r.bounded {
			out = append(out, r)
			continue
		}
		points := []witnessedPoint{{r.lo, r.start, r.begin}}
		ok := true
		pointsInEpoch := endpoints[r.epoch]
		i, _ := slices.BinarySearchFunc(pointsInEpoch, r.lo, func(p witnessedPoint, q Counts) int {
			return cmp.Or(cmp.Compare(p.counts.Input, q.Input), cmp.Compare(p.counts.Output, q.Output))
		})
		for _, p := range pointsInEpoch[i:] {
			if p.counts.Input > r.hi.Input {
				break
			}
			if samePoint(r.lo, p.counts) || samePoint(r.hi, p.counts) || !before(r.lo, p.counts) || !before(p.counts, r.hi) {
				continue
			}
			prev := points[len(points)-1]
			if p.at.Before(prev.at) || r.At.Before(p.at) || !before(prev.counts, p.counts) || samePoint(prev.counts, p.counts) {
				ok = false
				break
			}
			points = append(points, p)
		}
		if !ok || len(points) == 1 {
			out = append(out, r)
			continue
		}
		points = append(points, witnessedPoint{r.hi, r.At, r.Proof[0]})
		for j := 1; j < len(points); j++ {
			lo, hi := points[j-1], points[j]
			piece := r
			piece.aggregateOnly = true
			piece.Key.ID = ""
			piece.lo, piece.hi, piece.start, piece.At = lo.counts, hi.counts, lo.at, hi.at
			piece.Tokens, _ = difference(hi.counts, lo.counts)
			piece.begin, piece.Proof = lo.ref, []Reference{hi.ref}
			out = append(out, piece)
		}
	}
	return out
}
