package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ResistanceIsUseless/mseep/internal/app"
	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/health"
	"github.com/ResistanceIsUseless/mseep/internal/marketplace"
)

// ---------------------------------------------------------------------------
// View modes — order determines tab rendering order
// ---------------------------------------------------------------------------

type viewMode int

const (
	viewServers     viewMode = iota // 0
	viewProfiles                    // 1
	viewStatus                      // 2  ← new: client sync status
	viewHealth                      // 3
	viewApply                       // 4
	viewMarketplace                 // 5
	viewModeCount   = 6
)

// ---------------------------------------------------------------------------
// Key bindings
// ---------------------------------------------------------------------------

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	Enter      key.Binding
	Space      key.Binding
	Tab        key.Binding
	Help       key.Binding
	Quit       key.Binding
	Toggle     key.Binding
	Apply      key.Binding
	Health     key.Binding
	Profiles   key.Binding
	Refresh    key.Binding
	Back       key.Binding
	ModeToggle key.Binding // 'm' — toggle basic↔wrapper
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Space, k.Tab},
		{k.Toggle, k.Apply, k.Health, k.Profiles},
		{k.ModeToggle, k.Refresh, k.Back, k.Quit},
		{k.Help},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev tab"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next tab"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch view"),
	),
	Toggle: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle server"),
	),
	Apply: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "apply changes"),
	),
	Health: key.NewBinding(
		key.WithKeys("H"),
		key.WithHelp("H", "health check"),
	),
	Profiles: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "profiles"),
	),
	ModeToggle: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "toggle mode"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r", "ctrl+r"),
		key.WithHelp("r", "refresh"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// ---------------------------------------------------------------------------
// Dracula-inspired colour palette
// ---------------------------------------------------------------------------

var (
	primaryColor  = lipgloss.Color("#00D9FF")
	accentColor   = lipgloss.Color("#FF79C6")
	successColor  = lipgloss.Color("#50FA7B")
	warningColor  = lipgloss.Color("#FFB86C")
	errorColor    = lipgloss.Color("#FF5555")
	bgColor       = lipgloss.Color("#282A36")
	bgLightColor  = lipgloss.Color("#44475A")
	fgColor       = lipgloss.Color("#F8F8F2")
	mutedColor    = lipgloss.Color("#6272A4")
	borderColor   = lipgloss.Color("#6272A4")
	wrapperColor  = lipgloss.Color("#BD93F9") // purple for wrapper mode

	appTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Background(bgColor).
			Padding(0, 2).
			MarginBottom(1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(1, 0)

	tabBarStyle = lipgloss.NewStyle().
			Background(bgColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(borderColor).
			PaddingTop(0).
			PaddingBottom(0)

	tabStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Background(bgColor).
			Padding(0, 2).
			MarginRight(1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fgColor).
			Background(bgLightColor).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 2).
			MarginRight(1)

	tabSeparatorStyle = lipgloss.NewStyle().
				Foreground(borderColor).
				Padding(0, 0)

	contentStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Background(lipgloss.Color("#1E1F29"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(fgColor).
			Background(bgColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(borderColor).
			Padding(0, 1)

	serverItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedItemStyle = serverItemStyle.Copy().
				Bold(true).
				Foreground(primaryColor).
				Background(bgLightColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderForeground(accentColor)

	enabledStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	disabledStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	infoBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Foreground(fgColor).
			Padding(1, 2).
			Margin(1, 0)

	successBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(successColor).
			Foreground(successColor).
			Padding(1, 2).
			Margin(1, 0)

	errorBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(errorColor).
			Foreground(errorColor).
			Padding(1, 2).
			Margin(1, 0)

	warnBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(warningColor).
			Foreground(warningColor).
			Padding(1, 2).
			Margin(1, 0)

	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(borderColor).
				MarginBottom(1).
				PaddingBottom(0)

	// Mode badge styles
	basicModeBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(bgColor).
			Background(successColor).
			Padding(0, 1)

	wrapperModeBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(bgColor).
				Background(wrapperColor).
				Padding(0, 1)
)

// ---------------------------------------------------------------------------
// List item types
// ---------------------------------------------------------------------------

type serverItem struct{ config.Server }

func (i serverItem) Title() string {
	icon := disabledStyle.Render("○")
	name := disabledStyle.Render(i.Name)
	if i.Enabled {
		icon = enabledStyle.Render("●")
		name = enabledStyle.Render(i.Name)
	}
	badge := ""
	if i.Transport != "" {
		badge = " " + lipgloss.NewStyle().
			Background(bgLightColor).Foreground(mutedColor).Padding(0, 1).
			Render(i.Transport)
	}
	return fmt.Sprintf("%s %s%s", icon, name, badge)
}

func (i serverItem) Description() string {
	var parts []string
	if len(i.Tags) > 0 {
		parts = append(parts,
			lipgloss.NewStyle().Foreground(accentColor).
				Render(fmt.Sprintf("🏷  %s", strings.Join(i.Tags, ", "))))
	}
	if i.Command != "" {
		cmd := i.Command
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
		}
		parts = append(parts,
			lipgloss.NewStyle().Foreground(mutedColor).
				Render(fmt.Sprintf("⌘  %s", cmd)))
	}
	if len(i.Aliases) > 0 {
		parts = append(parts,
			lipgloss.NewStyle().Foreground(mutedColor).Italic(true).
				Render(fmt.Sprintf("aka: %s", strings.Join(i.Aliases, ", "))))
	}
	return strings.Join(parts, "  ")
}

func (i serverItem) FilterValue() string { return i.Name }

type profileItem struct {
	name    string
	servers []string
}

func (i profileItem) Title() string { return "📋 " + i.name }
func (i profileItem) Description() string {
	if len(i.servers) == 0 {
		return lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("(empty profile)")
	}
	srv := strings.Join(i.servers, ", ")
	if len(srv) > 70 {
		srv = srv[:67] + "..."
	}
	return lipgloss.NewStyle().Foreground(mutedColor).Render(srv)
}
func (i profileItem) FilterValue() string { return i.name }

type marketplaceItem struct{ marketplace.ServerEntry }

func (i marketplaceItem) Title() string {
	icon := "📦"
	if i.Installed {
		icon = "✅"
	}
	name := i.Name
	if i.Author != "" {
		name += " by " + i.Author
	}
	return fmt.Sprintf("%s %s", icon, name)
}

func (i marketplaceItem) Description() string {
	var parts []string
	if i.ServerEntry.Description != "" {
		desc := i.ServerEntry.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		parts = append(parts, desc)
	}
	if len(i.Tags) > 0 {
		parts = append(parts,
			lipgloss.NewStyle().Foreground(accentColor).
				Render(fmt.Sprintf("🏷  %s", strings.Join(i.Tags, ", "))))
	}
	if i.Repository != "" {
		repo := i.Repository
		if len(repo) > 50 {
			repo = repo[:47] + "..."
		}
		parts = append(parts,
			lipgloss.NewStyle().Foreground(mutedColor).Render("📂 "+repo))
	}
	return strings.Join(parts, "  ")
}

func (i marketplaceItem) FilterValue() string {
	return i.Name + " " + i.ServerEntry.Description + " " + strings.Join(i.Tags, " ")
}

// ---------------------------------------------------------------------------
// Message types
// ---------------------------------------------------------------------------

type healthCheckMsg struct{ results []health.CheckResult }
type applyMsg struct{ err error }
type refreshMsg struct{}
type errorMsg struct{ err error }
type marketplaceLoadedMsg struct{ servers []marketplace.ServerEntry }
type installMsg struct {
	serverName string
	app        *app.App
}
type statusLoadedMsg struct{ report *app.StatusReport }
type modeToggledMsg struct{ newMode string }

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type Model struct {
	app                *app.App
	mode               viewMode
	serverList         list.Model
	profileList        list.Model
	marketplaceList    list.Model
	viewport           viewport.Model
	spinner            spinner.Model
	help               help.Model
	marketplace        *marketplace.Marketplace
	marketplaceServers []marketplace.ServerEntry
	healthResults      []health.CheckResult
	statusReport       *app.StatusReport
	width              int
	height             int
	showHelp           bool
	loading            bool
	message            string
	messageAt          time.Time
	err                error
}

// New creates a new TUI model.
func New() (*Model, error) {
	a, err := app.LoadApp()
	if err != nil {
		return nil, err
	}

	// Server list
	items := make([]list.Item, 0, len(a.Canon.Servers))
	for _, s := range a.Canon.Servers {
		items = append(items, serverItem{s})
	}
	serverDelegate := list.NewDefaultDelegate()
	serverDelegate.Styles.SelectedTitle = selectedItemStyle
	serverDelegate.Styles.SelectedDesc = selectedItemStyle.Copy().Foreground(mutedColor)
	serverDelegate.SetHeight(3)
	serverDelegate.SetSpacing(1)
	serverList := list.New(items, serverDelegate, 0, 0)
	serverList.Title = "MCP Servers"
	serverList.SetShowStatusBar(false)
	serverList.SetShowPagination(true)
	serverList.SetShowHelp(false)
	serverList.Styles.Title = titleStyle

	// Profile list
	profileItems := buildProfileItems(a.Canon)
	profileDelegate := list.NewDefaultDelegate()
	profileDelegate.Styles.SelectedTitle = selectedItemStyle
	profileDelegate.SetHeight(2)
	profileDelegate.SetSpacing(1)
	profileList := list.New(profileItems, profileDelegate, 0, 0)
	profileList.Title = "Profiles"
	profileList.SetShowStatusBar(false)
	profileList.SetShowHelp(false)
	profileList.Styles.Title = titleStyle

	// Marketplace list
	mktDelegate := list.NewDefaultDelegate()
	mktDelegate.Styles.SelectedTitle = selectedItemStyle
	mktDelegate.Styles.SelectedDesc = selectedItemStyle.Copy().Foreground(mutedColor)
	mktDelegate.SetHeight(4)
	mktDelegate.SetSpacing(1)
	mktList := list.New([]list.Item{}, mktDelegate, 0, 0)
	mktList.Title = "MCP Marketplace"
	mktList.SetShowStatusBar(false)
	mktList.SetShowPagination(true)
	mktList.SetShowHelp(false)
	mktList.Styles.Title = titleStyle

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)

	return &Model{
		app:             a,
		mode:            viewServers,
		serverList:      serverList,
		profileList:     profileList,
		marketplaceList: mktList,
		viewport:        viewport.New(0, 0),
		spinner:         sp,
		help:            help.New(),
		marketplace:     marketplace.NewMarketplace(),
	}, nil
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
		// Pre-load status on startup so the Status tab is ready immediately.
		m.loadStatus(),
	)
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentH := m.height - 8 // title + tabs + statusbar
		m.serverList.SetSize(msg.Width-4, contentH)
		m.profileList.SetSize(msg.Width-4, contentH)
		m.marketplaceList.SetSize(msg.Width-4, contentH)
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = contentH
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Right):
			m.mode = (m.mode + 1) % viewModeCount
			return m, m.onTabChange()

		case key.Matches(msg, keys.Left):
			if m.mode == 0 {
				m.mode = viewModeCount - 1
			} else {
				m.mode--
			}
			return m, m.onTabChange()

		case key.Matches(msg, keys.Toggle), key.Matches(msg, keys.Space):
			if m.mode == viewServers {
				return m, m.toggleSelectedServer()
			}

		case key.Matches(msg, keys.Enter):
			switch m.mode {
			case viewProfiles:
				return m, m.applySelectedProfile()
			case viewMarketplace:
				return m, m.installSelectedServer()
			}

		case key.Matches(msg, keys.Apply):
			return m, m.applyChanges()

		case key.Matches(msg, keys.Health):
			m.mode = viewHealth
			return m, m.runHealthCheck()

		case key.Matches(msg, keys.ModeToggle):
			return m, m.doModeToggle()

		case key.Matches(msg, keys.Refresh):
			return m, m.refresh()
		}

	// ── Async message handlers ──────────────────────────────────────────────

	case healthCheckMsg:
		m.loading = false
		m.healthResults = msg.results
		m.mode = viewHealth
		m.updateHealthViewport()
		return m, nil

	case applyMsg:
		m.loading = false
		if msg.err != nil {
			m.setMessage(fmt.Sprintf("Error: %v", msg.err))
		} else {
			m.setMessage("Changes applied successfully")
		}
		// Reload status after apply so the Status tab reflects the new state.
		return m, tea.Batch(m.refresh(), m.loadStatus())

	case refreshMsg:
		m.loading = false
		newApp, err := app.LoadApp()
		if err != nil {
			m.err = err
			return m, nil
		}
		m.app = newApp
		m.updateServerList()
		m.updateProfileList()
		return m, nil

	case statusLoadedMsg:
		m.loading = false
		m.statusReport = msg.report
		return m, nil

	case modeToggledMsg:
		m.loading = false
		m.setMessage(fmt.Sprintf("Mode switched to %q — run apply to push to clients", msg.newMode))
		newApp, err := app.LoadApp()
		if err == nil {
			m.app = newApp
		}
		return m, m.loadStatus()

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case marketplaceLoadedMsg:
		m.loading = false
		m.marketplaceServers = msg.servers
		m.updateMarketplaceList()
		m.setMessage(fmt.Sprintf("Loaded %d servers from marketplace", len(msg.servers)))
		return m, nil

	case installMsg:
		m.loading = false
		m.app = msg.app
		m.setMessage(fmt.Sprintf("Installed %q — toggle to enable, then apply", msg.serverName))
		return m, m.loadMarketplace()

	case errorMsg:
		m.loading = false
		m.err = msg.err
		m.setMessage(fmt.Sprintf("Error: %v", msg.err))
		return m, nil
	}

	// Delegate to active list/viewport.
	switch m.mode {
	case viewServers:
		var cmd tea.Cmd
		m.serverList, cmd = m.serverList.Update(msg)
		cmds = append(cmds, cmd)
	case viewProfiles:
		var cmd tea.Cmd
		m.profileList, cmd = m.profileList.Update(msg)
		cmds = append(cmds, cmd)
	case viewMarketplace:
		var cmd tea.Cmd
		m.marketplaceList, cmd = m.marketplaceList.Update(msg)
		cmds = append(cmds, cmd)
	case viewHealth, viewApply, viewStatus:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// onTabChange triggers appropriate data loads when switching tabs.
func (m *Model) onTabChange() tea.Cmd {
	switch m.mode {
	case viewStatus:
		if m.statusReport == nil {
			return m.loadStatus()
		}
	case viewMarketplace:
		if len(m.marketplaceServers) == 0 {
			return m.loadMarketplace()
		}
	case viewHealth:
		// Show existing results; user presses H to re-run.
	case viewApply:
		m.updateApplyViewport()
	}
	return nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m Model) View() string {
	if m.err != nil {
		return errorBoxStyle.Render(fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err))
	}

	var sections []string

	// Title row with mode badge
	modeBadge := m.renderModeBadge()
	titleRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		appTitleStyle.Render("⚡ mseep — MCP Server Manager"),
		"  ",
		modeBadge,
	)
	sections = append(sections, titleRow)

	// Tab bar
	sections = append(sections, m.renderTabs())

	// Content
	var content string
	switch m.mode {
	case viewServers:
		content = m.renderServerView()
	case viewProfiles:
		content = m.renderProfileView()
	case viewStatus:
		content = m.renderStatusView()
	case viewHealth:
		content = m.renderHealthView()
	case viewApply:
		content = m.renderApplyView()
	case viewMarketplace:
		content = m.renderMarketplaceView()
	}
	sections = append(sections, contentStyle.Width(m.width).Height(m.height-8).Render(content))

	// Status bar
	sections = append(sections, m.renderStatusBar())

	full := lipgloss.JoinVertical(lipgloss.Top, sections...)
	if m.showHelp {
		return lipgloss.JoinVertical(lipgloss.Top, full, m.renderHelpView())
	}
	return full
}

