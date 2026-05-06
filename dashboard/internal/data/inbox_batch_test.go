package data

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santifer/career-ops/dashboard/internal/model"
)

func TestQueueBatchEntries_MovesAndWritesInput(t *testing.T) {
	root := writeTempPipeline(t, `## Pendientes

- [ ] https://a.example/1 | Acme | Backend Engineer | Remote
- [ ] https://b.example/2 | Beta | Senior SWE | NYC
- [ ] https://c.example/3 | Gamma | FDE | SF

## Procesadas
`)

	entries := []model.InboxEntry{
		{URL: "https://a.example/1", Company: "Acme", Title: "Backend Engineer", Location: "Remote"},
		{URL: "https://c.example/3", Company: "Gamma", Title: "FDE", Location: "SF"},
	}

	queued, inputPath, err := QueueBatchEntries(root, entries)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if queued != 2 {
		t.Errorf("queued = %d, want 2", queued)
	}

	pipelineText, _ := os.ReadFile(filepath.Join(root, "data", "pipeline.md"))
	pipeline := string(pipelineText)
	pendingSection := getSection(pipeline, "## Pendientes")
	if strings.Contains(pendingSection, "https://a.example/1") {
		t.Errorf("a.example still in Pendientes:\n%s", pendingSection)
	}
	if strings.Contains(pendingSection, "https://c.example/3") {
		t.Errorf("c.example still in Pendientes:\n%s", pendingSection)
	}
	if !strings.Contains(pendingSection, "https://b.example/2") {
		t.Errorf("b.example incorrectly removed from Pendientes:\n%s", pendingSection)
	}

	if !strings.Contains(pipeline, "### Batch queued") {
		t.Errorf("Batch queued subsection missing")
	}
	queuedSection := getSection(pipeline, "## Filtered Out")
	if !strings.Contains(queuedSection, "https://a.example/1") {
		t.Errorf("a.example missing from Batch queued")
	}
	if !strings.Contains(queuedSection, "https://c.example/3") {
		t.Errorf("c.example missing from Batch queued")
	}

	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read batch-input: %v", err)
	}
	input := string(inputBytes)
	if !strings.Contains(input, "id\turl\tsource\tnotes") {
		t.Errorf("batch-input missing header:\n%s", input)
	}
	if !strings.Contains(input, "https://a.example/1") || !strings.Contains(input, "https://c.example/3") {
		t.Errorf("batch-input missing entries:\n%s", input)
	}
	if !strings.HasPrefix(input, "id\turl\tsource\tnotes\n1\thttps://a.example/1\tinbox\t") {
		t.Errorf("batch-input first row malformed:\n%s", input)
	}
}

func TestQueueBatchEntries_EmptyReturnsError(t *testing.T) {
	root := writeTempPipeline(t, "## Pendientes\n\n- [ ] https://a.example/1 | Acme | SWE | NYC\n")
	_, _, err := QueueBatchEntries(root, nil)
	if !errors.Is(err, ErrNoEntriesToQueue) {
		t.Errorf("want ErrNoEntriesToQueue, got %v", err)
	}
}

func TestQueueBatchEntries_SkipsAlreadyMissingURL(t *testing.T) {
	root := writeTempPipeline(t, "## Pendientes\n\n- [ ] https://a.example/1 | Acme | SWE | NYC\n")
	entries := []model.InboxEntry{
		{URL: "https://a.example/1", Company: "Acme", Title: "SWE"},
		{URL: "https://gone.example/9", Company: "Ghost", Title: "X"},
	}
	queued, _, err := QueueBatchEntries(root, entries)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if queued != 1 {
		t.Errorf("queued = %d, want 1 (the missing URL should be skipped)", queued)
	}
}

func TestSelectedURLs_RoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := SaveSelectedURLs(root, []string{"https://b.example/2", "https://a.example/1"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := LoadSelectedURLs(root)
	if !got["https://a.example/1"] || !got["https://b.example/2"] {
		t.Errorf("loaded set missing entries: %v", got)
	}
	if len(got) != 2 {
		t.Errorf("loaded %d entries, want 2", len(got))
	}

	// Saving an empty list removes the file.
	if err := SaveSelectedURLs(root, nil); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	if _, err := os.Stat(SelectedURLsPath(root)); !os.IsNotExist(err) {
		t.Errorf("file should be removed; stat err = %v", err)
	}
}

func TestPruneSelectedURLs_DropsStale(t *testing.T) {
	root := t.TempDir()
	if err := SaveSelectedURLs(root, []string{"https://a.example/1", "https://stale.example/9"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	valid := map[string]bool{"https://a.example/1": true}
	kept, err := PruneSelectedURLs(root, valid)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(kept) != 1 || !kept["https://a.example/1"] {
		t.Errorf("kept = %v, want only a.example/1", kept)
	}
	disk := LoadSelectedURLs(root)
	if disk["https://stale.example/9"] {
		t.Errorf("stale URL not removed from disk: %v", disk)
	}
}
