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

	"mseep/internal/app"
	"mseep/internal/config"
	"mseep/internal/health"
	"mseep/internal/marketplace"
)

// View modes
type viewMode int

const (
	viewServers viewMode = iota
	viewProfiles
	viewHealth
	viewApply
	viewMarketplace
)

// Key bindings
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Space    key.Binding
	Tab      key.Binding
	Help     key.Binding
	Quit     key.Binding
	Toggle   key.Binding
	Apply    key.Binding
	Health   key.Binding
	Profiles key.Binding
	Refresh  key.Binding
	Back     key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Space, k.Tab},
		{k.Toggle, k.Apply, k.Health, k.Profiles},
		{k.Refresh, k.Back, k.Help, k.Quit},
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
		key.WithKeys("h"),
		key.WithHelp("h", "health check"),
	),
	Profiles: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "profiles"),
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

// Styles
var (
	// Color palette
	primaryColor   = lipgloss.Color("#00D9FF")
	accentColor    = lipgloss.Color("#FF79C6")
	successColor   = lipgloss.Color("#50FA7B")
	warningColor   = lipgloss.Color("#FFB86C")
	errorColor     = lipgloss.Color("#FF5555")
	bgColor        = lipgloss.Color("#282A36")
	bgLightColor   = lipgloss.Color("#44475A")
	fgColor        = lipgloss.Color("#F8F8F2")
	mutedColor     = lipgloss.Color("#6272A4")
	borderColor    = lipgloss.Color("#6272A4")

	// App title
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
	
	// Tab styles with better separation
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
	
	// Content area
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

	// Info boxes
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

	// Section headers
	sectionHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(borderColor).
		MarginBottom(1).
		PaddingBottom(0)
)

// Server item for list
type serverItem struct {
	config.Server
}

func (i serverItem) Title() string {
	icon := "○"
	style := disabledStyle
	if i.Enabled {
		icon = "●"
		style = enabledStyle
	}
	
	name := style.Render(i.Name)
	badge := ""
	
	// Add transport badge if available
	if i.Transport != "" {
		transportBadge := lipgloss.NewStyle().
			Background(bgLightColor).
			Foreground(mutedColor).
			Padding(0, 1).
			Render(i.Transport)
		badge = " " + transportBadge
	}
	
	return fmt.Sprintf("%s %s%s", icon, name, badge)
}

func (i serverItem) Description() string {
	var parts []string
	
	if len(i.Tags) > 0 {
		tagStyle := lipgloss.NewStyle().Foreground(accentColor)
		tags := tagStyle.Render(fmt.Sprintf("🏷️  %s", strings.Join(i.Tags, ", ")))
		parts = append(parts, tags)
	}
	
	if i.Command != "" {
		cmdStyle := lipgloss.NewStyle().Foreground(mutedColor)
		cmd := i.Command
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
		}
		parts = append(parts, cmdStyle.Render(fmt.Sprintf("📟 %s", cmd)))
	}
	
	if len(i.Aliases) > 0 {
		aliasStyle := lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
		aliases := aliasStyle.Render(fmt.Sprintf("aka: %s", strings.Join(i.Aliases, ", ")))
		parts = append(parts, aliases)
	}
	
	return strings.Join(parts, "  ")
}

func (i serverItem) FilterValue() string {
	return i.Name
}

// Marketplace item for list
type marketplaceItem struct {
	marketplace.ServerEntry
}

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
		tagStyle := lipgloss.NewStyle().Foreground(accentColor)
		tags := tagStyle.Render(fmt.Sprintf("🏷️  %s", strings.Join(i.Tags, ", ")))
		parts = append(parts, tags)
	}
	
	if i.Repository != "" {
		repoStyle := lipgloss.NewStyle().Foreground(mutedColor)
		repo := i.Repository
		if len(repo) > 50 {
			repo = repo[:47] + "..."
		}
		parts = append(parts, repoStyle.Render(fmt.Sprintf("📂 %s", repo)))
	}
	
	return strings.Join(parts, "  ")
}