// ---------------------------------------------------------------------------
// Tab rendering
// ---------------------------------------------------------------------------

var tabDefs = []struct {
	icon  string
	label string
}{
	{"📦", "Servers"},
	{"📋", "Profiles"},
	{"🖥 ", "Status"},
	{"🏥", "Health"},
	{"🚀", "Apply"},
	{"🛒", "Market"},
}

func (m *Model) renderTabs() string {
	var tabs []string
	for i, td := range tabDefs {
		full := td.icon + " " + td.label
		var tab string
		if viewMode(i) == m.mode {
			tab = activeTabStyle.Render(full)
		} else {
			tab = tabStyle.Render(full)
		}
		tabs = append(tabs, tab)
		if i < len(tabDefs)-1 {
			tabs = append(tabs, tabSeparatorStyle.Render("│"))
		}
	}
	return tabBarStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, tabs...),
	)
}

func (m *Model) renderModeBadge() string {
	mode := m.app.Canon.EffectiveMode()
	if mode == "wrapper" {
		return wrapperModeBadge.Render("⚡ WRAPPER")
	}
	return basicModeBadge.Render("  BASIC  ")
}

// ---------------------------------------------------------------------------
// Server view
// ---------------------------------------------------------------------------

func (m *Model) renderServerView() string {
	if len(m.app.Canon.Servers) == 0 {
		return infoBoxStyle.Render("No servers configured.\n\nAdd servers to your canonical configuration:\n  mseep marketplace search <name>\n  mseep enable <name>")
	}

	enabled, disabled := 0, 0
	for _, s := range m.app.Canon.Servers {
		if s.Enabled {
			enabled++
		} else {
			disabled++
		}
	}

	header := sectionHeaderStyle.Width(m.width - 4).Render("MCP Servers Configuration")
	stats := fmt.Sprintf("📊  %s enabled  ·  %s disabled  ·  %d total  ·  mode: %s   [m] to toggle mode",
		enabledStyle.Render(fmt.Sprintf("%d", enabled)),
		disabledStyle.Render(fmt.Sprintf("%d", disabled)),
		len(m.app.Canon.Servers),
		m.app.Canon.EffectiveMode(),
	)
	return strings.Join([]string{
		header,
		infoBoxStyle.Render(stats),
		m.serverList.View(),
	}, "\n")
}

