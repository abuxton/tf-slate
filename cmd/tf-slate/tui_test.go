package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/abuxton/tf-slate/internal/state"
)

func testTUIModel(deleted *[]string) tuiModel {
	deps := tuiDeps{
		terraformCmd: func(dir string, args ...string) (*exec.Cmd, error) { return exec.Command("true"), nil },
		capture:      func(dir string, args ...string) (string, error) { return "a.b\nc.d\n", nil },
		checkSetup:   func(dir string) error { return nil },
		deleteFile:   func(p string) error { *deleted = append(*deleted, p); return nil },
		fileExists:   func(string) bool { return false },
		shellCmd:     func(dir string) *exec.Cmd { return exec.Command("true") },
	}
	return newTUIModel(deps, []state.Summary{
		{Path: "/tmp/a/terraform.tfstate", ResourceCount: 2, Providers: []string{"aws"}},
		{Path: "/tmp/b/terraform.tfstate", ResourceCount: 0},
	}, false, false)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func send(m tuiModel, msgs ...tea.Msg) (tuiModel, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(tuiModel)
	}
	return m, cmd
}

func typeText(text string) []tea.Msg {
	msgs := []tea.Msg{}
	for _, r := range text {
		msgs = append(msgs, key(string(r)))
	}
	return msgs
}

func selectAction(m tuiModel, value string) tuiModel {
	m, _ = send(m, key("enter"))
	for i, a := range stateActionItems {
		if a.value == value {
			for j := 0; j < i; j++ {
				m, _ = send(m, key("down"))
			}
		}
	}
	return m
}

func TestTUINavigatesStatesAndActions(t *testing.T) {
	var deleted []string
	m, _ := send(testTUIModel(&deleted), key("down"), key("enter"))
	if m.screen != screenActions || m.cursor != 1 {
		t.Fatalf("screen=%v cursor=%d, want actions on second state", m.screen, m.cursor)
	}
	m, _ = send(m, key("esc"))
	if m.screen != screenStates {
		t.Fatalf("esc should return to state list, got %v", m.screen)
	}
	_, cmd := send(m, key("q"))
	if cmd == nil {
		t.Fatal("q should quit")
	}
}

func TestTUIListThenShowReturnsToResources(t *testing.T) {
	var deleted []string
	m := selectAction(testTUIModel(&deleted), "list")
	m, cmd := send(m, key("enter"))
	if cmd == nil {
		t.Fatal("list should return a command")
	}
	m, _ = send(m, cmd())
	if m.screen != screenResources || len(m.resources) != 2 {
		t.Fatalf("screen=%v resources=%v", m.screen, m.resources)
	}
	m, cmd = send(m, key("enter"))
	if cmd == nil || m.screen != screenResources {
		t.Fatalf("show should exec and stay on resources (screen=%v)", m.screen)
	}
}

func TestTUIListResultIsIgnoredAfterSelectingAnotherState(t *testing.T) {
	var deleted []string
	m := selectAction(testTUIModel(&deleted), "list")
	m, cmd := send(m, key("enter"))
	if cmd == nil {
		t.Fatal("list should return a command")
	}
	m, _ = send(m, key("esc"), key("down"), key("enter"))
	m, _ = send(m, cmd())
	if m.screen != screenActions || m.cursor != 1 || m.status != "" {
		t.Fatalf("stale result changed selected state: screen=%v cursor=%d status=%q", m.screen, m.cursor, m.status)
	}
}

func TestTUIDestroyRequiresExactConfirmation(t *testing.T) {
	var deleted []string
	m := selectAction(testTUIModel(&deleted), "destroy")
	m, _ = send(m, key("enter"))
	m, cmd := send(m, append(typeText("destroy"), key("enter"))...)
	if cmd != nil || m.status != "Destroy cancelled" {
		t.Fatalf("lowercase must cancel; status=%q cmd=%v", m.status, cmd)
	}
	m, _ = send(m, key("enter"))
	m, cmd = send(m, append(typeText("DESTROY"), key("enter"))...)
	if cmd == nil {
		t.Fatal("DESTROY should run terraform")
	}
}

