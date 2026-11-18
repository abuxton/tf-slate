package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/abuxton/tf-slate/internal/state"
)

type tuiScreen int

const (
	screenStates tuiScreen = iota
	screenActions
	screenResources
	screenInput
)

type inputKind int

const (
	inputAddress inputKind = iota
	inputDestroy
	inputDelete
	inputDeleteBackup
)

// tuiDeps isolates everything with side effects so the model can be tested
// through Update alone. Subprocesses stay in cmd/tf-slate.
type tuiDeps struct {
	terraformCmd func(dir string, args ...string) (*exec.Cmd, error)
	capture      func(dir string, args ...string) (string, error)
	checkSetup   func(dir string) error
	deleteFile   func(path string) error
	fileExists   func(path string) bool
	shellCmd     func(dir string) *exec.Cmd
}

func defaultTUIDeps() tuiDeps {
	return tuiDeps{
		terraformCmd: func(dir string, args ...string) (*exec.Cmd, error) {
			if _, err := exec.LookPath("terraform"); err != nil {
				return nil, errors.New("terraform executable not found; use the command shown on the menu manually")
			}
			cmd := exec.Command("terraform", args...)
			cmd.Dir = dir
			return cmd, nil
		},
		capture:    captureTerraform,
		checkSetup: checkTerraformSetup,
		deleteFile: os.Remove,
		fileExists: func(path string) bool { _, err := os.Stat(path); return err == nil },
		shellCmd: func(dir string) *exec.Cmd {
			shell, args := shellCommand()
			cmd := exec.Command(shell, args...)
			cmd.Dir = dir
			return cmd
		},
	}
}

type resourcesMsg struct {
	path      string
	resources []string
	err       error
}

type execDoneMsg struct {
	err  error
	quit bool
}

type tuiModel struct {
	deps      tuiDeps
	summaries []state.Summary
	screen    tuiScreen
	cursor    int
	action    int
	resources []string
	resCursor int
	resFrom   tuiScreen
	inKind    inputKind
	inValue   string
	status    string
	width     int
	height    int
	quitting  bool
}

type actionItem struct {
	value, label string
}

var stateActionItems = []actionItem{
	{"list", "List resources"},
	{"show", "Show a resource by address"},
	{"destroy", "Destroy resources"},
	{"visit", "Open a shell in the state directory"},
	{"delete", "Delete the state file"},
	{"back", "Back to the state list"},
	{"quit", "Quit"},
}

func newTUIModel(deps tuiDeps, summaries []state.Summary) tuiModel {
	return tuiModel{deps: deps, summaries: summaries, height: 24, width: 80}
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) current() state.Summary { return m.summaries[m.cursor] }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case resourcesMsg:
		return m.handleResources(msg), nil
	case execDoneMsg:
		if msg.quit {
			m.quitting = true
			return m, tea.Quit
		}
		if msg.err != nil {
			m.status = fmt.Sprintf("terraform command failed: %v", msg.err)
		} else {
			m.status = ""
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
		switch m.screen {
		case screenStates:
			return m.updateStates(msg)
		case screenActions:
			return m.updateActions(msg)
		case screenResources:
			return m.updateResources(msg)
		case screenInput:
			return m.updateInput(msg)
		}
	}
	return m, nil
}

func (m tuiModel) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	return m, tea.Quit
}

func (m tuiModel) updateStates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cursor = moveSelection(m.cursor, len(m.summaries), -1)
	case "down", "j":
		m.cursor = moveSelection(m.cursor, len(m.summaries), 1)
	case "enter":
		m.screen, m.action, m.status = screenActions, 0, ""
	case "q", "esc":
		return m.quit()
	}
	return m, nil
}

func (m tuiModel) updateActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.action = moveSelection(m.action, len(stateActionItems), -1)
	case "down", "j":
		m.action = moveSelection(m.action, len(stateActionItems), 1)
	case "esc", "b":
		m.screen, m.status = screenStates, ""
	case "q":
		return m.quit()
	case "enter":
		return m.runAction(stateActionItems[m.action].value)
	}
	return m, nil
}

func (m tuiModel) runAction(action string) (tea.Model, tea.Cmd) {
	summary := m.current()
	dir := filepath.Dir(summary.Path)
	m.status = ""
	switch action {
	case "list":
		return m, m.listCmd(summary.Path)
	case "show":
		m.screen, m.inKind, m.inValue = screenInput, inputAddress, ""
	case "destroy":
		m.screen, m.inKind, m.inValue = screenInput, inputDestroy, ""
	case "delete":
		m.screen, m.inKind, m.inValue = screenInput, inputDelete, ""
	case "visit":
		return m, tea.ExecProcess(m.deps.shellCmd(dir), func(err error) tea.Msg {
			if err != nil {
				return execDoneMsg{err: fmt.Errorf("visit failed: %w", err)}
			}
			return execDoneMsg{quit: true}
		})
	case "back":
		m.screen = screenStates
	case "quit":
		return m.quit()
	}
	return m, nil
}