// ---------------------------------------------------------------------------
// Profile view
// ---------------------------------------------------------------------------

func (m *Model) renderProfileView() string {
	header := sectionHeaderStyle.Width(m.width - 4).Render("Server Profiles")
	if len(m.app.Canon.Profiles) == 0 {
		return strings.Join([]string{
			header,
			infoBoxStyle.Render("No profiles yet.\n\nCreate one:\n  mseep profiles create <name> server1 server2\n  mseep profiles save <name>"),
		}, "\n")
	}
	return strings.Join([]string{
		header,
		m.profileList.View(),
		infoBoxStyle.Render("↵ apply profile  ·  a: apply canonical  ·  r: refresh"),
	}, "\n")
}

// ---------------------------------------------------------------------------
// Status view (new) — shows all clients and their sync state
// ---------------------------------------------------------------------------

func (m *Model) renderStatusView() string {
	header := sectionHeaderStyle.Width(m.width - 4).Render("Client Sync Status")

	if m.loading || m.statusReport == nil {
		return strings.Join([]string{header, infoBoxStyle.Render(m.spinner.View() + "  Loading client status…")}, "\n")
	}

	var sb strings.Builder

	// Column widths
	colName    := 14
	colDetect  := 10
	colPath    := 38
	colEnabled := 12

	// Header row
	headerRow := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %s",
		colName, "CLIENT",
		colDetect, "DETECTED",
		colPath, "CONFIG PATH",
		colEnabled, "ENABLED",
		"SYNC",
	)
	sb.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Bold(true).Render(headerRow) + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(borderColor).Render("  " + strings.Repeat("─", m.width-8)) + "\n\n")

	inSync    := 0
	outOfSync := 0
	detected  := 0

	for _, cs := range m.statusReport.Clients {
		// Detected indicator
		detectedStr := disabledStyle.Render("  ✗  not found")
		if cs.Installed {
			detected++
			detectedStr = enabledStyle.Render("  ✓  found")
		}

		// Shorten path for display
		displayPath := cs.Path
		if len(displayPath) > colPath {
			displayPath = "…" + displayPath[len(displayPath)-(colPath-1):]
		}
		if displayPath == "" {
			displayPath = "—"
		}

		// Count enabled canonical servers vs what's in the client
		canonEnabled := 0
		for _, s := range m.app.Canon.Servers {
			if s.Enabled {
				canonEnabled++
			}
		}

		// Count in-sync servers
		syncedCount := 0
		for _, ss := range cs.Servers {
			if ss.InSync {
				syncedCount++
			}
		}

		syncStr := lipgloss.NewStyle().Foreground(mutedColor).Render("—")
		if cs.Installed {
			if len(cs.Servers) == 0 && canonEnabled == 0 {
				syncStr = enabledStyle.Render("✓ in sync")
				inSync++
			} else if syncedCount == canonEnabled && canonEnabled > 0 {
				syncStr = enabledStyle.Render("✓ in sync")
				inSync++
			} else {
				syncStr = lipgloss.NewStyle().Foreground(warningColor).Render("⚠ out of sync")
				outOfSync++
			}
		}

		enabledStr := lipgloss.NewStyle().Foreground(mutedColor).Render("—")
		if cs.Installed {
			enabledStr = fmt.Sprintf("%d/%d servers", syncedCount, canonEnabled)
		}

		row := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %s",
			colName, cs.Name,
			colDetect+10, detectedStr, // +10 for ANSI codes
			colPath, displayPath,
			colEnabled+10, enabledStr,
			syncStr,
		)
		sb.WriteString(row + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(borderColor).Render("  " + strings.Repeat("─", m.width-8)) + "\n")

	// Summary
	summaryParts := []string{
		fmt.Sprintf("%s detected", enabledStyle.Render(fmt.Sprintf("%d", detected))),
		fmt.Sprintf("%s in sync", enabledStyle.Render(fmt.Sprintf("%d", inSync))),
	}
	if outOfSync > 0 {
		summaryParts = append(summaryParts,
			lipgloss.NewStyle().Foreground(warningColor).Render(fmt.Sprintf("%d out of sync — press 'a' to apply", outOfSync)))
	}
	sb.WriteString("\n  " + strings.Join(summaryParts, "  ·  ") + "\n")

	// Mode info
	modeInfo := fmt.Sprintf("\n  Delivery mode: %s    [m] to toggle", m.app.Canon.EffectiveMode())
	sb.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render(modeInfo) + "\n")

	return strings.Join([]string{header, sb.String()}, "\n")
}

