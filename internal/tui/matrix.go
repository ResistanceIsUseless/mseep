package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ResistanceIsUseless/mseep/internal/adapters/claude"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/cline"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/cursor"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/opencode"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/vscode"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/warp"
	"github.com/ResistanceIsUseless/mseep/internal/app"
	"github.com/ResistanceIsUseless/mseep/internal/config"
)

// MatrixModel is a simpler TUI focused on the server×client matrix
type MatrixModel struct {
	app           *app.App
	clients       []clientInfo
	width, height int
	cursorRow     int // server index
	cursorCol     int // client index (0 = global, 1+ = clients)
	message       string
	loading       bool
	pendingApply  bool
}

type clientInfo struct {
	name      string
	detected  bool
	enabled   bool
	shortName string
}

// Matrix key bindings
type matrixKeyMap struct {
	Up, Down, Left, Right key.Binding
	Toggle                key.Binding
	ToggleGlobal          key.Binding
	ToggleClient          key.Binding
	ToggleRow             key.Binding
	ToggleCol             key.Binding
	Apply                 key.Binding
	Refresh               key.Binding
	Quit                  key.Binding
	Help                  key.Binding
}

var matrixKeys = matrixKeyMap{
	Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:         key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
	Right:        key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
	Toggle:       key.NewBinding(key.WithKeys(" ", "enter"), key.WithHelp("space", "toggle")),
	ToggleGlobal: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "toggle global")),
	ToggleClient: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "toggle client")),
	ToggleRow:    key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "toggle row (all clients)")),
	ToggleCol:    key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "toggle column (all servers)")),
	Apply:        key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "apply")),
	Refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

// Colors
var (
	mxPrimary  = lipgloss.Color("#00D9FF")
	mxSuccess  = lipgloss.Color("#50FA7B")
	mxWarning  = lipgloss.Color("#FFB86C")
	mxError    = lipgloss.Color("#FF5555")
	mxMuted    = lipgloss.Color("#6272A4")
	mxBg       = lipgloss.Color("#282A36")
	mxBgLight  = lipgloss.Color("#44475A")
	mxFg       = lipgloss.Color("#F8F8F2")
	mxAccent   = lipgloss.Color("#FF79C6")
	mxOverride = lipgloss.Color("#BD93F9") // purple for overrides
)

// Styles
var (
	mxTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(mxPrimary).
			Padding(0, 1)

	mxHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(mxFg).
			Background(mxBgLight).
			Padding(0, 1)

	mxCellStyle = lipgloss.NewStyle().
			Foreground(mxMuted).
			Padding(0, 1)

	mxSelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(mxFg).
			Background(mxBgLight)

	mxEnabledStyle = lipgloss.NewStyle().
			Foreground(mxSuccess).
			Bold(true)

	mxDisabledStyle = lipgloss.NewStyle().
			Foreground(mxMuted)

	mxOverrideStyle = lipgloss.NewStyle().
			Foreground(mxOverride).
			Bold(true)

	mxStatusStyle = lipgloss.NewStyle().
			Foreground(mxFg).
			Background(mxBg).
			Padding(0, 1)

	mxMessageStyle = lipgloss.NewStyle().
			Foreground(mxWarning)
)

func NewMatrix() (*MatrixModel, error) {
	a, err := app.LoadApp()
	if err != nil {
		return nil, err
	}

	m := &MatrixModel{
		app:       a,
		cursorRow: 0,
		cursorCol: 0,
	}
	m.detectClients()
	return m, nil
}

func (m *MatrixModel) detectClients() {
	adapters := []struct {
		name, short string
		adapter     interface{ Detect() (bool, error) }
	}{
		{"claude", "Claude", claude.Adapter{}},
		{"cursor", "Cursor", cursor.Adapter{}},
		{"vscode", "VSCode", vscode.Adapter{}},
		{"warp", "Warp", warp.Adapter{}},
		{"opencode", "OCode", opencode.Adapter{}},
		{"cline", "Cline", cline.Adapter{}},
	}

	m.clients = nil
	for _, a := range adapters {
		detected, _ := a.adapter.Detect()
		enabled := m.app.Canon.IsClientEnabled(a.name)
		m.clients = append(m.clients, clientInfo{
			name:      a.name,
			shortName: a.short,
			detected:  detected,
			enabled:   enabled && detected,
		})
	}
}

