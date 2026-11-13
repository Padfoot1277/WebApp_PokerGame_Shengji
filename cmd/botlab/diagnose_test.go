package main

import "testing"

func TestEndgameComparisonDenominator(t *testing.T) {
	a := diagnosticCounts{Informative: 2, Optimal: [3]int{0, 2, 0}, MissedNetLevels: [3]int{2, 0, 2}, Corrected: [2]int{2, 0}, LowerRegret: [2]int{2, 0}}
	b := diagnosticCounts{Informative: 1, Optimal: [3]int{1, 0, 1}, MissedNetLevels: [3]int{0, 1, 0}, NewlyMissed: [2]int{1, 0}, HigherRegret: [2]int{1, 0}}
	total := diagnosticCounts{Informative: 3, Optimal: [3]int{1, 2, 1}, MissedNetLevels: [3]int{2, 1, 2}, Corrected: a.Corrected, NewlyMissed: b.NewlyMissed, LowerRegret: a.LowerRegret, HigherRegret: b.HigherRegret}
	results := compareEndings([]diagnosticDeal{{diagnosticCounts: a}, {diagnosticCounts: b}, {}}, total)
	c := results[0]
	if c.RegretReduction != 1./3 || c.Corrected != 2 || c.NewlyMissed != 1 || c.CandidateOptimalRate != 2./3 || c.RegretReduction95 != [2]float64{-1, 1} {
		t.Fatal(c)
	}
	if results[1].RegretReduction95 != [2]float64{} || results[1].OptimalRateChange95 != [2]float64{} {
		t.Fatal(results[1])
	}
	if compareEndings(nil, diagnosticCounts{}) != nil {
		t.Fatal("empty data should produce no rates or intervals")
	}
}
