package main

import "testing"

// The documented windows and sigmas are part of the reviewed profile; the
// generator must reproduce them exactly, not approximately.
func TestPlanSizesMatchesReviewedProfile(t *testing.T) {
	for _, tc := range []struct {
		profile            string
		sigmaLo, sigmaHi   float64
		todayFirst, todayN int
		todayBytes         int64
		todayMin, todayMax int64
	}{
		{"small", 2.2271, 2.2273, 287, 12, 790_846, 0, 0},
	} {
		p := profiles[tc.profile]
		plan, err := planSizes(p)
		if err != nil {
			t.Fatal(err)
		}
		if plan.sigma < tc.sigmaLo || plan.sigma > tc.sigmaHi {
			t.Fatalf("%s sigma = %v", tc.profile, plan.sigma)
		}
		if total := sum(plan.rest) + sum(p.fixed); total != p.totalBytes {
			t.Fatalf("%s total = %d, want %d", tc.profile, total, p.totalBytes)
		}
		if got := plan.todayStart + 1; got != tc.todayFirst {
			t.Fatalf("%s today window starts at rank %d, want %d", tc.profile, got, tc.todayFirst)
		}
		if plan.todayBytes != tc.todayBytes {
			t.Fatalf("%s today bytes = %d, want %d", tc.profile, plan.todayBytes, tc.todayBytes)
		}
		if tc.todayMin != 0 {
			window := plan.rest[plan.todayStart : plan.todayStart+tc.todayN]
			if window[0] != tc.todayMin || window[len(window)-1] != tc.todayMax {
				t.Fatalf("%s window range = [%d, %d]", tc.profile, window[0], window[len(window)-1])
			}
		}
	}
}

// Step 3 relies on the size sum being non-decreasing in sigma over the search
// interval; otherwise bisection would not find the largest feasible sigma.
func TestSizeSumIsMonotoneInSigma(t *testing.T) {
	for name, p := range profiles {
		n := p.files - len(p.fixed)
		previous := int64(-1)
		for step := 0; step <= 390; step++ {
			sigma := 0.1 + float64(step)*0.01
			total := sum(quantileSizes(p, n, sigma))
			if total < previous {
				t.Fatalf("%s: size sum decreases at sigma %.2f", name, sigma)
			}
			previous = total
		}
	}
}