func (m *MatrixModel) getClientStatus() []clientInfo {
	return m.clients
}

func (m *MatrixModel) Init() tea.Cmd {
	return m.refresh()
}

func (m *MatrixModel) refresh() tea.Cmd {
	return func() tea.Msg {
		newApp, err := app.LoadApp()
		if err != nil {
			return errorMsg{err}
		}
		return refreshedMsg{app: newApp}
	}
}

type refreshedMsg struct{ app *app.App }
type appliedMsg struct{ err error }
type errorMsg struct{ err error }

func (m *MatrixModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case refreshedMsg:
		m.app = msg.app
		m.detectClients()
		m.loading = false
		return m, nil

	case appliedMsg:
		m.loading = false
		if msg.err != nil {
			m.message = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.message = "Applied to all clients"
			m.pendingApply = false
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		switch {
		case key.Matches(msg, matrixKeys.Quit):
			return m, tea.Quit

		case key.Matches(msg, matrixKeys.Up):
			if m.cursorRow > 0 {
				m.cursorRow--
			}
			m.message = ""

		case key.Matches(msg, matrixKeys.Down):
			if m.cursorRow < len(m.app.Canon.Servers)-1 {
				m.cursorRow++
			}
			m.message = ""

		case key.Matches(msg, matrixKeys.Left):
			if m.cursorCol > 0 {
				m.cursorCol--
			}
			m.message = ""

		case key.Matches(msg, matrixKeys.Right):
			maxCol := m.countEnabledClients()
			if m.cursorCol < maxCol {
				m.cursorCol++
			}
			m.message = ""

		case key.Matches(msg, matrixKeys.Toggle):
			m.toggleCurrent()
			m.pendingApply = true

		case key.Matches(msg, matrixKeys.ToggleGlobal):
			m.toggleGlobal()
			m.pendingApply = true

		case key.Matches(msg, matrixKeys.ToggleClient):
			m.toggleClientEnabled()
			m.pendingApply = true

		case key.Matches(msg, matrixKeys.ToggleRow):
			m.toggleRow()
			m.pendingApply = true

		case key.Matches(msg, matrixKeys.ToggleCol):
			m.toggleColumn()
			m.pendingApply = true

		case key.Matches(msg, matrixKeys.Apply):
			return m, m.apply()

		case key.Matches(msg, matrixKeys.Refresh):
			m.loading = true
			return m, m.refresh()
		}
	}

	return m, nil
}

func (m *MatrixModel) countEnabledClients() int {
	count := 0
	for _, c := range m.clients {
		if c.enabled {
			count++
		}
	}
	return count
}

func (m *MatrixModel) getEnabledClients() []clientInfo {
	var result []clientInfo
	for _, c := range m.clients {
		if c.enabled {
			result = append(result, c)
		}
	}
	return result
}

func (m *MatrixModel) toggleCurrent() {
	if m.cursorRow >= len(m.app.Canon.Servers) {
		return
	}

	server := &m.app.Canon.Servers[m.cursorRow]

	if m.cursorCol == 0 {
		// Toggle global
		server.Enabled = !server.Enabled
		m.message = fmt.Sprintf("%s globally %s", server.Name, m.enabledStr(server.Enabled))
	} else {
		// Toggle per-client
		enabledClients := m.getEnabledClients()
		if m.cursorCol-1 < len(enabledClients) {
			client := enabledClients[m.cursorCol-1]
			currentState := m.app.Canon.IsServerEnabledForClient(server.Name, client.name)
			m.app.Canon.SetServerOverride(server.Name, client.name, !currentState)
			m.message = fmt.Sprintf("%s → %s: %s", server.Name, client.shortName, m.enabledStr(!currentState))
		}
	}

	_ = config.Save("", m.app.Canon)
}

func (m *MatrixModel) toggleGlobal() {
	if m.cursorRow >= len(m.app.Canon.Servers) {
		return
	}
	server := &m.app.Canon.Servers[m.cursorRow]
	server.Enabled = !server.Enabled
	m.message = fmt.Sprintf("%s globally %s", server.Name, m.enabledStr(server.Enabled))
	_ = config.Save("", m.app.Canon)
}