func (i marketplaceItem) FilterValue() string {
	return i.Name + " " + i.ServerEntry.Description + " " + strings.Join(i.Tags, " ")
}

// Model represents the TUI application state
type Model struct {
	app               *app.App
	mode              viewMode
	serverList        list.Model
	profileList       list.Model
	marketplaceList   list.Model
	viewport          viewport.Model
	spinner           spinner.Model
	help              help.Model
	marketplace       *marketplace.Marketplace
	marketplaceServers []marketplace.ServerEntry
	healthResults     []health.CheckResult
	width             int
	height            int
	showHelp          bool
	loading           bool
	message           string
	err               error
}

// New creates a new TUI model
func New() (*Model, error) {
	a, err := app.LoadApp()
	if err != nil {
		return nil, err
	}

	// Create server list
	items := make([]list.Item, 0, len(a.Canon.Servers))
	for _, server := range a.Canon.Servers {
		items = append(items, serverItem{server})
	}

	// Create server list with enhanced styling
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

	// Create profile list with enhanced styling
	profileItems := make([]list.Item, 0)
	for name := range a.Canon.Profiles {
		profileItems = append(profileItems, profileItem{name: name})
	}
	
	profileDelegate := list.NewDefaultDelegate()
	profileDelegate.Styles.SelectedTitle = selectedItemStyle
	profileDelegate.SetHeight(2)
	profileDelegate.SetSpacing(1)
	
	profileList := list.New(profileItems, profileDelegate, 0, 0)
	profileList.Title = "Profiles"
	profileList.SetShowStatusBar(false)
	profileList.SetShowHelp(false)
	profileList.Styles.Title = titleStyle

	// Create marketplace list with enhanced styling
	marketplaceDelegate := list.NewDefaultDelegate()
	marketplaceDelegate.Styles.SelectedTitle = selectedItemStyle
	marketplaceDelegate.Styles.SelectedDesc = selectedItemStyle.Copy().Foreground(mutedColor)
	marketplaceDelegate.SetHeight(4)  // Taller items for more info
	marketplaceDelegate.SetSpacing(1)
	
	marketplaceList := list.New([]list.Item{}, marketplaceDelegate, 0, 0)
	marketplaceList.Title = "MCP Marketplace"
	marketplaceList.SetShowStatusBar(false)
	marketplaceList.SetShowPagination(true)
	marketplaceList.SetShowHelp(false)
	marketplaceList.Styles.Title = titleStyle

	// Create spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF"))

	return &Model{
		app:             a,
		mode:            viewServers,
		serverList:      serverList,
		profileList:     profileList,
		marketplaceList: marketplaceList,
		viewport:        viewport.New(0, 0),
		spinner:         sp,
		help:            help.New(),
		marketplace:     marketplace.NewMarketplace(),
		showHelp:        false,
		loading:         false,
	}, nil
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.serverList.SetSize(msg.Width, msg.Height-4)
		m.profileList.SetSize(msg.Width, msg.Height-4)
		m.marketplaceList.SetSize(msg.Width, msg.Height-4)
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4
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
			m.mode = (m.mode + 1) % 5
			return m, nil

		case key.Matches(msg, keys.Left):
			if m.mode == 0 {
				m.mode = 4
			} else {
				m.mode--
			}
			return m, nil

		case key.Matches(msg, keys.Toggle), key.Matches(msg, keys.Space):
			if m.mode == viewServers {
				return m, m.toggleSelectedServer()
			}

		case key.Matches(msg, keys.Enter):
			if m.mode == viewProfiles {
				return m, m.applySelectedProfile()
			} else if m.mode == viewMarketplace {
				return m, m.installSelectedServer()
			}

		case key.Matches(msg, keys.Apply):
			return m, m.applyChanges()

		case key.Matches(msg, keys.Health):
			return m, m.runHealthCheck()

		case key.Matches(msg, keys.Refresh):
			return m, m.refresh()
		}

	case healthCheckMsg:
		m.loading = false
		m.healthResults = msg.results
		m.mode = viewHealth
		m.updateHealthView()
		return m, nil

	case applyMsg:
		m.loading = false
		if msg.err != nil {
			m.message = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.message = "Changes applied successfully"
		}
		return m, m.refresh()

	case refreshMsg:
		m.loading = false
		// Reload app state
		newApp, err := app.LoadApp()
		if err != nil {
			m.err = err
			return m, nil
		}
		m.app = newApp
		m.updateServerList()
		m.updateProfileList()
		return m, nil

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
		m.message = fmt.Sprintf("Loaded %d servers from marketplace", len(msg.servers))
		return m, nil

	case installMsg:
		m.loading = false
		m.app = msg.app
		m.message = fmt.Sprintf("Successfully installed server %q", msg.serverName)
		// Refresh marketplace to update installed status
		return m, m.loadMarketplace()

	case errorMsg:
		m.loading = false
		m.err = msg.err
		m.message = fmt.Sprintf("Error: %v", msg.err)
		return m, nil
	}

	// Update appropriate list based on mode
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
	case viewHealth, viewApply:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m Model) View() string {
	if m.err != nil {
		return errorBoxStyle.Render(fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err))
	}

	var sections []string

	// App title
	title := appTitleStyle.Render("🚀 mseep - MCP Server Manager")
	sections = append(sections, title)

	// Render tabs
	tabs := m.renderTabs()
	sections = append(sections, tabs)

	// Render content based on mode with enhanced styling
	var content string
	switch m.mode {
	case viewServers:
		content = m.renderServerView()
	case viewProfiles:
		content = m.renderProfileView()
	case viewHealth:
		content = m.renderHealthView()
	case viewApply:
		content = m.renderApplyView()
	case viewMarketplace:
		content = m.renderMarketplaceView()
	}
	
	// Apply content styling
	styledContent := contentStyle.Width(m.width).Height(m.height-8).Render(content)
	sections = append(sections, styledContent)

	// Render status bar
	status := m.renderStatusBar()
	sections = append(sections, status)

	// Compose full view
	fullView := lipgloss.JoinVertical(lipgloss.Top, sections...)

	if m.showHelp {
		helpView := m.renderHelpView()
		return lipgloss.JoinVertical(lipgloss.Top, fullView, helpView)
	}

	return fullView
}

