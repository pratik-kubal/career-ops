package data

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/santifer/career-ops/dashboard/internal/model"
)

// ParseInbox reads data/pipeline.md and returns pending entries from the
// "Pendientes" section. Lines look like:
//
//	- [ ] URL | COMPANY | TITLE | LOCATION
//
// Older lines may omit LOCATION; defensively handle titles that contain `|`.
// Tries both {path}/data/pipeline.md and {path}/pipeline.md for compatibility.
func ParseInbox(careerOpsPath string) []model.InboxEntry {
	filePath := filepath.Join(careerOpsPath, "data", "pipeline.md")
	content, err := os.ReadFile(filePath)
	if err != nil {
		filePath = filepath.Join(careerOpsPath, "pipeline.md")
		content, err = os.ReadFile(filePath)
		if err != nil {
			return nil
		}
	}

	entries := make([]model.InboxEntry, 0)
	inPending := false

	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## Pendientes") {
			inPending = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inPending = false
			continue
		}
		if !inPending || !strings.HasPrefix(line, "- [ ]") {
			continue
		}

		body := strings.TrimSpace(strings.TrimPrefix(line, "- [ ]"))
		fields := splitPipeRow(body)
		if len(fields) < 2 || fields[0] == "" {
			continue
		}

		entry := model.InboxEntry{URL: fields[0], Company: fields[1]}
		switch {
		case len(fields) == 3:
			entry.Title = fields[2]
		case len(fields) == 4:
			entry.Title = fields[2]
			entry.Location = fields[3]
		case len(fields) >= 5:
			// Title contained literal `|` — rejoin middle fields, last is location.
			entry.Title = strings.Join(fields[2:len(fields)-1], " | ")
			entry.Location = fields[len(fields)-1]
		}

		ScoreInboxEntry(&entry)
		entries = append(entries, entry)
	}

	return entries
}

func splitPipeRow(s string) []string {
	parts := strings.Split(s, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// inboxPrimaryKeywords are IC engineering / developer titles for the user's
// primary archetypes (AI-First SWE, Backend SWE, Senior SWE, Java SWE,
// FinTech SWE per config/profile.yml). Match → strong fit signal.
//
// Kept narrow on purpose: "applied ai" alone catches Architect/Strategist/
// Evangelist roles which are FDE-style, not IC engineering. Use the explicit
// "applied ai engineer" instead. Same logic for "ai engineer" (which DOES
// catch "AI Engineer, X" — fine, those are IC roles).
var inboxPrimaryKeywords = []string{
	"software engineer",
	"software developer",
	"backend engineer", "back-end engineer", "backend developer", "back-end developer",
	"full stack engineer", "full-stack engineer", "fullstack engineer",
	"java engineer", "java developer",
	"ai engineer", "applied ai engineer", "ai software engineer",
	"ml engineer", "machine learning engineer", "llm engineer", "genai engineer",
	"agent engineer", "founding engineer", "production engineer",
	"distributed systems",
}

// inboxSecondaryKeywords map to secondary archetypes (Cloud Platform, FDE,
// Solutions Engineering, SRE, Data — adjacent fits) plus AI architecture-
// flavored roles that aren't pure IC engineering.
var inboxSecondaryKeywords = []string{
	"platform engineer", "cloud engineer", "infrastructure engineer",
	"site reliability", "sre", "reliability engineer",
	"devops engineer", "devsecops",
	"forward deployed", "deployed engineer",
	"solutions engineer", "solutions architect",
	"integration engineer", "customer engineer",
	"data engineer", "data platform",
	"principal engineer", "staff engineer", "senior engineer",
	"product engineer", "application engineer",
	"applied ai", "ai architect", "ai application engineer",
}

// inboxGoodLocations are locations Pratik can reasonably take — Remote, Philly,
// or NE Corridor cities reachable by drive/Amtrak. Empty location is treated
// as neutral (does NOT count as good — TOP FIT requires confirmed match).
var inboxGoodLocations = []string{
	"remote", "anywhere", "worldwide",
	"philadelphia", "philly", "pennsylvania", ", pa",
	"new york", "nyc", "manhattan", "brooklyn", ", ny",
	"newark", "jersey city", "princeton", "trenton", "new jersey", ", nj",
	"wilmington", "delaware", ", de",
	"baltimore", "maryland", ", md",
	"washington, dc", "washington dc", "washington d.c.",
	"united states", "usa", "u.s.",
	"east coast", "americas", "north america",
	"amer", "namer", // region abbreviations used by SaaS hiring (AMER, NAMER)
}

// ScoreInboxEntry classifies an entry into a fit tier:
//
//	Tier 3 (★★★) — primary title AND good location → TOP FIT
//	Tier 2 (★★)  — primary title alone, OR secondary title + good location
//	Tier 1 (★)   — everything else
func ScoreInboxEntry(e *model.InboxEntry) {
	titleLow := strings.ToLower(e.Title)
	locLow := strings.ToLower(e.Location)

	primary := containsAny(titleLow, inboxPrimaryKeywords)
	secondary := !primary && containsAny(titleLow, inboxSecondaryKeywords)
	goodLoc := containsAny(locLow, inboxGoodLocations)

	switch {
	case primary && goodLoc:
		e.FitTier = 3
		e.FitLabel = "★★★"
	case primary, secondary && goodLoc:
		e.FitTier = 2
		e.FitLabel = "★★"
	default:
		e.FitTier = 1
		e.FitLabel = "★"
	}
}

func containsAny(s string, kws []string) bool {
	for _, k := range kws {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// CountInboxByTier returns the number of entries at each tier.
// Index 0 → tier 1, index 1 → tier 2, index 2 → tier 3.
func CountInboxByTier(entries []model.InboxEntry) [3]int {
	var counts [3]int
	for _, e := range entries {
		if e.FitTier >= 1 && e.FitTier <= 3 {
			counts[e.FitTier-1]++
		}
	}
	return counts
}
