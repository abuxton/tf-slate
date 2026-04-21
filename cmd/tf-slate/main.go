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
	"golang.org/x/term"
)

var version = "dev"

const (
	ansiReset    = "\x1b[0m"
	ansiBoldCyan = "\x1b[1;36m"
	ansiCyan     = "\x1b[36m"
	ansiGreen    = "\x1b[32m"
	ansiYellow   = "\x1b[33m"
)

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
	stdin            *os.File
	runTerraform     func(args ...string)
	captureTerraform func(args ...string) (string, error)
	visitDir         func(string) error
}

type menuOption struct {
	value       string
	label       string
	description string
}

type menuEvent struct {
	move      int
	text      string
	backspace bool
	confirm   bool
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
		stdin:            os.Stdin,
		runTerraform:     runTerraform,
		captureTerraform: captureTerraform,
		visitDir:         openShellInDir,
	}
	runTUIWithSession(session, summaries)
}

func runTUIWithSession(session interactiveSession, summaries []state.Summary) {
	selected := 0
	for {
		next, quit := promptStateSelection(session, summaries, selected)
		if quit {
			return
		}
		selected = next

		if runStateActions(session, summaries[selected]) {
			return
		}
	}
}

func runStateActions(session interactiveSession, summary state.Summary) bool {
	for {
		fmt.Fprintf(session.out, "\nState: %s\n", summary.Path)
		fmt.Fprintf(session.out, "Resources: %d | Providers: %s | Terraform: %s | Serial: %d\n", summary.ResourceCount, strings.Join(summary.Providers, ","), valueOrDash(summary.TerraformVersion), summary.Serial)
		action := promptMenu(session, "Suggested follow-up commands", "Choose the next Terraform action for this state file.", []menuOption{
			{value: "list", label: "List resources", description: fmt.Sprintf("terraform state list -state=%q", summary.Path)},
			{value: "show", label: "Show a resource by address", description: fmt.Sprintf("terraform state show -state=%q <resource-address>", summary.Path)},
			{value: "destroy", label: "Destroy resources", description: fmt.Sprintf("terraform destroy -state=%q", summary.Path)},
			{value: "visit", label: "Open a shell in the state directory", description: fmt.Sprintf("Open a shell in %q and exit tf-slate", filepath.Dir(summary.Path))},
			{value: "back", label: "Back to the state list", description: "Return to the previous menu."},
			{value: "quit", label: "Quit", description: "Exit tf-slate."},
		})

		switch action {
		case "list":
			if handleListAction(session, summary.Path) {
				return true
			}
		case "show":
			fmt.Fprint(session.out, "Enter resource address: ")
			addr, _ := session.reader.ReadString('\n')
			addr = strings.TrimSpace(addr)
			if addr == "" {
				fmt.Fprintln(session.out, "No resource address entered")
				continue
			}
			session.runTerraform("state", "show", "-state="+summary.Path, addr)
			switch handlePostShowAction(session, summary.Path) {
			case "list":
				if handleListAction(session, summary.Path) {
					return true
				}
			case "quit":
				return true
			}
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
		case "quit", "q":
			return true
		default:
			fmt.Fprintln(session.out, "No command executed")
		}
	}
}