func (m *Model) renderTabs() string {
	var tabs []string
	tabLabels := []string{"📦 Servers", "📋 Profiles", "🏥 Health", "🚀 Apply", "🛒 Marketplace"}
	tabIcons := []string{"📦", "📋", "🏥", "🚀", "🛒"}
	
	for i, label := range tabLabels {
		var tabContent string
		if viewMode(i) == m.mode {
			// Active tab with full label
			tabContent = activeTabStyle.Render(label)
		} else {
			// Inactive tab with icon only or shorter label
			simpleLabel := strings.TrimPrefix(label, tabIcons[i]+" ")
			tabContent = tabStyle.Render(tabIcons[i] + " " + simpleLabel)
		}
		tabs = append(tabs, tabContent)
	}
	
	// Add separators between tabs
	tabsWithSeparators := []string{}
	for i, tab := range tabs {
		tabsWithSeparators = append(tabsWithSeparators, tab)
		if i < len(tabs)-1 {
			tabsWithSeparators = append(tabsWithSeparators, tabSeparatorStyle.Render("│"))
		}
	}
	
	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, tabsWithSeparators...)
	return tabBarStyle.Width(m.width).Render(tabRow)
}

func (m *Model) renderServerView() string {
	if len(m.app.Canon.Servers) == 0 {
		return infoBoxStyle.Render("No servers configured. Add servers to your canonical configuration.")
	}

	var sections []string
	
	// Header with statistics
	enabled := 0
	disabled := 0
	for _, srv := range m.app.Canon.Servers {
		if srv.Enabled {
			enabled++
		} else {
			disabled++
		}
	}
	
	header := sectionHeaderStyle.Width(m.width - 4).Render("MCP Servers Configuration")
	sections = append(sections, header)
	
	statsBox := fmt.Sprintf("📊 Statistics: %s enabled | %s disabled | %d total",
		enabledStyle.Render(fmt.Sprintf("%d", enabled)),
		disabledStyle.Render(fmt.Sprintf("%d", disabled)),
		len(m.app.Canon.Servers))
	sections = append(sections, infoBoxStyle.Render(statsBox))
	
	// Server list
	sections = append(sections, m.serverList.View())
	
	return strings.Join(sections, "\n")
}