func (m tuiModel) listCmd(path string) tea.Cmd {
	deps := m.deps
	return func() tea.Msg {
		out, err := deps.capture(filepath.Dir(path), stateArgs(path, "state", "list")...)
		if err != nil {
			if trimmed := strings.TrimSpace(out); trimmed != "" {
				err = fmt.Errorf("%w: %s", err, trimmed)
			}
			return resourcesMsg{path: path, err: err}
		}
		return resourcesMsg{path: path, resources: parseTerraformListOutput(out)}
	}
}

func (m tuiModel) handleResources(msg resourcesMsg) tuiModel {
	if msg.err != nil {
		m.status = fmt.Sprintf("terraform state list failed: %v", msg.err)
		return m
	}
	if len(msg.resources) == 0 {
		m.status = "No resources found in the selected state file"
		return m
	}
	m.screen, m.resources, m.resCursor, m.status = screenResources, msg.resources, 0, ""
	return m
}

func (m tuiModel) updateResources(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.resCursor = moveSelection(m.resCursor, len(m.resources), -1)
	case "down", "j":
		m.resCursor = moveSelection(m.resCursor, len(m.resources), 1)
	case "esc", "b":
		m.screen, m.status = screenActions, ""
	case "q":
		return m.quit()
	case "enter":
		return m.show(m.resources[m.resCursor], screenResources)
	}
	return m, nil
}

// show runs terraform state show with the terminal handed to terraform, then
// returns to the originating screen.
func (m tuiModel) show(address string, from tuiScreen) (tea.Model, tea.Cmd) {
	path := m.current().Path
	return m.execTerraform(from, stateArgs(path, "state", "show", address)...)
}

func (m tuiModel) execTerraform(back tuiScreen, args ...string) (tea.Model, tea.Cmd) {
	dir := filepath.Dir(m.current().Path)
	m.screen = back
	if err := m.deps.checkSetup(dir); err != nil {
		m.status = err.Error()
		return m, nil
	}
	cmd, err := m.deps.terraformCmd(dir, args...)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.status = ""
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return execDoneMsg{err: err} })
}

func (m tuiModel) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen, m.status = screenActions, "Cancelled"
		return m, nil
	case tea.KeyBackspace:
		if r := []rune(m.inValue); len(r) > 0 {
			m.inValue = string(r[:len(r)-1])
		}
		return m, nil
	case tea.KeyRunes, tea.KeySpace:
		m.inValue += string(msg.Runes)
		if msg.Type == tea.KeySpace {
			m.inValue += " "
		}
		return m, nil
	case tea.KeyEnter:
		return m.submitInput()
	}
	return m, nil
}

func (m tuiModel) submitInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.inValue)
	summary := m.current()
	m.inValue = ""
	switch m.inKind {
	case inputAddress:
		if value == "" {
			m.screen, m.status = screenActions, "No resource address entered"
			return m, nil
		}
		return m.show(value, screenActions)
	case inputDestroy:
		if value != "DESTROY" {
			m.screen, m.status = screenActions, "Destroy cancelled"
			return m, nil
		}
		return m.execTerraform(screenActions, stateArgs(summary.Path, "destroy")...)
	case inputDelete:
		if value != "DELETE" {
			m.screen, m.status = screenActions, "Delete cancelled"
			return m, nil
		}
		return m.deleteCurrent()
	case inputDeleteBackup:
		m.screen = screenStates
		if strings.EqualFold(value, "y") {
			backup := summary.Path + ".backup"
			if err := m.deps.deleteFile(backup); err != nil {
				m.status = fmt.Sprintf("delete failed: %v", err)
			} else {
				m.status = fmt.Sprintf("Deleted %s and %s", summary.Path, backup)
			}
		}
		return m.dropCurrent()
	}
	return m, nil
}

func (m tuiModel) deleteCurrent() (tea.Model, tea.Cmd) {
	summary := m.current()
	if err := m.deps.deleteFile(summary.Path); err != nil {
		m.screen, m.status = screenActions, fmt.Sprintf("delete failed: %v", err)
		return m, nil
	}
	if m.deps.fileExists(summary.Path + ".backup") {
		m.screen, m.inKind = screenInput, inputDeleteBackup
		return m, nil
	}
	m.status = fmt.Sprintf("Deleted %s", summary.Path)
	return m.dropCurrent()
}

// dropCurrent removes the selected entry after its file was deleted.
func (m tuiModel) dropCurrent() (tea.Model, tea.Cmd) {
	status := m.status
	m.summaries = append(m.summaries[:m.cursor:m.cursor], m.summaries[m.cursor+1:]...)
	if len(m.summaries) == 0 {
		m.quitting = true
		m.status = "No Terraform state files remain"
		return m, tea.Quit
	}
	m.cursor = min(m.cursor, len(m.summaries)-1)
	m.screen, m.status = screenStates, status
	return m, nil
}

