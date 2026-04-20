package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/abuxton/tf-slate/internal/output"
	"github.com/abuxton/tf-slate/internal/state"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs, opts := newFlagSet(stderr)
	if len(args) == 0 {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 1
	}

	if opts.showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}
	if err := applyPositionalRoot(args, fs, opts); err != nil {
		return writeErr(stderr, err)
	}

	format, err := output.ParseFormat(opts.outputFormat)
	if err != nil {
		return writeErr(stderr, err)
	}
	if !opts.nonInteractive && format != output.FormatString {
		return writeErr(stderr, fmt.Errorf("--output %q requires --non-interactive", opts.outputFormat))
	}

	absRoot, err := filepath.Abs(opts.root)
	if err != nil {
		return writeErr(stderr, fmt.Errorf("resolve root path: %w", err))
	}

	paths, err := state.FindStateFiles(absRoot)
	if err != nil {
		return writeErr(stderr, err)
	}
	if len(paths) == 0 {
		fmt.Fprintf(stdout, "No Terraform state files found under %s\n", absRoot)
		return 0
	}

	summaries := make([]state.Summary, 0, len(paths))
	for _, p := range paths {
		s, err := state.SummarizeStateFile(p)
		if err != nil {
			fmt.Fprintf(stderr, "Skipping %s: %v\n", p, err)
			continue
		}
		summaries = append(summaries, s)
	}

	if len(summaries) == 0 {
		fmt.Fprintln(stdout, "No parseable Terraform state files were found")
		return 0
	}

	displaySummaries := summaries
	if opts.nonZero {
		displaySummaries = filterNonZeroSummaries(displaySummaries)
	}
	if opts.weighted {
		sortSummariesWeighted(displaySummaries)
	}
	if len(displaySummaries) == 0 {
		fmt.Fprintln(stdout, "No Terraform state files matched the selected filters")
		return 0
	}

	if opts.nonInteractive {
		if err := output.Write(stdout, format, displaySummaries); err != nil {
			return writeErr(stderr, err)
		}
		return 0
	}

	if err := output.Write(stdout, output.FormatString, displaySummaries); err != nil {
		return writeErr(stderr, err)
	}
	if opts.summarize {
		fmt.Fprintln(stdout)
		printSummaryTable(stdout, displaySummaries)
	}

	runTUI(displaySummaries)
	return 0
}

type options struct {
	root           string
	nonInteractive bool
	outputFormat   string
	summarize      bool
	nonZero        bool
	weighted       bool
	showVersion    bool
}

type interactiveSession struct {
	reader           *bufio.Reader
	out              io.Writer
	runTerraform     func(args ...string)
	captureTerraform func(args ...string) (string, error)
	visitDir         func(string) error
}

func newFlagSet(stderr io.Writer) (*flag.FlagSet, *options) {
	opts := &options{}

	fs := flag.NewFlagSet("tf-slate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&opts.root, "root", ".", "root path to scan for .tfstate files")
	fs.BoolVar(&opts.nonInteractive, "non-interactive", false, "print state summaries without prompts")
	fs.BoolVar(&opts.nonInteractive, "ni", false, "alias for --non-interactive")
	fs.StringVar(&opts.outputFormat, "output", string(output.FormatString), "non-interactive output format: string, json, yaml, or csv")
	fs.StringVar(&opts.outputFormat, "o", string(output.FormatString), "alias for --output")
	fs.BoolVar(&opts.summarize, "summarize", false, "print a summary table grouping state files by zero and non-zero resource counts")
	fs.BoolVar(&opts.summarize, "s", false, "alias for --summarize")
	fs.BoolVar(&opts.nonZero, "non-zero", false, "show only state files with more than zero resources")
	fs.BoolVar(&opts.nonZero, "nz", false, "alias for --non-zero")
	fs.BoolVar(&opts.weighted, "weighted", false, "sort state files by greatest resource count first")
	fs.BoolVar(&opts.weighted, "w", false, "alias for --weighted")
	fs.BoolVar(&opts.showVersion, "version", false, "print the tf-slate client version")
	fs.BoolVar(&opts.showVersion, "v", false, "alias for --version")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of %s:\n", fs.Name())
		fs.PrintDefaults()
	}

	return fs, opts
}