// ---------------------------------------------------------------------------
// Health view
// ---------------------------------------------------------------------------

func (m *Model) renderHealthView() string {
	header := sectionHeaderStyle.Width(m.width - 4).Render("Health Check Results")

	if m.loading {
		return strings.Join([]string{header, infoBoxStyle.Render(m.spinner.View() + "  Running health checks…")}, "\n")
	}
	if len(m.healthResults) == 0 {
		return strings.Join([]string{header, infoBoxStyle.Render("No health checks run yet.\n\nPress H to run checks against all enabled servers.")}, "\n")
	}

	healthy, unhealthy, timeout := 0, 0, 0
	for _, r := range m.healthResults {
		switch r.Status {
		case health.StatusHealthy:
			healthy++
		case health.StatusUnhealthy:
			unhealthy++
		case health.StatusTimeout:
			timeout++
		}
	}
	summary := fmt.Sprintf("📊  %s healthy  ·  %s unhealthy  ·  %s timeout",
		enabledStyle.Render(fmt.Sprintf("%d", healthy)),
		lipgloss.NewStyle().Foreground(errorColor).Render(fmt.Sprintf("%d", unhealthy)),
		lipgloss.NewStyle().Foreground(warningColor).Render(fmt.Sprintf("%d", timeout)))

	var results strings.Builder
	results.WriteString("Results:\n\n")
	for _, r := range m.healthResults {
		icon, st := "❓", lipgloss.NewStyle()
		switch r.Status {
		case health.StatusHealthy:
			icon, st = "✅", enabledStyle
		case health.StatusUnhealthy:
			icon, st = "❌", lipgloss.NewStyle().Foreground(errorColor)
		case health.StatusTimeout:
			icon, st = "⏱ ", lipgloss.NewStyle().Foreground(warningColor)
		case health.StatusError:
			icon, st = "⚠️ ", lipgloss.NewStyle().Foreground(errorColor)
		}
		results.WriteString(fmt.Sprintf("  %s %s  %s  (%v)\n",
			icon,
			st.Render(fmt.Sprintf("%-20s", r.ServerName)),
			r.Message,
			r.Duration.Round(time.Millisecond)))
	}

	return strings.Join([]string{
		header,
		infoBoxStyle.Render(summary),
		results.String(),
	}, "\n")
}

