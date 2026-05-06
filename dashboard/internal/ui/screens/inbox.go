package screens

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/santifer/career-ops/dashboard/internal/model"
	"github.com/santifer/career-ops/dashboard/internal/theme"
)

// InboxClosedMsg is emitted when the inbox screen is dismissed.
type InboxClosedMsg struct{}

// InboxOpenURLMsg requests the host app to open a URL in the user's browser.
type InboxOpenURLMsg struct{ URL string }

// InboxRefreshMsg requests a re-parse of pipeline.md.
type InboxRefreshMsg struct{}

// PipelineOpenInboxMsg signals that the pipeline screen wants to switch to inbox.
type PipelineOpenInboxMsg struct{}

const (
	inboxFilterAll = "all"
	inboxFilterTop = "top"
)

type inboxTab struct {
	filter string
	label  string
}

var inboxTabs = []inboxTab{
	{inboxFilterAll, "ALL"},
	{inboxFilterTop, "TOP FIT"},
}

const (
	inboxSortFit     = "fit"
	inboxSortCompany = "company"
	inboxSortTitle   = "title"
)

var inboxSortCycle = []string{inboxSortFit, inboxSortCompany, inboxSortTitle}

// InboxModel is the unevaluated-offers screen sourced from data/pipeline.md.
type InboxModel struct {
	entries      []model.InboxEntry
	filtered     []model.InboxEntry
	cursor       int
	scrollOffset int
	activeTab    int
	sortMode     string
	width        int
	height       int
	theme        theme.Theme
}

// NewInboxModel constructs an inbox screen.
func NewInboxModel(t theme.Theme, entries []model.InboxEntry, width, height int) InboxModel {
	m := InboxModel{
		entries:   entries,
		sortMode:  inboxSortFit,
		activeTab: 0,
		width:     width,
		height:    height,
		theme:     t,
	}
	m.applyFilterAndSort()
	return m
}

// Init implements tea.Model.
func (m InboxModel) Init() tea.Cmd { return nil }

// Resize updates dimensions.
func (m *InboxModel) Resize(w, h int) {
	m.width = w
	m.height = h
}

// Width returns the current width.
func (m InboxModel) Width() int { return m.width }

// Height returns the current height.
func (m InboxModel) Height() int { return m.height }

// CurrentEntry returns the entry under the cursor, if any.
func (m InboxModel) CurrentEntry() (model.InboxEntry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return model.InboxEntry{}, false
	}
	return m.filtered[m.cursor], true
}

// WithReloadedData rebuilds the inbox preserving the user's tab/sort/cursor when possible.
func (m InboxModel) WithReloadedData(entries []model.InboxEntry) InboxModel {
	selectedURL := ""
	if e, ok := m.CurrentEntry(); ok {
		selectedURL = e.URL
	}
	reloaded := NewInboxModel(m.theme, entries, m.width, m.height)
	reloaded.activeTab = m.activeTab
	reloaded.sortMode = m.sortMode
	reloaded.applyFilterAndSort()
	if selectedURL != "" {
		for i, e := range reloaded.filtered {
			if e.URL == selectedURL {
				reloaded.cursor = i
				reloaded.adjustScroll()
				return reloaded
			}
		}
	}
	if m.cursor < len(reloaded.filtered) {
		reloaded.cursor = m.cursor
		reloaded.adjustScroll()
	}
	return reloaded
}

