package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/abuxton/tf-slate/internal/state"
)

func main() {
	root := flag.String("root", ".", "root path to scan for .tfstate files")
	nonInteractive := flag.Bool("non-interactive", false, "print state summaries without prompts")
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

	printTable(summaries)
	if *nonInteractive {
		return
	}

	runTUI(summaries)
}

func printTable(summaries []state.Summary) {
	fmt.Println("Found Terraform state files:")
	fmt.Println("#  Resources  Providers         Terraform  Path")
	for i, s := range summaries {
		providers := "-"
		if len(s.Providers) > 0 {
			providers = strings.Join(s.Providers, ",")
		}
		version := s.TerraformVersion
		if version == "" {
			version = "-"
		}
		fmt.Printf("%-2d %-10d %-17s %-10s %s\n", i+1, s.ResourceCount, providers, version, s.Path)
	}
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
