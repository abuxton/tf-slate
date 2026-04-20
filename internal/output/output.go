package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/abuxton/tf-slate/internal/state"
)

type Format string

const (
	FormatString Format = "string"
	FormatJSON   Format = "json"
	FormatYAML   Format = "yaml"
	FormatCSV    Format = "csv"
)

type summaryRecord struct {
	Path             string   `json:"path"`
	ResourceCount    int      `json:"resource_count"`
	Providers        []string `json:"providers"`
	TerraformVersion string   `json:"terraform_version"`
	Serial           int      `json:"serial"`
	Lineage          string   `json:"lineage"`
}

func ParseFormat(value string) (Format, error) {
	format := Format(strings.ToLower(strings.TrimSpace(value)))
	switch format {
	case FormatString, FormatJSON, FormatYAML, FormatCSV:
		return format, nil
	default:
		return "", fmt.Errorf("unsupported output format %q (expected string, json, yaml, or csv)", value)
	}
}

func Write(w io.Writer, format Format, summaries []state.Summary) error {
	records := toRecords(summaries)

	switch format {
	case FormatString:
		writeString(w, records)
		return nil
	case FormatJSON:
		return writeJSON(w, records)
	case FormatYAML:
		return writeYAML(w, records)
	case FormatCSV:
		return writeCSV(w, records)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func toRecords(summaries []state.Summary) []summaryRecord {
	records := make([]summaryRecord, 0, len(summaries))
	for _, summary := range summaries {
		records = append(records, summaryRecord{
			Path:             summary.Path,
			ResourceCount:    summary.ResourceCount,
			Providers:        append([]string(nil), summary.Providers...),
			TerraformVersion: summary.TerraformVersion,
			Serial:           summary.Serial,
			Lineage:          summary.Lineage,
		})
	}
	return records
}

func writeString(w io.Writer, records []summaryRecord) {
	fmt.Fprintln(w, "Found Terraform state files:")
	fmt.Fprintln(w, "#  Resources  Providers         Terraform  Path")
	for i, record := range records {
		providers := "-"
		if len(record.Providers) > 0 {
			providers = strings.Join(record.Providers, ",")
		}
		version := record.TerraformVersion
		if version == "" {
			version = "-"
		}
		fmt.Fprintf(w, "%-2d %-10d %-17s %-10s %s\n", i+1, record.ResourceCount, providers, version, record.Path)
	}
}

func writeJSON(w io.Writer, records []summaryRecord) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(records)
}

func writeYAML(w io.Writer, records []summaryRecord) error {
	for i, record := range records {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "- path:", yamlString(record.Path))
		fmt.Fprintln(w, "  resource_count:", record.ResourceCount)
		if len(record.Providers) == 0 {
			fmt.Fprintln(w, "  providers: []")
		} else {
			fmt.Fprintln(w, "  providers:")
			for _, provider := range record.Providers {
				fmt.Fprintln(w, "    -", yamlString(provider))
			}
		}
		fmt.Fprintln(w, "  terraform_version:", yamlString(record.TerraformVersion))
		fmt.Fprintln(w, "  serial:", record.Serial)
		fmt.Fprintln(w, "  lineage:", yamlString(record.Lineage))
	}
	return nil
}

func yamlString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func writeCSV(w io.Writer, records []summaryRecord) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{
		"path",
		"resource_count",
		"providers",
		"terraform_version",
		"serial",
		"lineage",
	}); err != nil {
		return err
	}

	for _, record := range records {
		if err := writer.Write([]string{
			record.Path,
			strconv.Itoa(record.ResourceCount),
			strings.Join(record.Providers, ","),
			record.TerraformVersion,
			strconv.Itoa(record.Serial),
			record.Lineage,
		}); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}
