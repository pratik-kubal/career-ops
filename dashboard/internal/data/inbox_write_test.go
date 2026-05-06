package data

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempPipeline(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dataDir, "pipeline.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

func readPipeline(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "data", "pipeline.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func TestDeleteInboxEntry_MovesFromPendingToManuallyRemoved(t *testing.T) {
	root := writeTempPipeline(t, `# Pipeline

## Pendientes

- [ ] https://a.example/job/1 | Acme | Engineer | NYC
- [ ] https://b.example/job/2 | Beta | Engineer | Remote

## Procesadas

- [x] #001 | https://c.example/job/3 | Gamma | PM | 4.0/5 | PDF ✅
`)

	if err := DeleteInboxEntry(root, "https://a.example/job/1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got := readPipeline(t, root)
	if strings.Contains(getSection(got, "## Pendientes"), "https://a.example/job/1") {
		t.Errorf("entry still present in Pendientes:\n%s", got)
	}
	if !strings.Contains(got, "## Filtered Out") {
		t.Errorf("Filtered Out section not created:\n%s", got)
	}
	if !strings.Contains(got, "### Manually removed") {
		t.Errorf("Manually removed subsection not created:\n%s", got)
	}
	if !strings.Contains(getSection(got, "## Filtered Out"), "https://a.example/job/1") {
		t.Errorf("entry not moved to Filtered Out:\n%s", got)
	}
	// The other pending entry must remain.
	if !strings.Contains(getSection(got, "## Pendientes"), "https://b.example/job/2") {
		t.Errorf("untouched pending entry was lost:\n%s", got)
	}
	// Procesadas must be untouched.
	if !strings.Contains(getSection(got, "## Procesadas"), "https://c.example/job/3") {
		t.Errorf("Procesadas section was disturbed:\n%s", got)
	}
}

func TestDeleteInboxEntry_AppendsToExistingFilteredOut(t *testing.T) {
	root := writeTempPipeline(t, `## Pendientes

- [ ] https://a.example/job/1 | Acme | Engineer | NYC

## Filtered Out

### Sales (1)

- [ ] https://x.example/job/9 | XYZ | SE | NYC

## Procesadas
`)

	if err := DeleteInboxEntry(root, "https://a.example/job/1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got := readPipeline(t, root)
	if !strings.Contains(got, "### Manually removed") {
		t.Errorf("subsection not created under existing Filtered Out:\n%s", got)
	}
	// Existing subsection must remain.
	if !strings.Contains(got, "### Sales (1)") {
		t.Errorf("existing subsection lost:\n%s", got)
	}
	if !strings.Contains(getSection(got, "## Filtered Out"), "https://x.example/job/9") {
		t.Errorf("existing entry in Filtered Out lost:\n%s", got)
	}
}

func TestDeleteInboxEntry_NotPending(t *testing.T) {
	root := writeTempPipeline(t, `## Pendientes

- [ ] https://a.example/job/1 | Acme | Engineer | NYC
`)

	err := DeleteInboxEntry(root, "https://nope.example/x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInboxEntryNotPending) {
		t.Errorf("want ErrInboxEntryNotPending, got %v", err)
	}
}

func TestRestoreInboxEntry_RoundTrip(t *testing.T) {
	root := writeTempPipeline(t, `## Pendientes

- [ ] https://a.example/job/1 | Acme | Engineer | NYC
- [ ] https://b.example/job/2 | Beta | Engineer | Remote

## Procesadas
`)

	if err := DeleteInboxEntry(root, "https://b.example/job/2"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := RestoreInboxEntry(root, "https://b.example/job/2"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got := readPipeline(t, root)
	pending := getSection(got, "## Pendientes")
	if !strings.Contains(pending, "https://b.example/job/2") {
		t.Errorf("entry not restored to Pendientes:\n%s", got)
	}
	// Restore goes to TOP of Pendientes — should appear before the other entry.
	idxRestored := strings.Index(pending, "https://b.example/job/2")
	idxOther := strings.Index(pending, "https://a.example/job/1")
	if idxRestored < 0 || idxOther < 0 || idxRestored > idxOther {
		t.Errorf("restored entry not at top of Pendientes:\n%s", pending)
	}
	// Manually removed should now be empty (no entries left).
	filtered := getSection(got, "## Filtered Out")
	if strings.Contains(filtered, "https://b.example/job/2") {
		t.Errorf("entry still present in Filtered Out after restore:\n%s", filtered)
	}
}

func TestRestoreInboxEntry_NotRemoved(t *testing.T) {
	root := writeTempPipeline(t, `## Pendientes

- [ ] https://a.example/job/1 | Acme | Engineer | NYC

## Filtered Out

### Manually removed

`)

	err := RestoreInboxEntry(root, "https://nope.example/x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInboxEntryNotRemoved) {
		t.Errorf("want ErrInboxEntryNotRemoved, got %v", err)
	}
}

// getSection returns the slice of `content` between `header` and the next
// "## " header (exclusive). Returns "" if header not found.
func getSection(content, header string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == header {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}
