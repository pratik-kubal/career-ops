package screens

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/santifer/career-ops/dashboard/internal/data"
	"github.com/santifer/career-ops/dashboard/internal/model"
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
	m := NewInboxModel(th, entries, nil, 140, 30)

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

// TestInboxDeleteEmitsMsg verifies that pressing 'd' on a non-empty inbox emits
// an InboxDeleteEntryMsg for the current entry, pushes onto the undo stack, and
// sets feedback. 'u' then emits InboxRestoreEntryMsg for the same entry and
// pops the stack.
func TestInboxDeleteEmitsMsg(t *testing.T) {
	entries := []model.InboxEntry{
		{URL: "https://a.example/1", Company: "Acme", Title: "SWE", Location: "NYC", FitTier: 3, FitLabel: "★★★"},
		{URL: "https://b.example/2", Company: "Beta", Title: "SWE", Location: "Remote", FitTier: 2, FitLabel: "★★"},
	}
	m := NewInboxModel(theme.NewTheme("catppuccin-mocha"), entries, nil, 120, 30)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if cmd == nil {
		t.Fatal("expected delete cmd, got nil")
	}
	delMsg, ok := cmd().(InboxDeleteEntryMsg)
	if !ok {
		t.Fatalf("expected InboxDeleteEntryMsg, got %T", cmd())
	}
	if delMsg.Entry.URL != "https://a.example/1" {
		t.Errorf("delete URL = %q, want a.example/1", delMsg.Entry.URL)
	}
	if len(m.recentlyDeleted) != 1 {
		t.Errorf("undo stack size = %d, want 1", len(m.recentlyDeleted))
	}
	if !strings.Contains(m.feedback, "Deleted") {
		t.Errorf("feedback = %q, want contains 'Deleted'", m.feedback)
	}

	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd == nil {
		t.Fatal("expected undo cmd, got nil")
	}
	restoreMsg, ok := cmd().(InboxRestoreEntryMsg)
	if !ok {
		t.Fatalf("expected InboxRestoreEntryMsg, got %T", cmd())
	}
	if restoreMsg.Entry.URL != "https://a.example/1" {
		t.Errorf("restore URL = %q, want a.example/1", restoreMsg.Entry.URL)
	}
	if len(m.recentlyDeleted) != 0 {
		t.Errorf("undo stack size after undo = %d, want 0", len(m.recentlyDeleted))
	}
	if !strings.Contains(m.feedback, "Restored") {
		t.Errorf("feedback = %q, want contains 'Restored'", m.feedback)
	}
}

// TestInboxSelectAndBatch covers the full select → batch flow:
//   - space toggles selection and emits InboxSelectionChangedMsg
//   - 'b' once arms (no message), 'b' twice emits InboxApplyBatchMsg
//   - 'A' clears all selections
func TestInboxSelectAndBatch(t *testing.T) {
	entries := []model.InboxEntry{
		{URL: "https://a.example/1", Company: "Acme", Title: "SWE", Location: "NYC", FitTier: 3, FitLabel: "★★★"},
		{URL: "https://b.example/2", Company: "Beta", Title: "FDE", Location: "Remote", FitTier: 2, FitLabel: "★★"},
	}
	m := NewInboxModel(theme.NewTheme("catppuccin-mocha"), entries, nil, 120, 30)

	// Select entry 1 with space.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	if cmd == nil {
		t.Fatal("expected selection cmd")
	}
	selMsg, ok := cmd().(InboxSelectionChangedMsg)
	if !ok {
		t.Fatalf("expected InboxSelectionChangedMsg, got %T", cmd())
	}
	if len(selMsg.URLs) != 1 || selMsg.URLs[0] != "https://a.example/1" {
		t.Errorf("selection URLs = %v, want [a.example/1]", selMsg.URLs)
	}
	if !m.selected["https://a.example/1"] {
		t.Error("entry 1 not in selected set")
	}

	// First 'b' arms; no command emitted.
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if cmd != nil {
		t.Errorf("expected nil cmd on first 'b' (arming), got %T", cmd())
	}
	if !m.applyArmed {
		t.Error("applyArmed not set after first 'b'")
	}

	// Second 'b' confirms; emits InboxApplyBatchMsg with selected entries.
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if cmd == nil {
		t.Fatal("expected apply cmd on second 'b'")
	}
	applyMsg, ok := cmd().(InboxApplyBatchMsg)
	if !ok {
		t.Fatalf("expected InboxApplyBatchMsg, got %T", cmd())
	}
	if len(applyMsg.Entries) != 1 || applyMsg.Entries[0].URL != "https://a.example/1" {
		t.Errorf("apply entries = %v, want one a.example/1", applyMsg.Entries)
	}
	if m.applyArmed {
		t.Error("applyArmed should be reset after confirm")
	}
	if len(m.selected) != 0 {
		t.Errorf("selection should be cleared after apply, got %d", len(m.selected))
	}
}

func TestInboxArmedCancelsOnOtherKey(t *testing.T) {
	entries := []model.InboxEntry{
		{URL: "https://a.example/1", Company: "Acme", Title: "SWE", FitTier: 3, FitLabel: "★★★"},
	}
	m := NewInboxModel(theme.NewTheme("catppuccin-mocha"), entries, map[string]bool{"https://a.example/1": true}, 120, 30)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if !m.applyArmed {
		t.Fatal("applyArmed should be set")
	}
	// Pressing 'j' (down) should cancel the armed state.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.applyArmed {
		t.Error("applyArmed should be cancelled on non-'b' key")
	}
	if !strings.Contains(m.feedback, "cancelled") {
		t.Errorf("feedback = %q, want contains 'cancelled'", m.feedback)
	}
}

// TestInboxUndoEmpty verifies that pressing 'u' with an empty undo stack
// sets a 'Nothing to undo.' feedback and emits no command.
func TestInboxUndoEmpty(t *testing.T) {
	entries := []model.InboxEntry{
		{URL: "https://a.example/1", Company: "Acme", Title: "SWE", Location: "NYC", FitTier: 3, FitLabel: "★★★"},
	}
	m := NewInboxModel(theme.NewTheme("catppuccin-mocha"), entries, nil, 120, 30)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if cmd != nil {
		t.Errorf("expected nil cmd for empty undo, got %T", cmd())
	}
	if m.feedback != "Nothing to undo." {
		t.Errorf("feedback = %q, want 'Nothing to undo.'", m.feedback)
	}
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
