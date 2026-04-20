package main

import (
	"bufio"
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

func main() {
	root := flag.String("root", ".", "root path to scan for .tfstate files")
	nonInteractive := flag.Bool("non-interactive", false, "print state summaries without prompts")
	summarize := flag.Bool("summarize", false, "print a summary table grouping state files by zero and non-zero resource counts")
	flag.BoolVar(summarize, "s", false, "alias for -summarize")
	nonZero := flag.Bool("non-zero", false, "show only state files with more than zero resources")
	flag.BoolVar(nonZero, "nz", false, "alias for -non-zero")
	weighted := flag.Bool("weighted", false, "sort state files by greatest resource count first")
	flag.BoolVar(weighted, "w", false, "alias for -weighted")
	flag.Parse()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		exitErr(fmt.Errorf("resolve root path: %w", err))
	}

	paths, err := state.FindStateFiles(absRoot)
	if err != nil {
		exitErr(err)
	}
	if len(paths) == 0 {
		fmt.Printf("No Terraform state files found under %s\n", absRoot)
		return
	}

	summaries := make([]state.Summary, 0, len(paths))
	for _, p := range paths {
		s, err := state.SummarizeStateFile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", p, err)
			continue
		}
		summaries = append(summaries, s)
	}

	if len(summaries) == 0 {
		fmt.Println("No parseable Terraform state files were found")
		return
	}

	displaySummaries := summaries
	if *nonZero {
		displaySummaries = filterNonZeroSummaries(displaySummaries)
	}
	if *weighted {
		sortSummariesWeighted(displaySummaries)
	}
	if len(displaySummaries) == 0 {
		fmt.Println("No Terraform state files matched the selected filters")
		return
	}

	printTable(os.Stdout, displaySummaries)
	if *summarize {
		fmt.Fprintln(os.Stdout)
		printSummaryTable(os.Stdout, displaySummaries)
	}
	if *nonInteractive {
		return
	}

	runTUI(displaySummaries)
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

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
