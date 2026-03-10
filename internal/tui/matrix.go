package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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

type Tab int

const (
	MatrixTab Tab = iota
	ServersTab
	EditorTab
	NumTabs
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
	currentTab    Tab
	editor        textinput.Model
	editorContent string
	editorPath    string
	editorDirty   bool
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
	Tab1, Tab2, Tab3      key.Binding
	Save                  key.Binding
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
	Tab1:         key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "matrix tab")),
	Tab2:         key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "servers tab")),
	Tab3:         key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "editor tab")),
	Save:         key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("^s", "save")),
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

	te := textinput.New()
	te.CharLimit = 0

	m := &MatrixModel{
		app:        a,
		cursorRow:  0,
		cursorCol:  0,
		currentTab: MatrixTab,
		editor:     te,
	}
	m.detectClients()
	m.loadCanonicalForEditor()
	return m, nil
}

func (m *MatrixModel) loadCanonicalForEditor() {
	path, err := config.DefaultPath()
	if err != nil {
		m.editorPath = ""
		m.editorContent = ""
		return
	}
	m.editorPath = path
	content, err := os.ReadFile(path)
	if err != nil {
		m.editorContent = ""
	} else {
		m.editorContent = string(content)
	}
	m.editor.SetValue(m.editorContent)
	m.editorDirty = false
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
			if m.currentTab == MatrixTab || m.currentTab == ServersTab {
				if m.cursorRow > 0 {
					m.cursorRow--
				}
			}
			m.message = ""

		case key.Matches(msg, matrixKeys.Down):
			if m.currentTab == MatrixTab || m.currentTab == ServersTab {
				if m.cursorRow < len(m.app.Canon.Servers)-1 {
					m.cursorRow++
				}
			}
			m.message = ""

		case key.Matches(msg, matrixKeys.Left):
			if m.currentTab == MatrixTab && m.cursorCol > 0 {
				m.cursorCol--
			}
			m.message = ""

		case key.Matches(msg, matrixKeys.Right):
			if m.currentTab == MatrixTab {
				maxCol := m.countEnabledClients()
				if m.cursorCol < maxCol {
					m.cursorCol++
				}
			}
			m.message = ""

		case key.Matches(msg, matrixKeys.Toggle):
			if m.currentTab == MatrixTab || m.currentTab == ServersTab {
				m.toggleCurrent()
				m.pendingApply = true
			}

		case key.Matches(msg, matrixKeys.ToggleGlobal):
			if m.currentTab == MatrixTab {
				m.toggleGlobal()
				m.pendingApply = true
			}

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

		case key.Matches(msg, matrixKeys.Tab1):
			m.currentTab = MatrixTab
			m.message = ""

		case key.Matches(msg, matrixKeys.Tab2):
			m.currentTab = ServersTab
			m.cursorRow = 0
			m.message = ""

		case key.Matches(msg, matrixKeys.Tab3):
			m.currentTab = EditorTab
			m.cursorRow = 0
			m.loadCanonicalForEditor()
			m.message = ""

		case key.Matches(msg, matrixKeys.Save):
			if m.currentTab == EditorTab {
				m.saveEditor()
			}
		}
	}

	// Pass to editor if in editor tab
	if m.currentTab == EditorTab {
		// Check if editor content changed
		if m.editor.Value() != m.editorContent {
			m.editorDirty = true
		}
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
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

func (m *MatrixModel) saveEditor() {
	if m.editorPath == "" {
		m.message = "Error: no file path"
		return
	}

	content := m.editor.Value()
	if content == m.editorContent {
		m.message = "No changes to save"
		return
	}

	// Validate JSON before saving
	var testCanonical config.Canonical
	if err := json.Unmarshal([]byte(content), &testCanonical); err != nil {
		m.message = fmt.Sprintf("Invalid JSON: %v", err)
		return
	}

	err := os.WriteFile(m.editorPath, []byte(content), 0o644)
	if err != nil {
		m.message = fmt.Sprintf("Error saving: %v", err)
		return
	}

	m.editorContent = content
	m.editorDirty = false
	m.message = "Saved to " + m.editorPath

	// Reload app
	newApp, err := app.LoadApp()
	if err == nil {
		m.app = newApp
		m.detectClients()
	}
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
	// Render based on current tab
	switch m.currentTab {
	case MatrixTab:
		return m.renderMatrixView()
	case ServersTab:
		return m.renderServersView()
	case EditorTab:
		return m.renderEditorView()
	}
	return ""
}

func (m *MatrixModel) renderTabBar() string {
	tabs := []string{"[1] Matrix", "[2] Servers", "[3] Editor"}

	var result []string
	for i, tab := range tabs {
		if Tab(i) == m.currentTab {
			result = append(result, mxSelectedStyle.Padding(0, 1).Render(tab))
		} else {
			result = append(result, mxMutedTabStyle.Padding(0, 1).Render(tab))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, result...)
}

var mxMutedTabStyle = lipgloss.NewStyle().Foreground(mxMuted)

func (m *MatrixModel) renderMatrixView() string {
	var b strings.Builder

	// Title bar
	title := mxTitleStyle.Render("mseep")
	mode := m.app.Canon.EffectiveMode()
	modeBadge := lipgloss.NewStyle().Foreground(mxMuted).Render(fmt.Sprintf("mode: %s", mode))

	titleLine := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", modeBadge)
	b.WriteString(titleLine + "\n")

	// Tab bar
	b.WriteString(m.renderTabBar() + "\n\n")

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

func (m *MatrixModel) renderServersView() string {
	var b strings.Builder

	title := mxTitleStyle.Render("mseep")
	mode := m.app.Canon.EffectiveMode()
	modeBadge := lipgloss.NewStyle().Foreground(mxMuted).Render(fmt.Sprintf("mode: %s", mode))
	titleLine := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", modeBadge)
	b.WriteString(titleLine + "\n")
	b.WriteString(m.renderTabBar() + "\n\n")

	// Header
	header := mxHeaderStyle.Width(25).Render("Name")
	header += mxHeaderStyle.Width(50).Render("Description")
	header += mxHeaderStyle.Width(10).Align(lipgloss.Center).Render("Enabled")
	b.WriteString(header + "\n")

	sep := strings.Repeat("─", 85)
	b.WriteString(lipgloss.NewStyle().Foreground(mxMuted).Render(sep) + "\n")

	// Server rows
	for i, server := range m.app.Canon.Servers {
		isSelected := i == m.cursorRow

		name := server.Name
		if len(name) > 23 {
			name = name[:20] + "..."
		}
		desc := server.Description
		if len(desc) > 48 {
			desc = desc[:45] + "..."
		}

		nameStyle := mxCellStyle.Width(25)
		descStyle := mxCellStyle.Width(50)
		enabledStyle := mxCellStyle.Width(10).Align(lipgloss.Center)

		if isSelected {
			nameStyle = mxSelectedStyle.Width(25)
			descStyle = mxSelectedStyle.Width(50)
			enabledStyle = mxSelectedStyle.Width(10).Align(lipgloss.Center)
		}

		row := nameStyle.Render(name)
		row += descStyle.Render(desc)

		if server.Enabled {
			if isSelected {
				row += mxEnabledStyle.Copy().Background(mxBgLight).Width(10).Align(lipgloss.Center).Render("●")
			} else {
				row += mxEnabledStyle.Width(10).Align(lipgloss.Center).Render("●")
			}
		} else {
			row += enabledStyle.Render("○")
		}

		b.WriteString(row + "\n")
	}

	// Message
	if m.message != "" {
		b.WriteString("\n" + mxMessageStyle.Render(m.message) + "\n")
	}

	// Status bar
	b.WriteString("\n")
	enabled := 0
	for _, s := range m.app.Canon.Servers {
		if s.Enabled {
			enabled++
		}
	}
	parts := []string{
		fmt.Sprintf("%d/%d enabled", enabled, len(m.app.Canon.Servers)),
		"↑↓:navigate", "space:toggle", "q:quit",
	}
	b.WriteString(mxStatusStyle.Width(m.width).Render(strings.Join(parts, "  ·  ")))

	return b.String()
}

func (m *MatrixModel) renderEditorView() string {
	var b strings.Builder

	title := mxTitleStyle.Render("mseep")
	pathInfo := m.editorPath
	if pathInfo == "" {
		pathInfo = "no file"
	}
	pathBadge := lipgloss.NewStyle().Foreground(mxMuted).Render(pathInfo)

	if m.editorDirty {
		pathBadge += lipgloss.NewStyle().Foreground(mxWarning).Render(" (modified)")
	}

	titleLine := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", pathBadge)
	b.WriteString(titleLine + "\n")
	b.WriteString(m.renderTabBar() + "\n\n")

	// Editor content
	editorView := m.editor.View()
	lines := strings.Split(editorView, "\n")

	// Calculate available width for editor
	editorWidth := m.width - 4
	if editorWidth < 40 {
		editorWidth = 40
	}

	// Render each line with a line number gutter
	for i, line := range lines {
		lineNum := fmt.Sprintf("%3d ", i+1)
		lineNumStyle := lipgloss.NewStyle().Foreground(mxMuted)
		b.WriteString(lineNumStyle.Render(lineNum))
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Message
	if m.message != "" {
		b.WriteString("\n" + mxMessageStyle.Render(m.message) + "\n")
	}

	// Status bar
	parts := []string{
		"Editor",
		"↑↓: navigate",
		"^s: save",
	}
	if m.editorDirty {
		parts = append(parts, mxWarningStyle.Render("unsaved"))
	}
	parts = append(parts, "q:quit")
	b.WriteString("\n" + mxStatusStyle.Width(m.width).Render(strings.Join(parts, "  ·  ")))

	return b.String()
}