func (m *MatrixModel) toggleClientEnabled() {
	if m.cursorCol == 0 {
		return // Can't disable "Global" column
	}
	enabledClients := m.getEnabledClients()
	if m.cursorCol-1 >= len(enabledClients) {
		return
	}
	client := enabledClients[m.cursorCol-1]

	// Find the actual client and toggle
	for i := range m.clients {
		if m.clients[i].name == client.name {
			m.clients[i].enabled = !m.clients[i].enabled
			m.app.Canon.SetClientEnabled(client.name, m.clients[i].enabled)
			m.message = fmt.Sprintf("%s client %s", client.shortName, m.enabledStr(m.clients[i].enabled))
			break
		}
	}
	_ = config.Save("", m.app.Canon)
}

// toggleRow enables/disables the current server for ALL enabled clients
func (m *MatrixModel) toggleRow() {
	if m.cursorRow >= len(m.app.Canon.Servers) {
		return
	}
	server := &m.app.Canon.Servers[m.cursorRow]
	enabledClients := m.getEnabledClients()

	// Determine target state: if server is globally enabled, disable for all; otherwise enable for all
	newState := !server.Enabled

	// Set global state
	server.Enabled = newState

	// Clear all per-client overrides for this server (so they follow global)
	for _, client := range enabledClients {
		if m.app.Canon.Clients != nil {
			if c, exists := m.app.Canon.Clients[client.name]; exists {
				delete(c.Overrides, server.Name)
				m.app.Canon.Clients[client.name] = c
			}
		}
	}

	m.message = fmt.Sprintf("%s %s for all clients", server.Name, m.enabledStr(newState))
	_ = config.Save("", m.app.Canon)
}

// toggleColumn enables/disables ALL servers for the current client
func (m *MatrixModel) toggleColumn() {
	if m.cursorCol == 0 {
		// Column 0 is global - toggle all servers globally
		m.toggleAllServersGlobal()
		return
	}

	enabledClients := m.getEnabledClients()
	if m.cursorCol-1 >= len(enabledClients) {
		return
	}
	client := enabledClients[m.cursorCol-1]

	// Count how many are currently enabled for this client
	enabledCount := 0
	for _, server := range m.app.Canon.Servers {
		if m.app.Canon.IsServerEnabledForClient(server.Name, client.name) {
			enabledCount++
		}
	}

	// If more than half are enabled, disable all; otherwise enable all
	newState := enabledCount <= len(m.app.Canon.Servers)/2

	for _, server := range m.app.Canon.Servers {
		m.app.Canon.SetServerOverride(server.Name, client.name, newState)
	}

	m.message = fmt.Sprintf("All servers %s for %s", m.enabledStr(newState), client.shortName)
	_ = config.Save("", m.app.Canon)
}

// toggleAllServersGlobal toggles all servers globally
func (m *MatrixModel) toggleAllServersGlobal() {
	enabledCount := 0
	for _, server := range m.app.Canon.Servers {
		if server.Enabled {
			enabledCount++
		}
	}

	// If more than half are enabled, disable all; otherwise enable all
	newState := enabledCount <= len(m.app.Canon.Servers)/2

	for i := range m.app.Canon.Servers {
		m.app.Canon.Servers[i].Enabled = newState
	}

	m.message = fmt.Sprintf("All servers globally %s", m.enabledStr(newState))
	_ = config.Save("", m.app.Canon)
}

