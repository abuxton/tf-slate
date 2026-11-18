package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
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
		"Bucket",
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

func TestRunNoArgsShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(nil, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(nil) exitCode = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "Usage of tf-slate:") {
		t.Fatalf("run(nil) stdout = %q, want usage output", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("run(nil) stderr = %q, want empty", stderr.String())
	}
}

func TestRunSinglePositionalPathUsesRoot(t *testing.T) {
	dir := t.TempDir()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{dir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(%q) exitCode = %d, want 0", dir, exitCode)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	want := "No Terraform state files found under " + absDir
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("run(%q) stdout = %q, want %q", dir, stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("run(%q) stderr = %q, want empty", dir, stderr.String())
	}
}

func TestRunUnexpectedExtraArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"one", "two"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("run(unexpected args) exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "unexpected arguments: one, two") {
		t.Fatalf("run(unexpected args) stderr = %q, want unexpected arguments error", stderr.String())
	}
}

func TestHelpOutputIncludesOutputFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-h"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(-h) exitCode = %d, want 0", exitCode)
	}
	for _, want := range []string{"-ni", "-output", "-o", "non-interactive output format"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("help output missing %q in %q", want, stderr.String())
		}
	}
}

func TestRunNonInteractiveAlias(t *testing.T) {
	root := writeTestStateFile(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--ni", "-root", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(--ni) exitCode = %d, want 0 with stderr %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Found Terraform state files:") {
		t.Fatalf("run(--ni) stdout = %q, want table output", stdout.String())
	}
	if strings.Contains(stdout.String(), "Select a state file number") {
		t.Fatalf("run(--ni) stdout unexpectedly contained TUI prompt: %q", stdout.String())
	}
}

