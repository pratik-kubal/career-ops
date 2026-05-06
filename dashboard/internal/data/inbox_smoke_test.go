package data

import (
	"fmt"
	"testing"
)

// TestInboxSmoke is an exploratory test that parses the live pipeline.md and
// prints tier counts + a sample. Not asserting numbers — purely a smoke check
// that ParseInbox/ScoreInboxEntry work on real data. Run with:
//
//	go test ./internal/data -run TestInboxSmoke -v
func TestInboxSmoke(t *testing.T) {
	entries := ParseInbox("../../..")
	if len(entries) == 0 {
		t.Skip("no pipeline.md entries to test against")
	}

	counts := CountInboxByTier(entries)
	fmt.Printf("Total: %d  |  ★ %d   ★★ %d   ★★★ %d\n",
		len(entries), counts[0], counts[1], counts[2])

	tier3 := 0
	for _, e := range entries {
		if e.FitTier == 3 && tier3 < 10 {
			fmt.Printf("  ★★★  %-20s  %-50s  %s\n",
				truncateForTest(e.Company, 20),
				truncateForTest(e.Title, 50),
				truncateForTest(e.Location, 40))
			tier3++
		}
	}

	tier1 := 0
	fmt.Println("\nSample tier 1 (★) — should be off-archetype titles:")
	for _, e := range entries {
		if e.FitTier == 1 && tier1 < 5 {
			fmt.Printf("  ★   %-20s  %-50s  %s\n",
				truncateForTest(e.Company, 20),
				truncateForTest(e.Title, 50),
				truncateForTest(e.Location, 40))
			tier1++
		}
	}

	tier2 := 0
	fmt.Println("\nSample tier 2 (★★) — should be FDE/Solutions/secondary engineering:")
	for _, e := range entries {
		if e.FitTier == 2 && tier2 < 8 {
			fmt.Printf("  ★★  %-20s  %-50s  %s\n",
				truncateForTest(e.Company, 20),
				truncateForTest(e.Title, 50),
				truncateForTest(e.Location, 40))
			tier2++
		}
	}
}

func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