// Update handles input.
func (m InboxModel) Update(msg tea.Msg) (InboxModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m InboxModel) handleKey(msg tea.KeyMsg) (InboxModel, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m, func() tea.Msg { return InboxClosedMsg{} }

	case "down", "j":
		if len(m.filtered) > 0 {
			m.cursor++
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			m.adjustScroll()
		}

	case "up", "k":
		if len(m.filtered) > 0 {
			m.cursor--
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.adjustScroll()
		}

	case "f", "right", "l", "tab":
		m.activeTab = (m.activeTab + 1) % len(inboxTabs)
		m.applyFilterAndSort()
		m.cursor = 0
		m.scrollOffset = 0

	case "left", "h", "shift+tab":
		m.activeTab--
		if m.activeTab < 0 {
			m.activeTab = len(inboxTabs) - 1
		}
		m.applyFilterAndSort()
		m.cursor = 0
		m.scrollOffset = 0

	case "s":
		for i, s := range inboxSortCycle {
			if s == m.sortMode {
				m.sortMode = inboxSortCycle[(i+1)%len(inboxSortCycle)]
				break
			}
		}
		m.applyFilterAndSort()
		m.cursor = 0
		m.scrollOffset = 0

	case "g":
		m.cursor = 0
		m.scrollOffset = 0

	case "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
			m.adjustScroll()
		}

	case "pgdown", "ctrl+d":
		if len(m.filtered) > 0 {
			half := m.bodyHeight() / 2
			if half < 1 {
				half = 1
			}
			m.cursor += half
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			m.adjustScroll()
		}

	case "pgup", "ctrl+u":
		if len(m.filtered) > 0 {
			half := m.bodyHeight() / 2
			if half < 1 {
				half = 1
			}
			m.cursor -= half
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.adjustScroll()
		}

	case "enter", "o":
		if e, ok := m.CurrentEntry(); ok && e.URL != "" {
			return m, func() tea.Msg { return InboxOpenURLMsg{URL: e.URL} }
		}

	case "r":
		return m, func() tea.Msg { return InboxRefreshMsg{} }
	}
	return m, nil
}

// applyFilterAndSort filters by activeTab and sorts by sortMode.
func (m *InboxModel) applyFilterAndSort() {
	activeFilter := inboxTabs[m.activeTab].filter
	out := make([]model.InboxEntry, 0, len(m.entries))
	for _, e := range m.entries {
		switch activeFilter {
		case inboxFilterAll:
			out = append(out, e)
		case inboxFilterTop:
			if e.FitTier == 3 {
				out = append(out, e)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch m.sortMode {
		case inboxSortFit:
			if a.FitTier != b.FitTier {
				return a.FitTier > b.FitTier
			}
			if a.Company != b.Company {
				return strings.ToLower(a.Company) < strings.ToLower(b.Company)
			}
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		case inboxSortCompany:
			if a.Company != b.Company {
				return strings.ToLower(a.Company) < strings.ToLower(b.Company)
			}
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		case inboxSortTitle:
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		}
		return false
	})
	m.filtered = out
}

// adjustScroll keeps the cursor visible.
func (m *InboxModel) adjustScroll() {
	visible := m.bodyHeight()
	if visible < 1 {
		visible = 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m InboxModel) bodyHeight() int {
	// header(1) + tabs(2) + sortbar(1) + help(1) = 5; reserve a little extra.
	h := m.height - 6
	if h < 3 {
		h = 3
	}
	return h
}

// View renders the inbox.
func (m InboxModel) View() string {
	header := m.renderHeader()
	tabs := m.renderTabs()
	sortBar := m.renderSortBar()
	body := m.renderBody()
	help := m.renderHelp()
	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, sortBar, body, help)
}

func (m InboxModel) renderHeader() string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Text).
		Background(m.theme.Surface).
		Width(m.width).
		Padding(0, 2)

	title := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Mauve).Render("INBOX — UNEVALUATED OFFERS")

	counts := inboxCountsByTier(m.entries)
	info := lipgloss.NewStyle().Foreground(m.theme.Subtext).Render(
		fmt.Sprintf("%d total | ★★★ %d  ★★ %d  ★ %d",
			len(m.entries), counts[2], counts[1], counts[0]))

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(info) - 4
	if gap < 1 {
		gap = 1
	}
	return style.Render(title + strings.Repeat(" ", gap) + info)
}