// ---------------------------------------------------------------------------
// Apply view
// ---------------------------------------------------------------------------

func (m *Model) renderApplyView() string {
	header := sectionHeaderStyle.Width(m.width - 4).Render("Apply Configuration")

	modeStr := m.app.Canon.EffectiveMode()
	var modeBox string
	if modeStr == "wrapper" {
		modeBox = warnBoxStyle.Render(
			"Mode: WRAPPER\n" +
				"Each detected client will be updated to run  mseep proxy --client <name>\n" +
				"mseep will multiplex all enabled servers at runtime.",
		)
	} else {
		modeBox = infoBoxStyle.Render(
			"Mode: BASIC\n" +
				"All enabled servers will be written directly into each client's config file.",
		)
	}

	// List detected clients
	var clientLines strings.Builder
	clientLines.WriteString("Detected clients:\n")
	if m.statusReport != nil {
		for _, cs := range m.statusReport.Clients {
			if cs.Installed {
				clientLines.WriteString(fmt.Sprintf("  %s  %s\n",
					enabledStyle.Render("●"),
					cs.Name))
			}
		}
	} else {
		clientLines.WriteString("  (loading…)\n")
	}

	instructions := `Actions:
  a  — apply to all detected clients
  r  — refresh
  m  — toggle delivery mode
  ?  — help

Apply will:
  1. Create timestamped backups of each client config
  2. Preview a diff and ask for confirmation
  3. Safely merge, preserving unmanaged server entries`

	var msgBox string
	if m.message != "" {
		if strings.Contains(m.message, "success") || strings.Contains(m.message, "applied") {
			msgBox = successBoxStyle.Render("✅ " + m.message)
		} else if strings.Contains(m.message, "Error") {
			msgBox = errorBoxStyle.Render("❌ " + m.message)
		} else {
			msgBox = infoBoxStyle.Render(m.message)
		}
	}

	parts := []string{header, modeBox, infoBoxStyle.Render(clientLines.String()), infoBoxStyle.Render(instructions)}
	if msgBox != "" {
		parts = append(parts, msgBox)
	}
	return strings.Join(parts, "\n")
}