func TestTUIGuardsUninitializedSetup(t *testing.T) {
	var deleted []string
	m := testTUIModel(&deleted)
	m.deps.checkSetup = func(string) error { return errors.New("not initialized") }
	m = selectAction(m, "destroy")
	m, _ = send(m, key("enter"))
	m, cmd := send(m, append(typeText("DESTROY"), key("enter"))...)
	if cmd != nil || m.status != "not initialized" {
		t.Fatalf("status=%q cmd=%v", m.status, cmd)
	}
}

func TestTUIDeleteRequiresDELETEAndDropsEntry(t *testing.T) {
	var deleted []string
	m := selectAction(testTUIModel(&deleted), "delete")
	m, _ = send(m, key("enter"))
	m, _ = send(m, append(typeText("delete"), key("enter"))...)
	if len(deleted) != 0 || m.status != "Delete cancelled" {
		t.Fatalf("deleted=%v status=%q", deleted, m.status)
	}
	m, _ = send(m, key("enter"))
	m, _ = send(m, append(typeText(" DELETE "), key("enter"))...)
	if len(deleted) != 0 || m.status != "Delete cancelled" {
		t.Fatalf("surrounding whitespace must cancel; deleted=%v status=%q", deleted, m.status)
	}
	m, _ = send(m, key("enter"))
	m, _ = send(m, append(typeText("DELETE"), key("enter"))...)
	if len(deleted) != 1 || len(m.summaries) != 1 || m.screen != screenStates {
		t.Fatalf("deleted=%v summaries=%d screen=%v", deleted, len(m.summaries), m.screen)
	}
}

func TestTUIDeleteBackupEscapeDropsDeletedState(t *testing.T) {
	var deleted []string
	m := testTUIModel(&deleted)
	m.deps.fileExists = func(string) bool { return true }
	m = selectAction(m, "delete")
	m, _ = send(m, key("enter"))
	m, _ = send(m, append(typeText("DELETE"), key("enter"))...)
	m, _ = send(m, key("esc"))
	if len(deleted) != 1 || len(m.summaries) != 1 || m.screen != screenStates {
		t.Fatalf("deleted=%v summaries=%d screen=%v", deleted, len(m.summaries), m.screen)
	}
	if strings.HasSuffix(deleted[0], ".backup") {
		t.Fatalf("escape should decline backup deletion: %v", deleted)
	}
}

func TestTUIDeleteBackupErrorIsShownWhenLastStateIsRemoved(t *testing.T) {
	var deleted []string
	m := testTUIModel(&deleted)
	m.summaries = m.summaries[:1]
	m.all = m.summaries
	m.deps.fileExists = func(string) bool { return true }
	m.deps.deleteFile = func(path string) error {
		deleted = append(deleted, path)
		if strings.HasSuffix(path, ".backup") {
			return errors.New("permission denied")
		}
		return nil
	}
	m = selectAction(m, "delete")
	m, _ = send(m, key("enter"))
	m, _ = send(m, append(typeText("DELETE"), key("enter"))...)
	m, _ = send(m, append(typeText("y"), key("enter"))...)
	if !m.quitting || !strings.Contains(m.status, "delete failed: permission denied") ||
		!strings.Contains(m.status, "No Terraform state files remain") {
		t.Fatalf("quitting=%v status=%q", m.quitting, m.status)
	}
}