func (m *Model) renderProfileView() string {
	if len(m.app.Canon.Profiles) == 0 {
		return infoBoxStyle.Render("No profiles configured. Create profiles to quickly switch between server configurations.")
	}

	var sections []string
	
	header := sectionHeaderStyle.Width(m.width - 4).Render("Server Profiles")
	sections = append(sections, header)
	
	sections = append(sections, m.profileList.View())
	
	helpText := "💡 Press Enter to apply a profile | Press 'a' to apply current configuration"
	sections = append(sections, infoBoxStyle.Render(helpText))
	
	return strings.Join(sections, "\n")
}

func (m *Model) renderHealthView() string {
	var content strings.Builder
	
	header := sectionHeaderStyle.Width(m.width - 4).Render("🏥 Health Check Results")
	content.WriteString(header + "\n\n")
	
	if len(m.healthResults) == 0 {
		content.WriteString(infoBoxStyle.Render("No health checks run yet. Press 'h' to run health checks."))
		return content.String()
	}
	
	// Summary statistics
	healthy := 0
	unhealthy := 0
	timeout := 0
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
	
	summaryText := fmt.Sprintf("📊 Summary: %s healthy | %s unhealthy | %s timeout",
		enabledStyle.Render(fmt.Sprintf("%d", healthy)),
		lipgloss.NewStyle().Foreground(errorColor).Render(fmt.Sprintf("%d", unhealthy)),
		lipgloss.NewStyle().Foreground(warningColor).Render(fmt.Sprintf("%d", timeout)))
	content.WriteString(infoBoxStyle.Render(summaryText) + "\n\n")
	
	// Individual results
	content.WriteString("📋 Detailed Results:\n\n")
	for _, result := range m.healthResults {
		icon := "❓"
		var style lipgloss.Style
		
		switch result.Status {
		case health.StatusHealthy:
			icon = "✅"
			style = enabledStyle
		case health.StatusUnhealthy:
			icon = "❌"
			style = lipgloss.NewStyle().Foreground(errorColor)
		case health.StatusTimeout:
			icon = "⏱️"
			style = lipgloss.NewStyle().Foreground(warningColor)
		case health.StatusError:
			icon = "⚠️"
			style = lipgloss.NewStyle().Foreground(errorColor)
		}
		
		resultLine := fmt.Sprintf("%s %s - %s (%v)",
			icon,
			style.Render(result.ServerName),
			result.Message,
			result.Duration.Round(time.Millisecond))
		
		content.WriteString("  " + resultLine + "\n")
	}
	
	return content.String()
}

func (m *Model) renderApplyView() string {
	var sections []string
	
	header := sectionHeaderStyle.Width(m.width - 4).Render("🚀 Apply Configuration")
	sections = append(sections, header)
	
	instructions := `This will apply your canonical configuration to all detected clients.

Actions available:
• Press 'a' to apply changes
• Press 'r' to refresh and see current diff
• Press 'q' to go back

The system will:
1. Create automatic backups
2. Show a diff preview
3. Safely merge configurations
4. Preserve unmanaged servers`
	
	sections = append(sections, infoBoxStyle.Render(instructions))
	
	if m.message != "" {
		if strings.Contains(m.message, "success") {
			sections = append(sections, successBoxStyle.Render("✅ " + m.message))
		} else if strings.Contains(m.message, "Error") {
			sections = append(sections, errorBoxStyle.Render("❌ " + m.message))
		} else {
			sections = append(sections, infoBoxStyle.Render(m.message))
		}
	}
	
	return strings.Join(sections, "\n")
}

