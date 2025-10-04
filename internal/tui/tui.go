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
)

// View modes
type viewMode int

const (
	viewServers viewMode = iota
	viewProfiles
	viewHealth
	viewApply
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
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00D9FF")).
		Padding(1, 0)
	
	tabStyle = lipgloss.NewStyle().
		Padding(0, 1)
	
	activeTabStyle = tabStyle.Copy().
		Bold(true).
		Foreground(lipgloss.Color("#00D9FF")).
		BorderBottom(true).
		BorderStyle(lipgloss.ThickBorder())
	
	statusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
		Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"})
	
	serverItemStyle = lipgloss.NewStyle().
		PaddingLeft(2)
	
	selectedItemStyle = serverItemStyle.Copy().
		Foreground(lipgloss.Color("#00D9FF"))
	
	enabledStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#32CD32"))
	
	disabledStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#808080"))
)

// Server item for list
type serverItem struct {
	config.Server
}

func (i serverItem) Title() string {
	status := "◯"
	if i.Enabled {
		status = "●"
	}
	return fmt.Sprintf("%s %s", status, i.Name)
}

func (i serverItem) Description() string {
	desc := []string{}
	if len(i.Tags) > 0 {
		desc = append(desc, fmt.Sprintf("Tags: %s", strings.Join(i.Tags, ", ")))
	}
	if i.Command != "" {
		cmd := i.Command
		if len(cmd) > 30 {
			cmd = cmd[:27] + "..."
		}
		desc = append(desc, fmt.Sprintf("Cmd: %s", cmd))
	}
	return strings.Join(desc, " | ")
}

func (i serverItem) FilterValue() string {
	return i.Name
}

// Model represents the TUI application state
type Model struct {
	app          *app.App
	mode         viewMode
	serverList   list.Model
	profileList  list.Model
	viewport     viewport.Model
	spinner      spinner.Model
	help         help.Model
	healthResults []health.CheckResult
	width        int
	height       int
	showHelp     bool
	loading      bool
	message      string
	err          error
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

	serverList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	serverList.Title = "MCP Servers"
	serverList.SetShowStatusBar(true)
	serverList.SetShowPagination(true)
	serverList.SetShowHelp(false)
	serverList.Styles.Title = titleStyle

	// Create profile list
	profileItems := make([]list.Item, 0)
	for name := range a.Canon.Profiles {
		profileItems = append(profileItems, profileItem{name: name})
	}
	
	profileList := list.New(profileItems, list.NewDefaultDelegate(), 0, 0)
	profileList.Title = "Profiles"
	profileList.SetShowStatusBar(true)
	profileList.SetShowHelp(false)
	profileList.Styles.Title = titleStyle

	// Create spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF"))

	return &Model{
		app:         a,
		mode:        viewServers,
		serverList:  serverList,
		profileList: profileList,
		viewport:    viewport.New(0, 0),
		spinner:     sp,
		help:        help.New(),
		showHelp:    false,
		loading:     false,
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
			m.mode = (m.mode + 1) % 4
			return m, nil

		case key.Matches(msg, keys.Left):
			if m.mode == 0 {
				m.mode = 3
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
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.\n", m.err)
	}

	var content string

	// Render tabs
	tabs := m.renderTabs()

	// Render content based on mode
	switch m.mode {
	case viewServers:
		content = m.serverList.View()
	case viewProfiles:
		content = m.profileList.View()
	case viewHealth:
		content = m.viewport.View()
	case viewApply:
		content = m.viewport.View()
	}

	// Render status bar
	status := m.renderStatusBar()

	// Compose full view
	fullView := lipgloss.JoinVertical(
		lipgloss.Top,
		tabs,
		content,
		status,
	)

	if m.showHelp {
		helpView := m.help.View(keys)
		return lipgloss.JoinVertical(lipgloss.Top, fullView, helpView)
	}

	return fullView
}

func (m *Model) renderTabs() string {
	tabs := []string{}
	
	for i, label := range []string{"Servers", "Profiles", "Health", "Apply"} {
		if viewMode(i) == m.mode {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, tabStyle.Render(label))
		}
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m *Model) renderStatusBar() string {
	status := ""
	
	if m.loading {
		status = m.spinner.View() + " Loading..."
	} else if m.message != "" {
		status = m.message
		// Clear message after displaying
		go func() {
			time.Sleep(3 * time.Second)
			m.message = ""
		}()
	} else {
		switch m.mode {
		case viewServers:
			enabled := 0
			for _, srv := range m.app.Canon.Servers {
				if srv.Enabled {
					enabled++
				}
			}
			status = fmt.Sprintf("%d/%d servers enabled | Press ? for help",
				enabled, len(m.app.Canon.Servers))
		case viewProfiles:
			status = fmt.Sprintf("%d profiles | Press enter to apply",
				len(m.app.Canon.Profiles))
		case viewHealth:
			if len(m.healthResults) > 0 {
				healthy := 0
				for _, r := range m.healthResults {
					if r.Status == health.StatusHealthy {
						healthy++
					}
				}
				status = fmt.Sprintf("%d/%d healthy | Press r to refresh",
					healthy, len(m.healthResults))
			} else {
				status = "Press h to run health checks"
			}
		case viewApply:
			status = "Press a to apply changes | Press r to refresh"
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