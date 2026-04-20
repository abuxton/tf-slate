package main

import (
	"bufio"
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abuxton/tf-slate/internal/state"
)

func TestCountResourcePaths(t *testing.T) {
	summaries := []state.Summary{
		{Path: "a.tfstate", ResourceCount: 0},
		{Path: "b.tfstate", ResourceCount: 2},
		{Path: "c.tfstate", ResourceCount: 1},
		{Path: "d.tfstate", ResourceCount: 0},
	}

	zero, nonZero := countResourcePaths(summaries)
	if zero != 2 || nonZero != 2 {
		t.Fatalf("countResourcePaths() = (%d, %d), want (2, 2)", zero, nonZero)
	}
}

func TestFilterNonZeroSummaries(t *testing.T) {
	summaries := []state.Summary{
		{Path: "a.tfstate", ResourceCount: 0},
		{Path: "b.tfstate", ResourceCount: 3},
		{Path: "c.tfstate", ResourceCount: 1},
	}

	got := filterNonZeroSummaries(summaries)
	want := []state.Summary{
		{Path: "b.tfstate", ResourceCount: 3},
		{Path: "c.tfstate", ResourceCount: 1},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterNonZeroSummaries() = %#v, want %#v", got, want)
	}
}

func TestSortSummariesWeighted(t *testing.T) {
	summaries := []state.Summary{
		{Path: "c.tfstate", ResourceCount: 2},
		{Path: "a.tfstate", ResourceCount: 5},
		{Path: "b.tfstate", ResourceCount: 2},
	}

	sortSummariesWeighted(summaries)

	want := []state.Summary{
		{Path: "a.tfstate", ResourceCount: 5},
		{Path: "b.tfstate", ResourceCount: 2},
		{Path: "c.tfstate", ResourceCount: 2},
	}

	if !reflect.DeepEqual(summaries, want) {
		t.Fatalf("sortSummariesWeighted() = %#v, want %#v", summaries, want)
	}
}

func TestPrintSummaryTable(t *testing.T) {
	summaries := []state.Summary{
		{Path: "a.tfstate", ResourceCount: 0},
		{Path: "b.tfstate", ResourceCount: 4},
	}

	var buf bytes.Buffer
	printSummaryTable(&buf, summaries)

	output := buf.String()
	for _, want := range []string{
		"State resource summary:",
		"Path",
		"Count",
		"-----",
		"0 resources    1",
		"> 0 resources  1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("printSummaryTable() output missing %q in %q", want, output)
		}
	}
}

func TestRunShortVersionFlag(t *testing.T) {
	oldVersion := version
	version = "v1.2.3"
	defer func() {
		version = oldVersion
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-v"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(-v) exitCode = %d, want 0", exitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "v1.2.3" {
		t.Fatalf("run(-v) stdout = %q, want %q", got, "v1.2.3")
	}
	if stderr.Len() != 0 {
		t.Fatalf("run(-v) stderr = %q, want empty", stderr.String())
	}
}

func TestRunLongVersionFlag(t *testing.T) {
	oldVersion := version
	version = "dev-build"
	defer func() {
		version = oldVersion
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(--version) exitCode = %d, want 0", exitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "dev-build" {
		t.Fatalf("run(--version) stdout = %q, want %q", got, "dev-build")
	}
	if stderr.Len() != 0 {
		t.Fatalf("run(--version) stderr = %q, want empty", stderr.String())
	}
}

func TestHelpOutputIncludesVersionFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-h"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(-h) exitCode = %d, want 0", exitCode)
	}
	for _, want := range []string{"-version", "-v", "print the tf-slate client version"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("help output missing %q in %q", want, stderr.String())
		}
	}
}

func TestParseTerraformListOutput(t *testing.T) {
	got := parseTerraformListOutput("aws_instance.web\n\nmodule.db.aws_db_instance.primary\n")
	want := []string{"aws_instance.web", "module.db.aws_db_instance.primary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTerraformListOutput() = %#v, want %#v", got, want)
	}
}

func TestHandleListActionShowsResourceAndBack(t *testing.T) {
	var output bytes.Buffer
	var ran [][]string

	session := interactiveSession{
		reader: bufio.NewReader(strings.NewReader("2\nback\n")),
		out:    &output,
		runTerraform: func(args ...string) {
			ran = append(ran, append([]string(nil), args...))
		},
		captureTerraform: func(args ...string) (string, error) {
			return "aws_instance.web\nmodule.db.aws_db_instance.primary\n", nil
		},
		visitDir: func(string) error { return nil },
	}

	handleListAction(session, "/tmp/example.tfstate")

	wantRun := [][]string{{"state", "show", "-state=/tmp/example.tfstate", "module.db.aws_db_instance.primary"}}
	if !reflect.DeepEqual(ran, wantRun) {
		t.Fatalf("handleListAction() ran %#v, want %#v", ran, wantRun)
	}
	if !strings.Contains(output.String(), "Choose a resource to inspect or type back:") {
		t.Fatalf("handleListAction() output = %q, want resource prompt", output.String())
	}
}

func TestRunStateActionsVisitExits(t *testing.T) {
	var output bytes.Buffer
	var visited string

	session := interactiveSession{
		reader:           bufio.NewReader(strings.NewReader("visit\n")),
		out:              &output,
		runTerraform:     func(args ...string) {},
		captureTerraform: func(args ...string) (string, error) { return "", nil },
		visitDir: func(dir string) error {
			visited = dir
			return nil
		},
	}

	exited := runStateActions(session, state.Summary{Path: filepath.Join("/tmp", "stack", "terraform.tfstate")})
	if !exited {
		t.Fatalf("runStateActions(visit) = false, want true")
	}
	if visited != filepath.Join("/tmp", "stack") {
		t.Fatalf("visitDir() called with %q, want %q", visited, filepath.Join("/tmp", "stack"))
	}
	if !strings.Contains(output.String(), "visit  -> open a shell") {
		t.Fatalf("runStateActions() output = %q, want visit help text", output.String())
	}
}