func (m *Model) renderHelpView() string {
	helpText := `
🎮 Keyboard Shortcuts
─────────────────────

Navigation:
  ↑/k, ↓/j    Move up/down
  ←/h, →/l    Previous/next tab
  tab         Switch view
  enter       Select item
  esc         Go back

Actions:
  t, space    Toggle server
  a           Apply changes
  h           Run health check
  p           View profiles
  r           Refresh
  
General:
  ?           Toggle help
  q           Quit

💡 Tips:
  • Use fuzzy search to find servers quickly
  • Create profiles to save server configurations
  • Health checks help identify issues
  • All changes create automatic backups
`
	return infoBoxStyle.Width(m.width).Render(helpText)
}

func (m *Model) renderStatusBar() string {
	status := ""
	
	if m.loading {
		status = m.spinner.View() + " Loading..."
	} else if m.message != "" && time.Since(time.Now()).Seconds() < 3 {
		status = m.message
	} else {
		switch m.mode {
		case viewServers:
			enabled := 0
			for _, srv := range m.app.Canon.Servers {
				if srv.Enabled {
					enabled++
				}
			}
			status = fmt.Sprintf("📦 %d/%d servers enabled | ⌨️ t: toggle | ?: help | q: quit",
				enabled, len(m.app.Canon.Servers))
		case viewProfiles:
			status = fmt.Sprintf("📋 %d profiles | ⌨️ enter: apply | ?: help | q: quit",
				len(m.app.Canon.Profiles))
		case viewHealth:
			if len(m.healthResults) > 0 {
				healthy := 0
				for _, r := range m.healthResults {
					if r.Status == health.StatusHealthy {
						healthy++
					}
				}
				status = fmt.Sprintf("🏥 %d/%d healthy | ⌨️ h: recheck | r: refresh | q: quit",
					healthy, len(m.healthResults))
			} else {
				status = "🏥 Press 'h' to run health checks | ?: help | q: quit"
			}
		case viewApply:
			status = "🚀 Press 'a' to apply | r: refresh | ?: help | q: quit"
		case viewMarketplace:
			if len(m.marketplaceServers) > 0 {
				installed := 0
				for _, srv := range m.marketplaceServers {
					if srv.Installed {
						installed++
					}
				}
				status = fmt.Sprintf("🛒 %d servers available | %d installed | ⌨️ enter: install | r: refresh | ?: help | q: quit",
					len(m.marketplaceServers), installed)
			} else {
				status = "🛒 Press 'r' to load marketplace | ?: help | q: quit"
			}
		}
	}
	
	return statusBarStyle.Width(m.width).Render(status)
}

// Helper methods for actions

