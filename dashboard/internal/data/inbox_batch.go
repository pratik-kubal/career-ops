package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santifer/career-ops/dashboard/internal/model"
)

const (
	subsectionBatchQueued = "### Batch queued"
	batchInputFile        = "batch-input.tsv"
)

// ErrNoEntriesToQueue is returned when QueueBatchEntries is given an empty list.
var ErrNoEntriesToQueue = errors.New("no entries to queue")

// QueueBatchEntries does three things atomically per call:
//
//  1. Removes each entry's URL from the "## Pendientes" section of pipeline.md
//     and appends it to a "### Batch queued" subsection under "## Filtered Out".
//     Entries already missing from Pendientes are silently skipped (idempotent).
//
//  2. Writes the entries to batch/batch-input.tsv with the schema the existing
//     batch-runner.sh expects: id\turl\tsource\tnotes (header row included).
//     Overwrites any existing file.
//
//  3. Returns (queued count, batch-input path) on success.
//
// The caller is responsible for clearing any persisted selection state.
func QueueBatchEntries(careerOpsPath string, entries []model.InboxEntry) (int, string, error) {
	if len(entries) == 0 {
		return 0, "", ErrNoEntriesToQueue
	}

	path, lines, err := loadPipelineLines(careerOpsPath)
	if err != nil {
		return 0, "", err
	}

	queued := 0
	for _, entry := range entries {
		pStart, pEnd := findSectionRange(lines, sectionPendientes)
		if pStart < 0 {
			return queued, "", fmt.Errorf("section %q not found in pipeline.md", sectionPendientes)
		}
		idx := findPendingLine(lines, pStart+1, pEnd, entry.URL)
		if idx < 0 {
			// Entry no longer in Pendientes — skip (could already be queued/processed).
			continue
		}
		line := lines[idx]
		lines = append(lines[:idx], lines[idx+1:]...)
		lines = appendToBatchQueued(lines, line)
		queued++
	}

	if err := writePipeline(path, lines); err != nil {
		return queued, "", err
	}

	inputPath, err := writeBatchInputTSV(careerOpsPath, entries)
	if err != nil {
		return queued, "", err
	}
	return queued, inputPath, nil
}

// BatchHasExistingState reports whether batch/batch-state.tsv exists. Helps the
// dashboard warn the user that a prior batch run is still tracked.
func BatchHasExistingState(careerOpsPath string) bool {
	p := filepath.Join(careerOpsPath, "batch", "batch-state.tsv")
	_, err := os.Stat(p)
	return err == nil
}

func appendToBatchQueued(lines []string, entryLine string) []string {
	if sStart, sEnd := findSubsectionRange(lines, sectionFilteredOut, subsectionBatchQueued); sStart >= 0 {
		insertAt := sEnd
		for insertAt > sStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
			insertAt--
		}
		return insertLines(lines, insertAt, entryLine)
	}
	if pStart, pEnd := findSectionRange(lines, sectionFilteredOut); pStart >= 0 {
		insertAt := pEnd
		for insertAt > pStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
			insertAt--
		}
		return insertLines(lines, insertAt, "", subsectionBatchQueued, "", entryLine, "")
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	return append(lines, sectionFilteredOut, "", subsectionBatchQueued, "", entryLine)
}

func writeBatchInputTSV(careerOpsPath string, entries []model.InboxEntry) (string, error) {
	p := filepath.Join(careerOpsPath, "batch", batchInputFile)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("id\turl\tsource\tnotes\n")
	for i, e := range entries {
		notes := strings.TrimSpace(e.Company + " — " + e.Title)
		notes = strings.ReplaceAll(notes, "\t", " ")
		b.WriteString(fmt.Sprintf("%d\t%s\tinbox\t%s\n", i+1, e.URL, notes))
	}
	if err := os.WriteFile(p, []byte(b.String()), 0644); err != nil {
		return "", err
	}
	return p, nil
}
