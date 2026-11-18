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
	})
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
	m, _ = send(m, append(typeText("DELETE"), key("enter"))...)
	if len(deleted) != 1 || len(m.summaries) != 1 || m.screen != screenStates {
		t.Fatalf("deleted=%v summaries=%d screen=%v", deleted, len(m.summaries), m.screen)
	}
}

func TestTUIDeleteOffersBackupAndQuitsWhenEmpty(t *testing.T) {
	var deleted []string
	m := testTUIModel(&deleted)
	m.summaries = m.summaries[:1]
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