func TestTUIInputAppendsSpaceOnce(t *testing.T) {
	var deleted []string
	m := testTUIModel(&deleted)
	m.screen, m.inKind = screenInput, inputAddress
	m, _ = send(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if m.inValue != " " {
		t.Fatalf("input=%q, want one space", m.inValue)
	}
}

func TestTUIDeleteOffersBackupAndQuitsWhenEmpty(t *testing.T) {
	var deleted []string
	m := testTUIModel(&deleted)
	m.summaries = m.summaries[:1]
	m.all = m.summaries
	m.deps.fileExists = func(string) bool { return true }
	m = selectAction(m, "delete")
	m, _ = send(m, key("enter"))
	m, _ = send(m, append(typeText("DELETE"), key("enter"))...)
	if m.screen != screenInput || m.inKind != inputDeleteBackup {
		t.Fatalf("expected backup prompt, screen=%v", m.screen)
	}
	m, cmd := send(m, key("y"), key("enter"))
	if len(deleted) != 2 || !strings.HasSuffix(deleted[1], ".backup") {
		t.Fatalf("deleted=%v", deleted)
	}
	if cmd == nil || !m.quitting {
		t.Fatal("should quit when no states remain")
	}
}

func TestTUIVisitQuitsAfterShell(t *testing.T) {
	var deleted []string
	m := selectAction(testTUIModel(&deleted), "visit")
	_, cmd := send(m, key("enter"))
	if cmd == nil {
		t.Fatal("visit should exec a shell")
	}
	m, _ = send(m, execDoneMsg{quit: true})
	if !m.quitting {
		t.Fatal("visit should end the session")
	}
}

func TestWindowKeepsCursorVisible(t *testing.T) {
	for cursor := 0; cursor < 50; cursor++ {
		start, end := window(50, cursor, 10)
		if cursor < start || cursor >= end || end-start != 10 {
			t.Fatalf("cursor %d outside [%d,%d)", cursor, start, end)
		}
	}
}

func TestTUIViewKeepsStateSuffixAndWrapsSelectedPath(t *testing.T) {
	var deleted []string
	m := testTUIModel(&deleted)
	path := "/very/long/shared/root/that/exceeds/the/terminal/width/alpha.tfstate"
	m.summaries = []state.Summary{
		{Path: path, ResourceCount: 2},
		{Path: strings.TrimSuffix(path, "alpha.tfstate") + "beta.tfstate"},
	}
	m.width = 36
	if view := m.View(); !strings.Contains(view, "alpha.tfstate") || !strings.Contains(view, "beta.tfstate") {
		t.Fatalf("state picker does not retain distinguishing suffixes: %q", view)
	}
	m.screen = screenActions
	view := strings.ReplaceAll(m.View(), "\n", "")
	if !strings.Contains(view, path) {
		t.Fatalf("selected path is not fully readable: %q", view)
	}
}

func TestTUIHidesAndTogglesZeroResourceStates(t *testing.T) {
	var deleted []string
	base := testTUIModel(&deleted)
	m := newTUIModel(base.deps, []state.Summary{
		{Path: "/tmp/empty/terraform.tfstate", ResourceCount: 0},
		{Path: "/tmp/few/terraform.tfstate", ResourceCount: 1},
		{Path: "/tmp/many/terraform.tfstate", ResourceCount: 5},
	}, true, true)
	if len(m.summaries) != 2 || m.summaries[0].Path != "/tmp/many/terraform.tfstate" {
		t.Fatalf("default view should be non-zero and weighted, got %v", m.summaries)
	}
	m, _ = send(m, key("down"), key("z"))
	if len(m.summaries) != 3 || m.summaries[2].ResourceCount != 0 {
		t.Fatalf("z should reveal empty states, got %v", m.summaries)
	}
	if m.current().Path != "/tmp/few/terraform.tfstate" {
		t.Fatalf("selection should be kept across toggle, got %s", m.current().Path)
	}
	m, _ = send(m, key("z"))
	if len(m.summaries) != 2 {
		t.Fatalf("z again should hide empty states, got %v", m.summaries)
	}
}

func TestTUIDeleteKeepsHiddenStates(t *testing.T) {
	var deleted []string
	base := testTUIModel(&deleted)
	m := newTUIModel(base.deps, []state.Summary{
		{Path: "/tmp/empty/terraform.tfstate", ResourceCount: 0},
		{Path: "/tmp/one/terraform.tfstate", ResourceCount: 1},
	}, true, true)
	m = selectAction(m, "delete")
	m, _ = send(m, key("enter"))
	m, cmd := send(m, append(typeText("DELETE"), key("enter"))...)
	if cmd != nil || len(m.summaries) != 1 || m.summaries[0].ResourceCount != 0 || m.hideZero {
		t.Fatalf("remaining empty state should be shown; summaries=%v hideZero=%v", m.summaries, m.hideZero)
	}
}