func applyPositionalRoot(args []string, fs *flag.FlagSet, opts *options) error {
	switch fs.NArg() {
	case 0:
		return nil
	case 1:
		if hasRootFlag(args) {
			return fmt.Errorf("unexpected argument %q", fs.Arg(0))
		}
		opts.root = fs.Arg(0)
		return nil
	default:
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), ", "))
	}
}

func hasRootFlag(args []string) bool {
	for _, arg := range args {
		switch {
		case arg == "-root", arg == "--root":
			return true
		case strings.HasPrefix(arg, "-root="), strings.HasPrefix(arg, "--root="):
			return true
		}
	}
	return false
}


func printSummaryTable(w io.Writer, summaries []state.Summary) {
	zero, nonZero := countResourcePaths(summaries)
	rows := [][2]string{
		{"0 resources", strconv.Itoa(zero)},
		{"> 0 resources", strconv.Itoa(nonZero)},
	}
	labelWidth := len("Bucket")
	countWidth := len("Count")
	for _, row := range rows {
		labelWidth = max(labelWidth, len(row[0]))
		countWidth = max(countWidth, len(row[1]))
	}

	fmt.Fprintln(w, "State resource summary:")
	fmt.Fprintf(w, "%-*s  %-*s\n", labelWidth, "Bucket", countWidth, "Count")
	fmt.Fprintf(w, "%s  %s\n", strings.Repeat("-", labelWidth), strings.Repeat("-", countWidth))
	for _, row := range rows {
		fmt.Fprintf(w, "%-*s  %s\n", labelWidth, row[0], row[1])
	}
}

func countResourcePaths(summaries []state.Summary) (zero, nonZero int) {
	for _, s := range summaries {
		if s.ResourceCount == 0 {
			zero++
			continue
		}
		nonZero++
	}
	return zero, nonZero
}

func filterNonZeroSummaries(summaries []state.Summary) []state.Summary {
	filtered := make([]state.Summary, 0, len(summaries))
	for _, s := range summaries {
		if s.ResourceCount > 0 {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func sortSummariesWeighted(summaries []state.Summary) {
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].ResourceCount != summaries[j].ResourceCount {
			return summaries[i].ResourceCount > summaries[j].ResourceCount
		}
		return summaries[i].Path < summaries[j].Path
	})
}

func runTUI(summaries []state.Summary) {
	session := interactiveSession{
		reader:           bufio.NewReader(os.Stdin),
		out:              os.Stdout,
		runTerraform:     runTerraform,
		captureTerraform: captureTerraform,
		visitDir:         openShellInDir,
	}
	runTUIWithSession(session, summaries)
}

