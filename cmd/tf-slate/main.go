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
	"sort"
	"strconv"
	"strings"

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

	printTable(stdout, displaySummaries)
	if opts.summarize {
		fmt.Fprintln(stdout)
		printSummaryTable(stdout, displaySummaries)
	}
	if opts.nonInteractive {
		return 0
	}

	runTUI(displaySummaries)
	return 0
}

type options struct {
	root           string
	nonInteractive bool
	summarize      bool
	nonZero        bool
	weighted       bool
	showVersion    bool
}

func newFlagSet(stderr io.Writer) (*flag.FlagSet, *options) {
	opts := &options{}

	fs := flag.NewFlagSet("tf-slate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&opts.root, "root", ".", "root path to scan for .tfstate files")
	fs.BoolVar(&opts.nonInteractive, "non-interactive", false, "print state summaries without prompts")
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

func printTable(w io.Writer, summaries []state.Summary) {
	fmt.Fprintln(w, "Found Terraform state files:")
	fmt.Fprintln(w, "#  Resources  Providers         Terraform  Path")
	for i, s := range summaries {
		providers := "-"
		if len(s.Providers) > 0 {
			providers = strings.Join(s.Providers, ",")
		}
		version := s.TerraformVersion
		if version == "" {
			version = "-"
		}
		fmt.Fprintf(w, "%-2d %-10d %-17s %-10s %s\n", i+1, s.ResourceCount, providers, version, s.Path)
	}
}

func printSummaryTable(w io.Writer, summaries []state.Summary) {
	zero, nonZero := countResourcePaths(summaries)
	rows := [][2]string{
		{"0 resources", strconv.Itoa(zero)},
		{"> 0 resources", strconv.Itoa(nonZero)},
	}
	labelWidth := len("Path")
	countWidth := len("Count")
	for _, row := range rows {
		labelWidth = max(labelWidth, len(row[0]))
		countWidth = max(countWidth, len(row[1]))
	}

	fmt.Fprintln(w, "State resource summary:")
	fmt.Fprintf(w, "%-*s  %-*s\n", labelWidth, "Path", countWidth, "Count")
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
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\nSelect a state file number to review (or q to quit): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if strings.EqualFold(input, "q") {
			return
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(summaries) {
			fmt.Println("Invalid selection")
			continue
		}

		s := summaries[idx-1]
		fmt.Printf("\nState: %s\n", s.Path)
		fmt.Printf("Resources: %d | Providers: %s | Terraform: %s | Serial: %d\n", s.ResourceCount, strings.Join(s.Providers, ","), valueOrDash(s.TerraformVersion), s.Serial)
		fmt.Println("Suggested follow-up commands:")
		fmt.Printf("  list   -> terraform state list -state=%q\n", s.Path)
		fmt.Printf("  show   -> terraform state show -state=%q <resource-address>\n", s.Path)
		fmt.Printf("  destroy-> terraform destroy -state=%q\n", s.Path)
		fmt.Print("Run a suggested command now? [list/show/destroy/n]: ")
		action, _ := reader.ReadString('\n')
		action = strings.TrimSpace(strings.ToLower(action))

		switch action {
		case "list":
			runTerraform("state", "list", "-state="+s.Path)
		case "show":
			fmt.Print("Enter resource address: ")
			addr, _ := reader.ReadString('\n')
			addr = strings.TrimSpace(addr)
			if addr == "" {
				fmt.Println("No resource address entered")
				continue
			}
			runTerraform("state", "show", "-state="+s.Path, addr)
		case "destroy":
			fmt.Print("Type DESTROY to confirm: ")
			confirm, _ := reader.ReadString('\n')
			if strings.TrimSpace(confirm) != "DESTROY" {
				fmt.Println("Destroy cancelled")
				continue
			}
			runTerraform("destroy", "-state="+s.Path)
		default:
			fmt.Println("No command executed")
		}
	}
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