func TestRunJSONOutput(t *testing.T) {
	root := writeTestStateFile(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--ni", "-o", "json", "-root", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(json) exitCode = %d, want 0 with stderr %q", exitCode, stderr.String())
	}

	var got []struct {
		Path             string   `json:"path"`
		ResourceCount    int      `json:"resource_count"`
		Providers        []string `json:"providers"`
		TerraformVersion string   `json:"terraform_version"`
		Serial           int      `json:"serial"`
		Lineage          string   `json:"lineage"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("run(json) stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if len(got) != 1 {
		t.Fatalf("run(json) returned %d summaries, want 1", len(got))
	}
	if got[0].Path == "" || got[0].ResourceCount != 2 {
		t.Fatalf("run(json) summary = %#v, want populated summary", got[0])
	}
	if strings.Contains(stdout.String(), "State resource summary:") {
		t.Fatalf("run(json) unexpectedly included summary footer: %q", stdout.String())
	}
}

func TestRunCSVOutput(t *testing.T) {
	root := writeTestStateFile(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--ni", "--output", "csv", "-root", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(csv) exitCode = %d, want 0 with stderr %q", exitCode, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("run(csv) line count = %d, want 2 in %q", len(lines), stdout.String())
	}
	if got := lines[0]; got != "path,resource_count,providers,terraform_version,serial,lineage" {
		t.Fatalf("run(csv) header = %q, want csv header", got)
	}
}

func TestRunOutputRequiresNonInteractive(t *testing.T) {
	root := writeTestStateFile(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-root", root, "-o", "json"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("run(-o json) exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), `requires --non-interactive`) {
		t.Fatalf("run(-o json) stderr = %q, want non-interactive requirement", stderr.String())
	}
}

func TestParseTerraformListOutput(t *testing.T) {
	got := parseTerraformListOutput("aws_instance.web\n\nmodule.db.aws_db_instance.primary\n")
	want := []string{"aws_instance.web", "module.db.aws_db_instance.primary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTerraformListOutput() = %#v, want %#v", got, want)
	}
}

func TestReadTTYMenuEventFromReader(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want menuEvent
	}{
		{
			name: "arrow down",
			in:   "\x1b[B",
			want: menuEvent{move: 1},
		},
		{
			name: "arrow up",
			in:   "\x1b[A",
			want: menuEvent{move: -1},
		},
		{
			name: "enter",
			in:   "\r",
			want: menuEvent{confirm: true},
		},
		{
			name: "digit",
			in:   "4",
			want: menuEvent{text: "4"},
		},
		{
			name: "backspace",
			in:   "\x7f",
			want: menuEvent{backspace: true},
		},
		{
			name: "lone ESC treated as back",
			in:   "\x1b",
			want: menuEvent{text: "back"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := readTTYMenuEventFromReader(bufio.NewReader(strings.NewReader(tt.in)))
			if !ok {
				t.Fatalf("readTTYMenuEventFromReader() ok = false, want true")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("readTTYMenuEventFromReader() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRunTUIWithSessionUsesCompactStatePrompt(t *testing.T) {
	var output bytes.Buffer

	summaries := []state.Summary{
		{Path: "/tmp/first.tfstate", ResourceCount: 1, Providers: []string{"aws"}, TerraformVersion: "1.6.0", Serial: 1},
		{Path: "/tmp/second.tfstate", ResourceCount: 2, Providers: []string{"random"}, TerraformVersion: "1.6.1", Serial: 2},
	}

	session := interactiveSession{
		reader:           bufio.NewReader(strings.NewReader(moveDown(1) + moveDown(5) + "q\n")),
		out:              &output,
		runTerraform:     func(dir string, args ...string) {},
		captureTerraform: func(dir string, args ...string) (string, error) { return "", nil },
		visitDir:         func(string) error { return nil },
	}

	runTUIWithSession(session, summaries)

	got := output.String()
	for _, want := range []string{
		"/tmp/second.tfstate",
		"Selection [1/2]:",
		"Use \u2191/\u2193 then Enter, or type a number or q.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("runTUIWithSession() output missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{
		"/tmp/first.tfstate — 1 resources",
		"/tmp/second.tfstate — 2 resources",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("runTUIWithSession() output unexpectedly contained %q in %q", unwanted, got)
		}
	}
}

func TestHandleListActionArrowSelectionShowsResourceAndLoops(t *testing.T) {
	var output bytes.Buffer
	var ran [][]string

	session := interactiveSession{
		reader: bufio.NewReader(strings.NewReader(moveDown(1) + "\n" + "back\n")),
		out:    &output,
		runTerraform: func(dir string, args ...string) {
			ran = append(ran, append([]string(nil), args...))
		},
		captureTerraform: func(dir string, args ...string) (string, error) {
			return "aws_instance.web\nmodule.db.aws_db_instance.primary\n", nil
		},
		visitDir: func(string) error { return nil },
	}

	exited := handleListAction(session, "/tmp/example.tfstate")
	if exited {
		t.Fatalf("handleListAction() = true, want false")
	}

	wantRun := [][]string{{"state", "show", "-state=/tmp/example.tfstate", "module.db.aws_db_instance.primary"}}
	if !reflect.DeepEqual(ran, wantRun) {
		t.Fatalf("handleListAction() ran %#v, want %#v", ran, wantRun)
	}
	got := output.String()
	for _, want := range []string{
		"State resources",
		"What next?",
		"List another resource",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("handleListAction() output missing %q in %q", want, got)
		}
	}
}

func TestHandleListActionQuitAfterShowingResource(t *testing.T) {
	var output bytes.Buffer
	var ran [][]string

	session := interactiveSession{
		reader: bufio.NewReader(strings.NewReader("\n" + moveDown(2))),
		out:    &output,
		runTerraform: func(dir string, args ...string) {
			ran = append(ran, append([]string(nil), args...))
		},
		captureTerraform: func(dir string, args ...string) (string, error) {
			return "aws_instance.web\nmodule.db.aws_db_instance.primary\n", nil
		},
		visitDir: func(string) error { return nil },
	}

	exited := handleListAction(session, "/tmp/example.tfstate")
	if !exited {
		t.Fatalf("handleListAction() = false, want true")
	}

	wantRun := [][]string{{"state", "show", "-state=/tmp/example.tfstate", "aws_instance.web"}}
	if !reflect.DeepEqual(ran, wantRun) {
		t.Fatalf("handleListAction() ran %#v, want %#v", ran, wantRun)
	}
}

func TestRunStateActionsShowThenBack(t *testing.T) {
	var output bytes.Buffer
	var ran [][]string

	session := interactiveSession{
		reader: bufio.NewReader(strings.NewReader(moveDown(1) + "aws_instance.web\n" + moveDown(4) + moveDown(5))),
		out:    &output,
		runTerraform: func(dir string, args ...string) {
			ran = append(ran, append([]string(nil), args...))
		},
		captureTerraform: func(dir string, args ...string) (string, error) { return "", nil },
		visitDir:         func(string) error { return nil },
	}

	exited := runStateActions(session, state.Summary{Path: "/tmp/example.tfstate"})
	if exited {
		t.Fatalf("runStateActions(show) = true, want false")
	}

	wantRun := [][]string{{"state", "show", "-state=/tmp/example.tfstate", "aws_instance.web"}}
	if !reflect.DeepEqual(ran, wantRun) {
		t.Fatalf("runStateActions(show) ran %#v, want %#v", ran, wantRun)
	}
}

func TestRunStateActionsDestroyCancelledAndBack(t *testing.T) {
	var output bytes.Buffer
	var ran [][]string

	session := interactiveSession{
		reader: bufio.NewReader(strings.NewReader(moveDown(2) + "NOPE\n" + moveDown(5))),
		out:    &output,
		runTerraform: func(dir string, args ...string) {
			ran = append(ran, append([]string(nil), args...))
		},
		captureTerraform: func(dir string, args ...string) (string, error) { return "", nil },
		visitDir:         func(string) error { return nil },
	}

	exited := runStateActions(session, state.Summary{Path: "/tmp/example.tfstate"})
	if exited {
		t.Fatalf("runStateActions(destroy) = true, want false")
	}
	if len(ran) != 0 {
		t.Fatalf("runStateActions(destroy) ran %#v, want no terraform command", ran)
	}
	if !strings.Contains(output.String(), "Destroy cancelled") {
		t.Fatalf("runStateActions(destroy) output = %q, want cancel message", output.String())
	}
}

func TestRunStateActionsVisitExits(t *testing.T) {
	var output bytes.Buffer
	var visited string

	session := interactiveSession{
		reader:           bufio.NewReader(strings.NewReader(moveDown(3))),
		out:              &output,
		runTerraform:     func(dir string, args ...string) {},
		captureTerraform: func(dir string, args ...string) (string, error) { return "", nil },
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
	if !strings.Contains(output.String(), "Open a shell in") {
		t.Fatalf("runStateActions() output = %q, want visit menu text", output.String())
	}
}

func deleteTestSession(input string, deleted *[]string) interactiveSession {
	return interactiveSession{
		reader:     bufio.NewReader(strings.NewReader(input)),
		out:        &bytes.Buffer{},
		deleteFile: func(path string) error { *deleted = append(*deleted, path); return nil },
	}
}

func TestDeleteStateFileRequiresExactConfirmation(t *testing.T) {
	var deleted []string
	session := deleteTestSession("delete\n", &deleted)

	if deleteStateFile(session, state.Summary{Path: "/w/terraform.tfstate"}) {
		t.Fatalf("deleteStateFile() = true for wrong confirmation")
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted %v, want nothing", deleted)
	}
}

func TestDeleteStateFileWarnsAndRemovesBackupOnlyWhenAsked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "terraform.tfstate")
	for _, f := range []string{path, path + ".backup"} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range []struct {
		answer string
		want   []string
	}{
		{"n\n", []string{path}},
		{"y\n", []string{path, path + ".backup"}},
	} {
		var deleted []string
		session := deleteTestSession("DELETE\n"+tt.answer, &deleted)
		out := session.out.(*bytes.Buffer)

		if !deleteStateFile(session, state.Summary{Path: path, ResourceCount: 3}) {
			t.Fatalf("deleteStateFile() = false, want true")
		}
		if !reflect.DeepEqual(deleted, tt.want) {
			t.Fatalf("answer %q deleted %v, want %v", tt.answer, deleted, tt.want)
		}
		if !strings.Contains(out.String(), "3 managed resource") {
			t.Fatalf("output = %q, want resource warning", out.String())
		}
	}
}

func TestDeleteStateFileReportsRemovalError(t *testing.T) {
	session := interactiveSession{
		reader:     bufio.NewReader(strings.NewReader("DELETE\n")),
		out:        &bytes.Buffer{},
		deleteFile: func(string) error { return errors.New("permission denied") },
	}
	if deleteStateFile(session, state.Summary{Path: "/w/terraform.tfstate"}) {
		t.Fatalf("deleteStateFile() = true on removal error")
	}
	if !strings.Contains(session.out.(*bytes.Buffer).String(), "permission denied") {
		t.Fatalf("error not reported")
	}
}

func TestRunTUIDeleteRemovesEntryAndEndsWhenEmpty(t *testing.T) {
	var deleted []string
	var output bytes.Buffer
	session := interactiveSession{
		reader:     bufio.NewReader(strings.NewReader("1\n" + moveDown(4) + "DELETE\n")),
		out:        &output,
		deleteFile: func(p string) error { deleted = append(deleted, p); return nil },
	}

	runTUIWithSession(session, []state.Summary{{Path: "/w/only.tfstate"}})

	if !reflect.DeepEqual(deleted, []string{"/w/only.tfstate"}) {
		t.Fatalf("deleted %v", deleted)
	}
	if !strings.Contains(output.String(), "No Terraform state files remain") {
		t.Fatalf("output = %q, want empty-list message", output.String())
	}
}

func moveDown(count int) string {
	return strings.Repeat("\x1b[B", count) + "\n"
}

func TestStateArgs(t *testing.T) {
	tests := []struct {
		name string
		path string
		sub  []string
		want []string
	}{
		{"default destroy", "/w/terraform.tfstate", []string{"destroy"}, []string{"destroy"}},
		{"default show", "/w/terraform.tfstate", []string{"state", "show", "a.b"}, []string{"state", "show", "a.b"}},
		{"custom destroy", "/w/x.tfstate", []string{"destroy"}, []string{"destroy", "-state=/w/x.tfstate"}},
		{"custom list", "/w/x.tfstate", []string{"state", "list"}, []string{"state", "list", "-state=/w/x.tfstate"}},
		{"custom show", "/w/x.tfstate", []string{"state", "show", "a.b"}, []string{"state", "show", "-state=/w/x.tfstate", "a.b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stateArgs(tt.path, tt.sub...); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("stateArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRunStateActionsDestroyRunsInStateDir(t *testing.T) {
	var output bytes.Buffer
	var gotDir string
	var gotArgs []string
	session := interactiveSession{
		reader: bufio.NewReader(strings.NewReader("destroy\nDESTROY\nback\n")),
		out:    &output,
		runTerraform: func(dir string, args ...string) {
			gotDir, gotArgs = dir, args
		},
		checkSetup: func(string) error { return nil },
	}

	runStateActions(session, state.Summary{Path: filepath.Join("/w", "stack", "terraform.tfstate")})

	if gotDir != filepath.Join("/w", "stack") || !reflect.DeepEqual(gotArgs, []string{"destroy"}) {
		t.Fatalf("destroy ran in %q with %#v", gotDir, gotArgs)
	}
}

func TestRunStateActionsDestroySkippedWhenNotInitialized(t *testing.T) {
	var output bytes.Buffer
	ran := false
	session := interactiveSession{
		reader:       bufio.NewReader(strings.NewReader("destroy\nDESTROY\nback\n")),
		out:          &output,
		runTerraform: func(string, ...string) { ran = true },
		checkSetup:   func(string) error { return errors.New("not initialized") },
	}

	runStateActions(session, state.Summary{Path: "/w/terraform.tfstate"})

	if ran {
		t.Fatalf("terraform ran despite failed setup check")
	}
	if !strings.Contains(output.String(), "not initialized") {
		t.Fatalf("output = %q, want setup message", output.String())
	}
}

func TestCheckTerraformSetup(t *testing.T) {
	dir := t.TempDir()
	if err := checkTerraformSetup(dir); err == nil || !strings.Contains(err.Error(), "configuration") {
		t.Fatalf("empty dir error = %v, want missing configuration", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkTerraformSetup(dir); err == nil || !strings.Contains(err.Error(), "terraform init") {
		t.Fatalf("uninitialized dir error = %v, want init hint", err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".terraform"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkTerraformSetup(dir); err != nil {
		t.Fatalf("ready dir error = %v", err)
	}
}

func writeTestStateFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "terraform.tfstate")
	content := `{
  "terraform_version": "1.6.0",
  "serial": 7,
  "lineage": "lineage-1",
  "resources": [
    {
      "mode": "managed",
      "provider_name": "aws",
      "instances": [{}, {}]
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return dir
}