func (m *Model) toggleSelectedServer() tea.Cmd {
	if item, ok := m.serverList.SelectedItem().(serverItem); ok {
		for i := range m.app.Canon.Servers {
			if m.app.Canon.Servers[i].Name == item.Name {
				m.app.Canon.Servers[i].Enabled = !m.app.Canon.Servers[i].Enabled
				config.Save("", m.app.Canon)
				m.updateServerList()
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
		for _, srv := range m.app.Canon.Servers {
			if srv.Enabled {
				servers = append(servers, srv)
			}
		}
		
		results := mgr.CheckServers(ctx, servers)
		return healthCheckMsg{results: results}
	}
}

func (m *Model) refresh() tea.Cmd {
	if m.mode == viewMarketplace {
		return m.loadMarketplace()
	}
	
	m.loading = true
	return func() tea.Msg {
		return refreshMsg{}
	}
}

func (m *Model) updateServerList() {
	items := make([]list.Item, 0, len(m.app.Canon.Servers))
	for _, server := range m.app.Canon.Servers {
		items = append(items, serverItem{server})
	}
	m.serverList.SetItems(items)
}

func (m *Model) updateProfileList() {
	items := make([]list.Item, 0, len(m.app.Canon.Profiles))
	for name := range m.app.Canon.Profiles {
		items = append(items, profileItem{name: name})
	}
	m.profileList.SetItems(items)
}

func (m *Model) updateHealthView() {
	var content strings.Builder
	content.WriteString("\n Health Check Results\n")
	content.WriteString(" ═══════════════════\n\n")
	
	for _, result := range m.healthResults {
		icon := "✗"
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
		
		if result.Status == health.StatusHealthy {
			icon = "✓"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#32CD32"))
		} else if result.Status == health.StatusTimeout {
			icon = "⏱"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
		}
		
		line := fmt.Sprintf(" %s %s - %s (%v)\n", icon, result.ServerName, result.Message, result.Duration.Round(time.Millisecond))
		content.WriteString(style.Render(line))
	}
	
	m.viewport.SetContent(content.String())
}

func (m *Model) renderMarketplaceView() string {
	var sections []string
	
	header := sectionHeaderStyle.Width(m.width - 4).Render("🛒 MCP Marketplace")
	sections = append(sections, header)
	
	if len(m.marketplaceServers) == 0 {
		instructions := `Welcome to the MCP Marketplace!

Discover and install MCP servers from the community:
• Press 'r' to refresh and load available servers
• Browse servers by name, description, or tags  
• Press 'enter' to install a server to your canonical config
• Installed servers are marked with ✅

The marketplace aggregates servers from:
• Official Anthropic MCP servers
• Community curated awesome lists
• GitHub repositories`
		
		sections = append(sections, infoBoxStyle.Render(instructions))
		return strings.Join(sections, "\n")
	}
	
	// Show marketplace statistics
	installed := 0
	for _, srv := range m.marketplaceServers {
		if srv.Installed {
			installed++
		}
	}
	
	statsBox := fmt.Sprintf("📊 Statistics: %d total servers | %s installed | %s available to install",
		len(m.marketplaceServers),
		enabledStyle.Render(fmt.Sprintf("%d", installed)),
		lipgloss.NewStyle().Foreground(primaryColor).Render(fmt.Sprintf("%d", len(m.marketplaceServers)-installed)))
	sections = append(sections, infoBoxStyle.Render(statsBox))
	
	// Server list
	sections = append(sections, m.marketplaceList.View())
	
	return strings.Join(sections, "\n")
}

func (m *Model) installSelectedServer() tea.Cmd {
	if item, ok := m.marketplaceList.SelectedItem().(marketplaceItem); ok {
		if item.Installed {
			m.message = fmt.Sprintf("Server %q is already installed", item.Name)
			return nil
		}
		
		m.loading = true
		return func() tea.Msg {
			err := m.marketplace.InstallServer(item.ServerEntry, m.app.Canon)
			if err != nil {
				return errorMsg{err: err}
			}
			// Reload app state after installation
			newApp, err := app.LoadApp()
			if err != nil {
				return errorMsg{err: err}
			}
			return installMsg{serverName: item.Name, app: newApp}
		}
	}
	return nil
}

func (m *Model) loadMarketplace() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		ctx := context.Background()
		servers, err := m.marketplace.GetServers(ctx, m.app.Canon)
		if err != nil {
			return errorMsg{err: err}
		}
		return marketplaceLoadedMsg{servers: servers}
	}
}

func (m *Model) updateMarketplaceList() {
	items := make([]list.Item, 0, len(m.marketplaceServers))
	for _, server := range m.marketplaceServers {
		items = append(items, marketplaceItem{server})
	}
	m.marketplaceList.SetItems(items)
}

// Profile item for list
type profileItem struct {
	name string
}

func (i profileItem) Title() string       { return i.name }
func (i profileItem) Description() string  { return "" }
func (i profileItem) FilterValue() string { return i.name }

// Messages
type healthCheckMsg struct {
	results []health.CheckResult
}

type applyMsg struct {
	err error
}

type refreshMsg struct{}

type errorMsg struct {
	err error
}

type marketplaceLoadedMsg struct {
	servers []marketplace.ServerEntry
}

type installMsg struct {
	serverName string
	app        *app.App
}