// updateApplyViewport rebuilds the apply viewport content (used on tab switch).
func (m *Model) updateApplyViewport() {
	m.viewport.SetContent(m.renderApplyView())
}

// ---------------------------------------------------------------------------
// Marketplace view
// ---------------------------------------------------------------------------

func (m *Model) renderMarketplaceView() string {
	header := sectionHeaderStyle.Width(m.width - 4).Render("MCP Marketplace")

	if m.loading {
		return strings.Join([]string{header, infoBoxStyle.Render(m.spinner.View() + "  Loading marketplace…")}, "\n")
	}
	if len(m.marketplaceServers) == 0 {
		return strings.Join([]string{header, infoBoxStyle.Render(
			"Discover and install MCP servers.\n\n" +
				"  r  — load available servers\n" +
				"  ↵  — install selected server\n\n" +
				"Sources: mcpservers.org, GitHub awesome lists",
		)}, "\n")
	}

	installed := 0
	for _, s := range m.marketplaceServers {
		if s.Installed {
			installed++
		}
	}
	stats := fmt.Sprintf("📊  %d total  ·  %s installed  ·  %s available",
		len(m.marketplaceServers),
		enabledStyle.Render(fmt.Sprintf("%d", installed)),
		lipgloss.NewStyle().Foreground(primaryColor).Render(fmt.Sprintf("%d", len(m.marketplaceServers)-installed)))

	return strings.Join([]string{header, infoBoxStyle.Render(stats), m.marketplaceList.View()}, "\n")
}

