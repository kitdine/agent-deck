package main

import (
	"errors"
	"math"
)

const mib = int64(1) << 20

// profile holds the parameters of a synthetic fixture profile. Only `small`
// remains: automated tests use it; measurement uses the frozen real corpus.
type profile struct {
	name           string
	files          int
	totalBytes     int64
	fixed          []int64
	median         float64
	lower, upper   int64
	spanDays       int
	todayFiles     int
	todayTarget    int64
	todayContinued int
}

var profiles = map[string]profile{
	"small": {
		name: "small", files: 400, totalBytes: 64 * mib,
		fixed:  []int64{8 * mib, 4 * mib},
		median: 16 << 10, lower: 512, upper: 3 * mib, spanDays: 30,
		todayFiles: 12, todayTarget: 786_432, todayContinued: 3,
	},
}

// sizePlan is the seed-independent size set of a profile.
type sizePlan struct {
	sigma float64
	// rest holds the n non-fixed sizes in ascending quantile order.
	rest []int64
	// todayStart is the first index in rest of the today-related window.
	todayStart int
	todayBytes int64
}

func quantileSizes(p profile, n int, sigma float64) []int64 {
	out := make([]int64, n)
	lnMedian := math.Log(p.median)
	for i := 1; i <= n; i++ {
		q := (float64(i) - 0.5) / float64(n)
		z := math.Sqrt2 * math.Erfinv(2*q-1)
		x := math.Exp(lnMedian + sigma*z)
		x = math.Min(math.Max(x, float64(p.lower)), float64(p.upper))
		out[i-1] = int64(math.Floor(x))
	}
	return out
}

func sum(values []int64) int64 {
	var total int64
	for _, v := range values {
		total += v
	}
	return total
}

// planSizes implements steps 1-4 of the fixture size algorithm.
func planSizes(p profile) (sizePlan, error) {
	n := p.files - len(p.fixed)
	target := p.totalBytes - sum(p.fixed)
	lo, hi := 0.1, 4.0
	for range 60 {
		mid := (lo + hi) / 2
		if sum(quantileSizes(p, n, mid)) <= target {
			lo = mid
		} else {
			hi = mid
		}
	}
	sizes := quantileSizes(p, n, lo)
	remaining := target - sum(sizes)
	if remaining < 0 {
		return sizePlan{}, errors.New("fixture: size bisection exceeded the target")
	}
	for j := n - 1; j >= 0 && remaining > 0; j-- {
		add := min(p.upper-sizes[j], remaining)
		sizes[j] += add
		remaining -= add
	}
	if remaining != 0 {
		return sizePlan{}, errors.New("fixture: remaining bytes cannot be allocated under the upper bound")
	}
	// Remainder goes to the largest files, so the order stays ascending and
	// ties keep their quantile order.
	k := p.todayFiles
	start, best := 0, int64(-1)
	window := sum(sizes[:k])
	for s := 0; s+k <= n; s++ {
		if s > 0 {
			window += sizes[s+k-1] - sizes[s-1]
		}
		diff := window - p.todayTarget
		if diff < 0 {
			diff = -diff
		}
		if best < 0 || diff < best {
			best, start = diff, s
		}
	}
	return sizePlan{sigma: lo, rest: sizes, todayStart: start, todayBytes: sum(sizes[start : start+k])}, nil
}

// medianSize is the nearest-rank median of the non-fixed sizes.
func (s sizePlan) medianSize() int64 {
	return s.rest[(len(s.rest)+1)/2-1]
}