func runTUIWithSession(session interactiveSession, summaries []state.Summary) {
	for {
		fmt.Fprint(session.out, "\nSelect a state file number to review (or q to quit): ")
		input, _ := session.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if strings.EqualFold(input, "q") {
			return
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(summaries) {
			fmt.Fprintln(session.out, "Invalid selection")
			continue
		}

		if runStateActions(session, summaries[idx-1]) {
			return
		}
	}
}

func runStateActions(session interactiveSession, summary state.Summary) bool {
	for {
		fmt.Fprintf(session.out, "\nState: %s\n", summary.Path)
		fmt.Fprintf(session.out, "Resources: %d | Providers: %s | Terraform: %s | Serial: %d\n", summary.ResourceCount, strings.Join(summary.Providers, ","), valueOrDash(summary.TerraformVersion), summary.Serial)
		fmt.Fprintln(session.out, "Suggested follow-up commands:")
		fmt.Fprintf(session.out, "  list   -> terraform state list -state=%q\n", summary.Path)
		fmt.Fprintf(session.out, "  show   -> terraform state show -state=%q <resource-address>\n", summary.Path)
		fmt.Fprintf(session.out, "  destroy-> terraform destroy -state=%q\n", summary.Path)
		fmt.Fprintf(session.out, "  visit  -> open a shell in %q and exit tf-slate\n", filepath.Dir(summary.Path))
		fmt.Fprint(session.out, "Run a suggested command now? [list/show/destroy/visit/back/n]: ")

		action, _ := session.reader.ReadString('\n')
		action = strings.TrimSpace(strings.ToLower(action))

		switch action {
		case "list":
			handleListAction(session, summary.Path)
		case "show":
			fmt.Fprint(session.out, "Enter resource address: ")
			addr, _ := session.reader.ReadString('\n')
			addr = strings.TrimSpace(addr)
			if addr == "" {
				fmt.Fprintln(session.out, "No resource address entered")
				continue
			}
			session.runTerraform("state", "show", "-state="+summary.Path, addr)
		case "destroy":
			fmt.Fprint(session.out, "Type DESTROY to confirm: ")
			confirm, _ := session.reader.ReadString('\n')
			if strings.TrimSpace(confirm) != "DESTROY" {
				fmt.Fprintln(session.out, "Destroy cancelled")
				continue
			}
			session.runTerraform("destroy", "-state="+summary.Path)
		case "visit":
			fmt.Fprintf(session.out, "Opening shell in %s\n", filepath.Dir(summary.Path))
			if err := session.visitDir(filepath.Dir(summary.Path)); err != nil {
				fmt.Fprintf(session.out, "visit failed: %v\n", err)
				continue
			}
			return true
		case "back", "b", "n":
			fmt.Fprintln(session.out, "Returning to the state file list")
			return false
		default:
			fmt.Fprintln(session.out, "No command executed")
		}
	}
}

func handleListAction(session interactiveSession, statePath string) {
	output, err := session.captureTerraform("state", "list", "-state="+statePath)
	if err != nil {
		fmt.Fprintf(session.out, "terraform state list failed: %v\n", err)
		if strings.TrimSpace(output) != "" {
			fmt.Fprintln(session.out, output)
		}
		return
	}

	resources := parseTerraformListOutput(output)
	if len(resources) == 0 {
		fmt.Fprintln(session.out, "No resources found in the selected state file")
		return
	}

	for {
		fmt.Fprintln(session.out, "State resources:")
		for i, resource := range resources {
			fmt.Fprintf(session.out, "  %d. %s\n", i+1, resource)
		}
		fmt.Fprint(session.out, "Choose a resource to inspect or type back: ")

		input, _ := session.reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		switch input {
		case "back", "b":
			return
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(resources) {
			fmt.Fprintln(session.out, "Invalid selection")
			continue
		}

		session.runTerraform("state", "show", "-state="+statePath, resources[idx-1])
	}
}

func parseTerraformListOutput(output string) []string {
	lines := strings.Split(output, "\n")
	resources := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		resources = append(resources, line)
	}
	return resources
}

func runTerraform(args ...string) {
	if _, err := exec.LookPath("terraform"); err != nil {
		fmt.Println("terraform executable not found; copy and run the suggested command manually")
		return
	}
	cmd := exec.Command("terraform", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Printf("terraform command failed: %v\n", err)
	}
}

func captureTerraform(args ...string) (string, error) {
	if _, err := exec.LookPath("terraform"); err != nil {
		return "", errors.New("terraform executable not found")
	}
	cmd := exec.Command("terraform", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func openShellInDir(dir string) error {
	shell, args := shellCommand()
	cmd := exec.Command(shell, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func shellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		if shell := strings.TrimSpace(os.Getenv("COMSPEC")); shell != "" {
			return shell, []string{"/K"}
		}
		return "cmd.exe", []string{"/K"}
	}

	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		return shell, []string{"-i"}
	}
	return "/bin/sh", []string{"-i"}
}

func valueOrDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func writeErr(w io.Writer, err error) int {
	fmt.Fprintln(w, err)
	return 1
}