// ---------------------------------------------------------------------------
// Help overlay
// ---------------------------------------------------------------------------

func (m *Model) renderHelpView() string {
	return infoBoxStyle.Width(m.width).Render(`Keyboard Shortcuts
──────────────────
Navigation:   ↑/k · ↓/j · ←/h · →/l · tab
Actions:      t/space: toggle server  ·  a: apply  ·  H: health check
              m: toggle mode (basic↔wrapper)  ·  p: profiles  ·  r: refresh
              ↵: select (profile/marketplace)
General:      ?: help  ·  q: quit  ·  esc: back

Tips:
  • Toggle a server with t/space, then apply with a
  • Mode badge in the title shows current delivery mode
  • Status tab shows which clients are detected and in sync
  • Marketplace auto-loads when you switch to the tab`)
}

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------

func (m *Model) renderStatusBar() string {
	var status string
	if m.loading {
		status = m.spinner.View() + " Loading…"
	} else if m.message != "" && time.Since(m.messageAt) < 5*time.Second {
		status = m.message
	} else {
		switch m.mode {
		case viewServers:
			enabled := 0
			for _, s := range m.app.Canon.Servers {
				if s.Enabled {
					enabled++
				}
			}
			status = fmt.Sprintf("📦 %d/%d enabled  ·  t: toggle  ·  m: mode  ·  a: apply  ·  ?: help  ·  q: quit",
				enabled, len(m.app.Canon.Servers))
		case viewProfiles:
			status = fmt.Sprintf("📋 %d profiles  ·  ↵: apply profile  ·  ?: help  ·  q: quit",
				len(m.app.Canon.Profiles))
		case viewStatus:
			detected := 0
			if m.statusReport != nil {
				for _, cs := range m.statusReport.Clients {
					if cs.Installed {
						detected++
					}
				}
			}
			status = fmt.Sprintf("🖥  %d clients detected  ·  r: refresh  ·  a: apply  ·  q: quit", detected)
		case viewHealth:
			if len(m.healthResults) > 0 {
				ok := 0
				for _, r := range m.healthResults {
					if r.Status == health.StatusHealthy {
						ok++
					}
				}
				status = fmt.Sprintf("🏥 %d/%d healthy  ·  H: recheck  ·  r: refresh  ·  q: quit",
					ok, len(m.healthResults))
			} else {
				status = "🏥 Press H to run health checks  ·  q: quit"
			}
		case viewApply:
			status = "🚀 a: apply  ·  m: toggle mode  ·  r: refresh  ·  ?: help  ·  q: quit"
		case viewMarketplace:
			status = fmt.Sprintf("🛒 %d servers  ·  ↵: install  ·  r: refresh  ·  q: quit",
				len(m.marketplaceServers))
		}
	}
	return statusBarStyle.Width(m.width).Render(status)
}

// ---------------------------------------------------------------------------
// Action helpers
// ---------------------------------------------------------------------------

func (m *Model) toggleSelectedServer() tea.Cmd {
	if item, ok := m.serverList.SelectedItem().(serverItem); ok {
		for i := range m.app.Canon.Servers {
			if m.app.Canon.Servers[i].Name == item.Name {
				m.app.Canon.Servers[i].Enabled = !m.app.Canon.Servers[i].Enabled
				_ = config.Save("", m.app.Canon)
				m.updateServerList()
				enabled := m.app.Canon.Servers[i].Enabled
				state := "disabled"
				if enabled {
					state = "enabled"
				}
				m.setMessage(fmt.Sprintf("%q %s — press 'a' to apply to clients", item.Name, state))
				break
			}
		}
	}
	return nil
}

func (m *Model) applySelectedProfile() tea.Cmd {
	if item, ok := m.profileList.SelectedItem().(profileItem); ok {
		m.loading = true
		return func() tea.Msg {
			err := m.app.Apply("", item.name, true)
			return applyMsg{err: err}
		}
	}
	return nil
}

func (m *Model) applyChanges() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		err := m.app.Apply("", "", true)
		return applyMsg{err: err}
	}
}