var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleHelp   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleSelect = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// window returns the slice bounds that keep cursor visible within rows lines.
func window(total, cursor, rows int) (int, int) {
	rows = max(rows, 3)
	if total <= rows {
		return 0, total
	}
	start := min(max(cursor-rows/2, 0), total-rows)
	return start, start + rows
}

func (m tuiModel) listView(items []string, cursor int) string {
	start, end := window(len(items), cursor, m.height-10)
	var b strings.Builder
	for i := start; i < end; i++ {
		if i == cursor {
			b.WriteString(styleSelect.Render("› "+items[i]) + "\n")
		} else {
			b.WriteString("  " + items[i] + "\n")
		}
	}
	if len(items) > end-start {
		b.WriteString(styleDim.Render(fmt.Sprintf("  showing %d-%d of %d", start+1, end, len(items))) + "\n")
	}
	return b.String()
}

func (m tuiModel) View() string {
	if m.quitting {
		if m.status != "" {
			return m.status + "\n"
		}
		return ""
	}
	var b strings.Builder
	switch m.screen {
	case screenStates:
		b.WriteString(styleTitle.Render("Select a state file") + "\n")
		b.WriteString(styleHelp.Render("↑/↓ move · enter select · q quit") + "\n\n")
		items := make([]string, len(m.summaries))
		for i, s := range m.summaries {
			items[i] = fmt.Sprintf("%s  (%d resources)", s.Path, s.ResourceCount)
		}
		b.WriteString(m.listView(items, m.cursor))
		b.WriteString("\n" + m.summaryLine())
	case screenActions:
		b.WriteString(styleTitle.Render("State: "+m.current().Path) + "\n")
		b.WriteString(m.summaryLine() + "\n")
		b.WriteString(styleHelp.Render("↑/↓ move · enter run · esc back · q quit") + "\n\n")
		items := make([]string, len(stateActionItems))
		for i, a := range stateActionItems {
			items[i] = a.label
		}
		b.WriteString(m.listView(items, m.action))
		b.WriteString("\n" + styleDim.Render(m.describeAction(stateActionItems[m.action].value)) + "\n")
	case screenResources:
		b.WriteString(styleTitle.Render("State resources") + "\n")
		b.WriteString(styleHelp.Render("↑/↓ move · enter show · esc back · q quit") + "\n\n")
		b.WriteString(m.listView(m.resources, m.resCursor))
	case screenInput:
		b.WriteString(m.inputView())
	}
	if m.status != "" {
		b.WriteString("\n" + styleWarn.Render(m.status) + "\n")
	}
	return b.String()
}

func (m tuiModel) summaryLine() string {
	s := m.current()
	return styleDim.Render(fmt.Sprintf("%d resources | providers: %s | terraform: %s | serial: %d",
		s.ResourceCount, valueOrDash(strings.Join(s.Providers, ",")), valueOrDash(s.TerraformVersion), s.Serial))
}

func (m tuiModel) describeAction(action string) string {
	path := m.current().Path
	switch action {
	case "list":
		return describeTerraform(path, "state", "list")
	case "show":
		return describeTerraform(path, "state", "show", "<resource-address>")
	case "destroy":
		return describeTerraform(path, "destroy")
	case "visit":
		return fmt.Sprintf("Open a shell in %q and exit tf-slate", filepath.Dir(path))
	case "delete":
		return fmt.Sprintf("Permanently remove %q (does not destroy resources)", path)
	}
	return ""
}

func (m tuiModel) inputView() string {
	s := m.current()
	var prompt string
	switch m.inKind {
	case inputAddress:
		prompt = "Enter resource address:"
	case inputDestroy:
		prompt = styleWarn.Render("Destroy managed resources.") + "\nType DESTROY to confirm:"
	case inputDelete:
		warn := ""
		if s.ResourceCount > 0 {
			warn = styleWarn.Render(fmt.Sprintf("WARNING: this state tracks %d managed resource(s). Deleting it orphans them; run destroy first unless they are already gone.", s.ResourceCount)) + "\n"
		}
		prompt = warn + "This permanently deletes " + s.Path + "\nType DELETE to confirm:"
	case inputDeleteBackup:
		prompt = fmt.Sprintf("Deleted %s\nAlso delete %s.backup? [y/N]:", s.Path, s.Path)
	}
	return prompt + " " + m.inValue + "█\n" + styleHelp.Render("enter submit · esc cancel") + "\n"
}

// runBubbleTUI starts the Bubble Tea interface. It renders inline (no alternate
// screen) so terraform output remains in the scrollback.
func runBubbleTUI(summaries []state.Summary) error {
	_, err := tea.NewProgram(newTUIModel(defaultTUIDeps(), summaries)).Run()
	return err
}

// stdioIsTerminal reports whether both stdin and stdout are terminals, which the
// full-screen interface requires.
func stdioIsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