func handleListAction(session interactiveSession, statePath string) bool {
	output, err := session.captureTerraform("state", "list", "-state="+statePath)
	if err != nil {
		fmt.Fprintf(session.out, "terraform state list failed: %v\n", err)
		if strings.TrimSpace(output) != "" {
			fmt.Fprintln(session.out, output)
		}
		return false
	}

	resources := parseTerraformListOutput(output)
	if len(resources) == 0 {
		fmt.Fprintln(session.out, "No resources found in the selected state file")
		return false
	}

	for {
		options := make([]menuOption, 0, len(resources)+2)
		for _, resource := range resources {
			options = append(options, menuOption{
				value:       "resource:" + resource,
				label:       resource,
				description: fmt.Sprintf("terraform state show -state=%q %s", statePath, resource),
			})
		}
		options = append(options,
			menuOption{value: "back", label: "Back to state actions", description: "Return to the previous menu."},
			menuOption{value: "quit", label: "Quit", description: "Exit tf-slate."},
		)

		selection := promptMenu(session, "State resources", "Select a resource to inspect.", options)
		switch selection {
		case "back":
			return false
		case "quit":
			return true
		}

		resource := strings.TrimPrefix(selection, "resource:")
		session.runTerraform("state", "show", "-state="+statePath, resource)
		switch handlePostShowAction(session, statePath) {
		case "list":
			continue
		case "back":
			return false
		case "quit":
			return true
		}
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

func handlePostShowAction(session interactiveSession, statePath string) string {
	for {
		action := promptMenu(session, "What next?", "Choose what to do after viewing resource details.", []menuOption{
			{value: "list", label: "List another resource", description: fmt.Sprintf("terraform state list -state=%q", statePath)},
			{value: "back", label: "Back to state actions", description: "Return to the previous menu."},
			{value: "quit", label: "Quit", description: "Exit tf-slate."},
		})

		switch action {
		case "list":
			return "list"
		case "back":
			return "back"
		case "quit":
			return "quit"
		default:
			fmt.Fprintln(session.out, "Invalid selection")
		}
	}
}

func promptStateSelection(session interactiveSession, summaries []state.Summary, selected int) (int, bool) {
	if len(summaries) == 0 {
		return 0, true
	}
	if selected < 0 || selected >= len(summaries) {
		selected = 0
	}

	var typed string
	for {
		renderStatePrompt(session.out, summaries, selected, typed)

		event, ok := readMenuEvent(session)
		if !ok {
			return selected, true
		}

		if event.move != 0 {
			selected = moveSelection(selected, len(summaries), event.move)
			if !event.confirm {
				continue
			}
		}

		if event.backspace {
			if len(typed) > 0 {
				typed = typed[:len(typed)-1]
			}
			continue
		}

		if event.text != "" {
			typed += event.text
		}
		if !event.confirm {
			continue
		}

		input := strings.TrimSpace(typed)
		typed = ""
		if input == "" {
			return selected, false
		}

		switch strings.ToLower(input) {
		case "q", "quit", "exit":
			return selected, true
		}

		idx, err := strconv.Atoi(input)
		if err == nil && idx >= 1 && idx <= len(summaries) {
			return idx - 1, false
		}

		fmt.Fprintln(session.out, "Invalid selection")
	}
}

func promptMenu(session interactiveSession, title, description string, options []menuOption) string {
	selected := 0
	var typed string
	for {
		renderMenu(session.out, title, description, options, selected, typed)

		event, ok := readMenuEvent(session)
		if !ok {
			return "quit"
		}

		if event.move != 0 {
			selected = moveSelection(selected, len(options), event.move)
			if !event.confirm {
				continue
			}
		}

		if event.backspace {
			if len(typed) > 0 {
				typed = typed[:len(typed)-1]
			}
			continue
		}

		if event.text != "" {
			typed += event.text
		}
		if !event.confirm {
			continue
		}

		input := strings.TrimSpace(typed)
		typed = ""
		if input == "" {
			return options[selected].value
		}

		switch strings.ToLower(input) {
		case "q", "quit", "exit":
			return "quit"
		case "back", "b":
			return "back"
		}

		idx, err := strconv.Atoi(input)
		if err == nil && idx >= 1 && idx <= len(options) {
			return options[idx-1].value
		}

		for idx, option := range options {
			if strings.EqualFold(input, option.value) || strings.EqualFold(input, option.label) {
				selected = idx
				return option.value
			}
		}

		fmt.Fprintln(session.out, "Invalid selection")
	}
}

func renderStatePrompt(w io.Writer, summaries []state.Summary, selected int, typed string) {
	summary := summaries[selected]
	fmt.Fprintf(w, "\n%sSelect a state file%s\n", ansiBoldCyan, ansiReset)
	fmt.Fprintf(w, "%sReview state files discovered in the scan.%s\n", ansiCyan, ansiReset)
	fmt.Fprintf(w, "%sUse ↑/↓ then Enter, or type a number or q.%s\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "%sCurrent [%d/%d]%s\n", ansiGreen, selected+1, len(summaries), ansiReset)
	fmt.Fprintf(w, "%s%s%s\n", ansiCyan, summary.Path, ansiReset)
	fmt.Fprintf(w, "%s%d resources | providers: %s | terraform: %s%s\n", ansiCyan, summary.ResourceCount, valueOrDash(strings.Join(summary.Providers, ",")), valueOrDash(summary.TerraformVersion), ansiReset)
	fmt.Fprintf(w, "%sSelection [%d/%d]:%s %s\n", ansiYellow, selected+1, len(summaries), ansiReset, typed)
}

func renderMenu(w io.Writer, title, description string, options []menuOption, selected int, typed string) {
	fmt.Fprintf(w, "\n%s%s%s\n", ansiBoldCyan, title, ansiReset)
	if description != "" {
		fmt.Fprintf(w, "%s%s%s\n", ansiCyan, description, ansiReset)
	}
	fmt.Fprintf(w, "%sUse ↑/↓ then Enter to choose. Type a number, back, or q instead.%s\n", ansiYellow, ansiReset)
	for idx, option := range options {
		prefix := "  "
		color := ansiCyan
		if idx == selected {
			prefix = "› "
			color = ansiGreen
		}
		line := option.label
		if option.description != "" {
			line += " — " + option.description
		}
		fmt.Fprintf(w, "%s%s%s%s\n", color, prefix, line, ansiReset)
	}
	fmt.Fprintf(w, "%sSelection:%s %s\n", ansiYellow, ansiReset, typed)
}

func readInteractiveInput(reader *bufio.Reader) (string, bool) {
	input, err := reader.ReadString('\n')
	if err != nil && len(input) == 0 {
		return "", false
	}
	return strings.TrimRight(input, "\r\n"), true
}

func readMenuEvent(session interactiveSession) (menuEvent, bool) {
	if session.stdin != nil && term.IsTerminal(int(session.stdin.Fd())) {
		return readMenuEventFromTerminal(session.stdin)
	}
	return readBufferedMenuEvent(session.reader)
}

func readBufferedMenuEvent(reader *bufio.Reader) (menuEvent, bool) {
	input, ok := readInteractiveInput(reader)
	if !ok {
		return menuEvent{}, false
	}
	move, arrowInput := parseArrowInput(input)
	if arrowInput {
		return menuEvent{move: move, confirm: true}, true
	}
	return menuEvent{text: input, confirm: true}, true
}

func readMenuEventFromTerminal(stdin *os.File) (menuEvent, bool) {
	fd := int(stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return readBufferedMenuEvent(bufio.NewReader(stdin))
	}
	defer term.Restore(fd, state)

	event, ok := readTTYMenuEventFromReader(stdin)
	if !ok {
		return menuEvent{}, false
	}
	return event, true
}

func readTTYMenuEventFromReader(r io.Reader) (menuEvent, bool) {
	buf := make([]byte, 1)
	if _, err := io.ReadFull(r, buf); err != nil {
		return menuEvent{}, false
	}

	switch buf[0] {
	case '\r', '\n':
		return menuEvent{confirm: true}, true
	case 0x7f, 0x08:
		return menuEvent{backspace: true}, true
	case 0x1b:
		seq := make([]byte, 2)
		if _, err := io.ReadFull(r, seq); err != nil {
			return menuEvent{}, false
		}
		switch string(seq) {
		case "[A":
			return menuEvent{move: -1}, true
		case "[B":
			return menuEvent{move: 1}, true
		default:
			return menuEvent{}, true
		}
	default:
		if buf[0] >= 32 && buf[0] <= 126 {
			return menuEvent{text: string(buf[0])}, true
		}
		return menuEvent{}, true
	}
}

func parseArrowInput(input string) (int, bool) {
	remaining := strings.TrimSpace(input)
	if remaining == "" {
		return 0, false
	}

	move := 0
	for len(remaining) > 0 {
		switch {
		case strings.HasPrefix(remaining, "\x1b[A"):
			move--
			remaining = strings.TrimPrefix(remaining, "\x1b[A")
		case strings.HasPrefix(remaining, "\x1b[B"):
			move++
			remaining = strings.TrimPrefix(remaining, "\x1b[B")
		default:
			return 0, false
		}
	}
	return move, true
}

func moveSelection(selected, total, move int) int {
	if total == 0 {
		return selected
	}
	updated := selected
	if move > 0 {
		for i := 0; i < move; i++ {
			updated = (updated + 1) % total
		}
		return updated
	}
	for i := move; i < 0; i++ {
		updated--
		if updated < 0 {
			updated = total - 1
		}
	}
	return updated
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