func (m InboxModel) renderTabs() string {
	counts := inboxCountsByTier(m.entries)
	var tabs []string
	var underParts []string

	for i, tab := range inboxTabs {
		var n int
		switch tab.filter {
		case inboxFilterAll:
			n = len(m.entries)
		case inboxFilterTop:
			n = counts[2]
		}
		label := fmt.Sprintf(" %s (%d) ", tab.label, n)
		if i == m.activeTab {
			s := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Mauve)
			tabs = append(tabs, s.Render(label))
			underParts = append(underParts, strings.Repeat("━", lipgloss.Width(label)))
		} else {
			s := lipgloss.NewStyle().Foreground(m.theme.Subtext)
			tabs = append(tabs, s.Render(label))
			underParts = append(underParts, strings.Repeat("─", lipgloss.Width(label)))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	underline := lipgloss.NewStyle().Foreground(m.theme.Overlay).Render(strings.Join(underParts, ""))
	pad := lipgloss.NewStyle().Padding(0, 1)
	return pad.Render(row) + "\n" + pad.Render(underline)
}

func (m InboxModel) renderSortBar() string {
	style := lipgloss.NewStyle().Foreground(m.theme.Subtext).Width(m.width).Padding(0, 2)
	sortLabel := fmt.Sprintf("[Sort: %s]", m.sortMode)
	count := fmt.Sprintf("%d shown", len(m.filtered))
	return style.Render(fmt.Sprintf("%s  %s", sortLabel, count))
}

func (m InboxModel) renderBody() string {
	if len(m.filtered) == 0 {
		empty := lipgloss.NewStyle().Foreground(m.theme.Subtext).Padding(1, 2)
		return empty.Render("No entries in this tab.")
	}

	visible := m.bodyHeight()
	start := m.scrollOffset
	end := start + visible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, m.renderRow(i, m.filtered[i]))
	}
	return strings.Join(lines, "\n")
}

func (m InboxModel) renderRow(idx int, e model.InboxEntry) string {
	pad := lipgloss.NewStyle().Padding(0, 2)

	fitW := 5
	companyW := 20
	locW := 24
	titleW := m.width - fitW - companyW - locW - 12
	if titleW < 20 {
		titleW = 20
	}

	fitColor := m.theme.Subtext
	switch e.FitTier {
	case 3:
		fitColor = m.theme.Green
	case 2:
		fitColor = m.theme.Yellow
	}
	fitStyle := lipgloss.NewStyle().Foreground(fitColor).Bold(true).Width(fitW)
	companyStyle := lipgloss.NewStyle().Foreground(m.theme.Text).Width(companyW)
	titleStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext).Width(titleW)
	locStyle := lipgloss.NewStyle().Foreground(m.theme.Sky).Width(locW)

	loc := e.Location
	if loc == "" {
		loc = "—"
	}

	line := fmt.Sprintf(" %s %s %s %s",
		fitStyle.Render(truncateRunes(e.FitLabel, fitW)),
		companyStyle.Render(truncateRunes(e.Company, companyW)),
		titleStyle.Render(truncateRunes(e.Title, titleW)),
		locStyle.Render(truncateRunes(loc, locW)),
	)

	if idx == m.cursor {
		sel := lipgloss.NewStyle().Background(m.theme.Overlay).Width(m.width - 4)
		return pad.Render(sel.Render(line))
	}
	return pad.Render(line)
}

func (m InboxModel) renderHelp() string {
	style := lipgloss.NewStyle().
		Foreground(m.theme.Subtext).
		Background(m.theme.Surface).
		Width(m.width).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Text)
	descStyle := lipgloss.NewStyle().Foreground(m.theme.Subtext)
	brand := lipgloss.NewStyle().Foreground(m.theme.Overlay).Render("career-ops by santifer.io")

	keys := keyStyle.Render("↑↓/jk") + descStyle.Render(" nav  ") +
		keyStyle.Render("←→/hl") + descStyle.Render(" tabs  ") +
		keyStyle.Render("s") + descStyle.Render(" sort  ") +
		keyStyle.Render("o/Enter") + descStyle.Render(" open URL  ") +
		keyStyle.Render("r") + descStyle.Render(" refresh  ") +
		keyStyle.Render("Esc") + descStyle.Render(" back")

	gap := m.width - lipgloss.Width(keys) - lipgloss.Width(brand) - 2
	if gap < 1 {
		gap = 1
	}
	return style.Render(keys + strings.Repeat(" ", gap) + brand)
}

// inboxCountsByTier returns counts at indices 0=tier1, 1=tier2, 2=tier3.
func inboxCountsByTier(entries []model.InboxEntry) [3]int {
	var counts [3]int
	for _, e := range entries {
		if e.FitTier >= 1 && e.FitTier <= 3 {
			counts[e.FitTier-1]++
		}
	}
	return counts
}
