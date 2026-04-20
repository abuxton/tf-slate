package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/abuxton/tf-slate/internal/state"
)

func TestParseFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    Format
		wantErr bool
	}{
		{name: "string", input: "string", want: FormatString},
		{name: "json upper", input: "JSON", want: FormatJSON},
		{name: "yaml spaced", input: " yaml ", want: FormatYAML},
		{name: "csv", input: "csv", want: FormatCSV},
		{name: "invalid", input: "toml", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseFormat(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFormat(%q) error = %v, want nil", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteString(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Write(&buf, FormatString, sampleSummaries()); err != nil {
		t.Fatalf("Write(string) error = %v, want nil", err)
	}

	output := buf.String()
	for _, want := range []string{
		"Found Terraform state files:",
		"#  Resources  Providers         Terraform  Path",
		"aws,random",
		"terraform.tfstate",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Write(string) output missing %q in %q", want, output)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Write(&buf, FormatJSON, sampleSummaries()); err != nil {
		t.Fatalf("Write(json) error = %v, want nil", err)
	}

	output := buf.String()
	for _, want := range []string{
		`"path": "terraform.tfstate"`,
		`"resource_count": 2`,
		`"providers": [`,
		`"terraform_version": "1.6.0"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Write(json) output missing %q in %q", want, output)
		}
	}
}

func TestWriteYAML(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Write(&buf, FormatYAML, sampleSummaries()); err != nil {
		t.Fatalf("Write(yaml) error = %v, want nil", err)
	}

	output := buf.String()
	for _, want := range []string{
		`- path: "terraform.tfstate"`,
		`  resource_count: 2`,
		`  providers:`,
		`    - "aws"`,
		`  terraform_version: "1.6.0"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Write(yaml) output missing %q in %q", want, output)
		}
	}
}

func TestWriteCSV(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Write(&buf, FormatCSV, sampleSummaries()); err != nil {
		t.Fatalf("Write(csv) error = %v, want nil", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("Write(csv) line count = %d, want 2 in %q", len(lines), buf.String())
	}
	if got := lines[0]; got != "path,resource_count,providers,terraform_version,serial,lineage" {
		t.Fatalf("Write(csv) header = %q, want %q", got, "path,resource_count,providers,terraform_version,serial,lineage")
	}
	if !strings.Contains(lines[1], `"aws,random"`) {
		t.Fatalf("Write(csv) row missing providers field in %q", lines[1])
	}
}

func sampleSummaries() []state.Summary {
	return []state.Summary{
		{
			Path:             "terraform.tfstate",
			ResourceCount:    2,
			Providers:        []string{"aws", "random"},
			TerraformVersion: "1.6.0",
			Serial:           7,
			Lineage:          "lineage-1",
		},
	}
}
