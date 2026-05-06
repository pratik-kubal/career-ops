package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInboxEntryNotPending means the URL is not in the "## Pendientes" section.
	ErrInboxEntryNotPending = errors.New("inbox entry not in pending")
	// ErrInboxEntryNotRemoved means the URL is not in the manually-removed subsection.
	ErrInboxEntryNotRemoved = errors.New("inbox entry not in manually-removed")
)

const (
	sectionPendientes    = "## Pendientes"
	sectionFilteredOut   = "## Filtered Out"
	subsectionManuallyRm = "### Manually removed"
)

// DeleteInboxEntry moves a pending entry from "## Pendientes" into a
// "### Manually removed" subsection under "## Filtered Out". If either header
// is missing it is created. Returns ErrInboxEntryNotPending if the URL is not
// currently in pending.
func DeleteInboxEntry(careerOpsPath, url string) error {
	path, lines, err := loadPipelineLines(careerOpsPath)
	if err != nil {
		return err
	}

	pStart, pEnd := findSectionRange(lines, sectionPendientes)
	if pStart < 0 {
		return fmt.Errorf("%w: %q section not found", ErrInboxEntryNotPending, sectionPendientes)
	}
	matchIdx := findPendingLine(lines, pStart+1, pEnd, url)
	if matchIdx < 0 {
		return fmt.Errorf("%w: %s", ErrInboxEntryNotPending, url)
	}

	entryLine := lines[matchIdx]
	lines = append(lines[:matchIdx], lines[matchIdx+1:]...)
	lines = appendToManuallyRemoved(lines, entryLine)

	return writePipeline(path, lines)
}

// RestoreInboxEntry moves an entry from "### Manually removed" back to the
// top of "## Pendientes". Returns ErrInboxEntryNotRemoved if not present.
func RestoreInboxEntry(careerOpsPath, url string) error {
	path, lines, err := loadPipelineLines(careerOpsPath)
	if err != nil {
		return err
	}

	rStart, rEnd := findSubsectionRange(lines, sectionFilteredOut, subsectionManuallyRm)
	if rStart < 0 {
		return fmt.Errorf("%w: subsection %q not present", ErrInboxEntryNotRemoved, subsectionManuallyRm)
	}
	matchIdx := findPendingLine(lines, rStart+1, rEnd, url)
	if matchIdx < 0 {
		return fmt.Errorf("%w: %s", ErrInboxEntryNotRemoved, url)
	}

	entryLine := lines[matchIdx]
	lines = append(lines[:matchIdx], lines[matchIdx+1:]...)
	lines = insertAtTopOfPending(lines, entryLine)

	return writePipeline(path, lines)
}

// loadPipelineLines reads pipeline.md and returns its path and split lines.
// Tries {careerOpsPath}/data/pipeline.md first, then {careerOpsPath}/pipeline.md.
func loadPipelineLines(careerOpsPath string) (string, []string, error) {
	path := filepath.Join(careerOpsPath, "data", "pipeline.md")
	content, err := os.ReadFile(path)
	if err != nil {
		path = filepath.Join(careerOpsPath, "pipeline.md")
		content, err = os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
	}
	return path, strings.Split(string(content), "\n"), nil
}

func writePipeline(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// findSectionRange locates a level-2 ("## ") section by exact header match.
// Returns [start, end) where start is the header line and end is the next
// level-2 header (or len(lines) if none). Returns -1, -1 if not found.
func findSectionRange(lines []string, header string) (int, int) {
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return start, end
}

// findSubsectionRange locates a level-3 ("### ") subsection inside a level-2
// section. Returns [start, end) where end stops at the next "### " or "## ".
func findSubsectionRange(lines []string, parentHeader, subHeader string) (int, int) {
	pStart, pEnd := findSectionRange(lines, parentHeader)
	if pStart < 0 {
		return -1, -1
	}
	subStart := -1
	for i := pStart + 1; i < pEnd; i++ {
		if strings.TrimSpace(lines[i]) == subHeader {
			subStart = i
			break
		}
	}
	if subStart < 0 {
		return -1, -1
	}
	subEnd := pEnd
	for i := subStart + 1; i < pEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ") {
			subEnd = i
			break
		}
	}
	return subStart, subEnd
}

// findPendingLine returns the index in lines[start:end) whose URL field
// equals `url`. Returns -1 if not found.
func findPendingLine(lines []string, start, end int, url string) int {
	for i := start; i < end; i++ {
		if pendingLineMatchesURL(lines[i], url) {
			return i
		}
	}
	return -1
}

func pendingLineMatchesURL(line, url string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- [ ]") {
		return false
	}
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "- [ ]"))
	fields := strings.SplitN(body, "|", 2)
	if len(fields) == 0 {
		return false
	}
	return strings.TrimSpace(fields[0]) == url
}

// insertLines returns a new slice with `newLines` inserted at index `at`.
func insertLines(lines []string, at int, newLines ...string) []string {
	out := make([]string, 0, len(lines)+len(newLines))
	out = append(out, lines[:at]...)
	out = append(out, newLines...)
	out = append(out, lines[at:]...)
	return out
}

// appendToManuallyRemoved inserts entryLine into the manually-removed
// subsection, creating the subsection (and parent section) if absent.
func appendToManuallyRemoved(lines []string, entryLine string) []string {
	if sStart, sEnd := findSubsectionRange(lines, sectionFilteredOut, subsectionManuallyRm); sStart >= 0 {
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
		return insertLines(lines, insertAt, "", subsectionManuallyRm, "", entryLine, "")
	}

	// Neither parent nor subsection exists — append both at end of file.
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	return append(lines, sectionFilteredOut, "", subsectionManuallyRm, "", entryLine)
}

// insertAtTopOfPending inserts entryLine at the top of "## Pendientes"
// (immediately after the header and any blank lines). Creates the section
// at the start of the file if missing.
func insertAtTopOfPending(lines []string, entryLine string) []string {
	pStart, _ := findSectionRange(lines, sectionPendientes)
	if pStart < 0 {
		block := []string{sectionPendientes, "", entryLine, ""}
		return append(block, lines...)
	}
	insertAt := pStart + 1
	for insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) == "" {
		insertAt++
	}
	return insertLines(lines, insertAt, entryLine)
}