func (m *Model) runHealthCheck() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		ctx := context.Background()
		mgr := health.NewManager()
		servers := []config.Server{}
		for _, s := range m.app.Canon.Servers {
			if s.Enabled {
				servers = append(servers, s)
			}
		}
		results := mgr.CheckServers(ctx, servers)
		return healthCheckMsg{results: results}
	}
}

// doModeToggle switches between basic and wrapper, persists to canonical.
func (m *Model) doModeToggle() tea.Cmd {
	m.loading = true
	currentMode := m.app.Canon.EffectiveMode()
	return func() tea.Msg {
		canon, err := config.Load("")
		if err != nil {
			return errorMsg{err: err}
		}
		if currentMode == "wrapper" {
			canon.Settings.Mode = "basic"
		} else {
			canon.Settings.Mode = "wrapper"
			// Initialise defaults if not already set.
			if canon.Settings.Wrapper.PollIntervalSeconds == 0 {
				canon.Settings.Wrapper.PollIntervalSeconds = 5
			}
			if !canon.Settings.Wrapper.NamespaceTools {
				canon.Settings.Wrapper.NamespaceTools = true
			}
			if canon.Settings.Wrapper.NamespaceSeparator == "" {
				canon.Settings.Wrapper.NamespaceSeparator = "__"
			}
		}
		if err := config.Save("", canon); err != nil {
			return errorMsg{err: err}
		}
		return modeToggledMsg{newMode: canon.Settings.Mode}
	}
}

func (m *Model) loadStatus() tea.Cmd {
	return func() tea.Msg {
		report, err := m.app.GetStatusReport("")
		if err != nil {
			return errorMsg{err: err}
		}
		return statusLoadedMsg{report: report}
	}
}

func (m *Model) refresh() tea.Cmd {
	if m.mode == viewMarketplace {
		return m.loadMarketplace()
	}
	m.loading = true
	return func() tea.Msg { return refreshMsg{} }
}

func (m *Model) loadMarketplace() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		servers, err := m.marketplace.GetServers(context.Background(), m.app.Canon)
		if err != nil {
			return errorMsg{err: err}
		}
		return marketplaceLoadedMsg{servers: servers}
	}
}

func (m *Model) installSelectedServer() tea.Cmd {
	if item, ok := m.marketplaceList.SelectedItem().(marketplaceItem); ok {
		if item.Installed {
			m.setMessage(fmt.Sprintf("%q is already installed", item.Name))
			return nil
		}
		m.loading = true
		return func() tea.Msg {
			if err := m.marketplace.InstallServer(item.ServerEntry, m.app.Canon); err != nil {
				return errorMsg{err: err}
			}
			newApp, err := app.LoadApp()
			if err != nil {
				return errorMsg{err: err}
			}
			return installMsg{serverName: item.Name, app: newApp}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// List update helpers
// ---------------------------------------------------------------------------

func (m *Model) updateServerList() {
	items := make([]list.Item, 0, len(m.app.Canon.Servers))
	for _, s := range m.app.Canon.Servers {
		items = append(items, serverItem{s})
	}
	m.serverList.SetItems(items)
}

func (m *Model) updateProfileList() {
	m.profileList.SetItems(buildProfileItems(m.app.Canon))
}

func buildProfileItems(canon *config.Canonical) []list.Item {
	items := make([]list.Item, 0, len(canon.Profiles))
	for name, servers := range canon.Profiles {
		items = append(items, profileItem{name: name, servers: servers})
	}
	return items
}

func (m *Model) updateMarketplaceList() {
	items := make([]list.Item, 0, len(m.marketplaceServers))
	for _, s := range m.marketplaceServers {
		items = append(items, marketplaceItem{s})
	}
	m.marketplaceList.SetItems(items)
}

func (m *Model) updateHealthViewport() {
	var b strings.Builder
	for _, r := range m.healthResults {
		icon, color := "❓", fgColor
		switch r.Status {
		case health.StatusHealthy:
			icon, color = "✅", successColor
		case health.StatusUnhealthy:
			icon, color = "❌", errorColor
		case health.StatusTimeout:
			icon, color = "⏱ ", warningColor
		case health.StatusError:
			icon, color = "⚠️ ", errorColor
		}
		b.WriteString(lipgloss.NewStyle().Foreground(color).Render(
			fmt.Sprintf("  %s  %-20s  %s  (%v)\n",
				icon, r.ServerName, r.Message, r.Duration.Round(time.Millisecond)),
		))
	}
	m.viewport.SetContent(b.String())
}

// setMessage sets the status bar message and records when it was set.
func (m *Model) setMessage(msg string) {
	m.message = msg
	m.messageAt = time.Now()
}
