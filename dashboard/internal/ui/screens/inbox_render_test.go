package screens

import (
	"fmt"
	"strings"
	"testing"

	"github.com/santifer/career-ops/dashboard/internal/data"
	"github.com/santifer/career-ops/dashboard/internal/theme"
)

// TestInboxRender renders the inbox once at a fixed terminal size and prints
// the output stripped of ANSI codes. Lets us eyeball the layout without
// running the TUI interactively. Run with:
//
//	go test ./internal/ui/screens -run TestInboxRender -v
func TestInboxRender(t *testing.T) {
	entries := data.ParseInbox("../../../..")
	if len(entries) == 0 {
		t.Skip("no pipeline.md entries to render against")
	}

	th := theme.NewTheme("catppuccin-latte")
	m := NewInboxModel(th, entries, 140, 30)

	out := stripANSI(m.View())
	fmt.Println("\n--- ALL tab (default sort: fit) ---")
	fmt.Println(out)

	// Switch to TOP FIT tab
	m.activeTab = 1
	m.applyFilterAndSort()
	out = stripANSI(m.View())
	fmt.Println("\n--- TOP FIT tab ---")
	fmt.Println(out)
}

func stripANSI(s string) string {
	// Tiny stripper: remove ESC [ ... m sequences. Good enough for tests.
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			i = j + 1
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