func (m *MatrixModel) enabledStr(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func (m *MatrixModel) apply() tea.Cmd {
	m.loading = true
	m.message = "Applying..."
	return func() tea.Msg {
		err := m.app.ApplySilent("", "")
		return appliedMsg{err: err}
	}
}

func (m *MatrixModel) View() string {
	var b strings.Builder

	// Title bar
	title := mxTitleStyle.Render("mseep")
	mode := m.app.Canon.EffectiveMode()
	modeBadge := lipgloss.NewStyle().Foreground(mxMuted).Render(fmt.Sprintf("mode: %s", mode))

	// Profile hints
	profileHints := ""
	i := 1
	for name := range m.app.Canon.Profiles {
		if i <= 3 {
			profileHints += fmt.Sprintf(" %d:%s", i, name)
			i++
		}
	}
	if profileHints != "" {
		profileHints = lipgloss.NewStyle().Foreground(mxMuted).Render(profileHints)
	}

	titleLine := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", modeBadge, profileHints)
	b.WriteString(titleLine + "\n\n")

	// Build the matrix
	enabledClients := m.getEnabledClients()

	// Calculate column widths
	serverColWidth := 20
	for _, s := range m.app.Canon.Servers {
		if len(s.Name) > serverColWidth-2 {
			serverColWidth = len(s.Name) + 2
		}
	}
	if serverColWidth > 25 {
		serverColWidth = 25
	}
	cellWidth := 8

	// Header row
	header := mxHeaderStyle.Width(serverColWidth).Render("Server")
	header += mxHeaderStyle.Width(cellWidth).Align(lipgloss.Center).Render("All")
	for _, c := range enabledClients {
		header += mxHeaderStyle.Width(cellWidth).Align(lipgloss.Center).Render(c.shortName)
	}
	b.WriteString(header + "\n")

	// Separator
	sep := strings.Repeat("─", serverColWidth+cellWidth*(len(enabledClients)+1))
	b.WriteString(lipgloss.NewStyle().Foreground(mxMuted).Render(sep) + "\n")

	// Server rows
	for i, server := range m.app.Canon.Servers {
		row := m.renderServerRow(i, server, enabledClients, serverColWidth, cellWidth)
		b.WriteString(row + "\n")
	}

	// Message
	if m.message != "" {
		b.WriteString("\n" + mxMessageStyle.Render(m.message) + "\n")
	}

	// Status bar
	b.WriteString("\n")
	status := m.renderStatusBar()
	b.WriteString(status)

	return b.String()
}

func (m *MatrixModel) renderServerRow(idx int, server config.Server, clients []clientInfo, nameWidth, cellWidth int) string {
	isSelectedRow := idx == m.cursorRow

	// Server name
	name := server.Name
	if len(name) > nameWidth-2 {
		name = name[:nameWidth-5] + "..."
	}

	nameStyle := mxCellStyle.Width(nameWidth)
	if isSelectedRow && m.cursorCol == 0 {
		nameStyle = mxSelectedStyle.Width(nameWidth)
	}
	row := nameStyle.Render(name)

	// Global toggle (column 0)
	globalCell := m.renderCell(server.Enabled, false, isSelectedRow && m.cursorCol == 0, cellWidth)
	row += globalCell

	// Per-client cells
	for ci, client := range clients {
		isOverride := m.hasOverride(server.Name, client.name)
		enabled := m.app.Canon.IsServerEnabledForClient(server.Name, client.name)
		isSelected := isSelectedRow && m.cursorCol == ci+1
		row += m.renderCell(enabled, isOverride, isSelected, cellWidth)
	}

	return row
}

func (m *MatrixModel) renderCell(enabled, isOverride, isSelected bool, width int) string {
	var symbol string
	var style lipgloss.Style

	if enabled {
		symbol = "[●]"
		if isOverride {
			style = mxOverrideStyle
		} else {
			style = mxEnabledStyle
		}
	} else {
		symbol = "[ ]"
		if isOverride {
			style = mxOverrideStyle
		} else {
			style = mxDisabledStyle
		}
	}

	if isSelected {
		style = style.Copy().Background(mxBgLight).Bold(true)
	}

	return style.Width(width).Align(lipgloss.Center).Render(symbol)
}

func (m *MatrixModel) hasOverride(serverName, clientName string) bool {
	if m.app.Canon.Clients == nil {
		return false
	}
	client, exists := m.app.Canon.Clients[clientName]
	if !exists {
		return false
	}
	_, hasOverride := client.Overrides[serverName]
	return hasOverride
}

func (m *MatrixModel) renderStatusBar() string {
	enabled := 0
	for _, s := range m.app.Canon.Servers {
		if s.Enabled {
			enabled++
		}
	}

	parts := []string{
		fmt.Sprintf("%d/%d enabled", enabled, len(m.app.Canon.Servers)),
	}

	if m.pendingApply {
		parts = append(parts, mxWarningStyle.Render("unsaved"))
	}

	parts = append(parts, "space:toggle", "R:row", "C:col", "a:apply", "q:quit")

	return mxStatusStyle.Width(m.width).Render(strings.Join(parts, "  ·  "))
}

var mxWarningStyle = lipgloss.NewStyle().Foreground(mxWarning).Bold(true)
