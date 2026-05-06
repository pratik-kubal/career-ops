package data

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const inboxSelectedFile = "inbox-selected.tsv"

// SelectedURLsPath returns the path to the selection sidecar file.
func SelectedURLsPath(careerOpsPath string) string {
	return filepath.Join(careerOpsPath, "data", inboxSelectedFile)
}

// LoadSelectedURLs reads data/inbox-selected.tsv and returns a set of URLs.
// Missing file → empty set, no error. Lines starting with '#' are comments.
func LoadSelectedURLs(careerOpsPath string) map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(SelectedURLsPath(careerOpsPath))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		url := strings.TrimSpace(line)
		if url == "" || strings.HasPrefix(url, "#") {
			continue
		}
		out[url] = true
	}
	return out
}

// SaveSelectedURLs replaces data/inbox-selected.tsv with the given URLs.
// Empty list → file is removed (idempotent).
func SaveSelectedURLs(careerOpsPath string, urls []string) error {
	p := SelectedURLsPath(careerOpsPath)
	if len(urls) == 0 {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return nil
		}
		return os.Remove(p)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	sorted := append([]string(nil), urls...)
	sort.Strings(sorted)
	body := "# inbox selections — one URL per line. Auto-managed by the dashboard.\n" +
		strings.Join(sorted, "\n") + "\n"
	return os.WriteFile(p, []byte(body), 0644)
}

// SaveSelectedURLsFromSet is a convenience wrapper for callers holding a set.
func SaveSelectedURLsFromSet(careerOpsPath string, set map[string]bool) error {
	urls := make([]string, 0, len(set))
	for url, ok := range set {
		if ok {
			urls = append(urls, url)
		}
	}
	return SaveSelectedURLs(careerOpsPath, urls)
}

// PruneSelectedURLs removes entries from the sidecar that are no longer in the
// inbox (e.g., manually deleted). Returns the kept set. Safe no-op if the
// file is missing.
func PruneSelectedURLs(careerOpsPath string, validURLs map[string]bool) (map[string]bool, error) {
	current := LoadSelectedURLs(careerOpsPath)
	if len(current) == 0 {
		return current, nil
	}
	kept := map[string]bool{}
	for url := range current {
		if validURLs[url] {
			kept[url] = true
		}
	}
	if len(kept) == len(current) {
		return kept, nil
	}
	if err := SaveSelectedURLsFromSet(careerOpsPath, kept); err != nil {
		return current, fmt.Errorf("prune selections: %w", err)
	}
	return kept, nil
}
