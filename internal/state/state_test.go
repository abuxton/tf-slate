package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindStateFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a", "terraform.tfstate"), `{}`)
	mustWrite(t, filepath.Join(dir, "b", "nested.tfstate"), `{}`)
	mustWrite(t, filepath.Join(dir, "b", "nope.txt"), `x`)

	files, err := FindStateFiles(dir)
	if err != nil {
		t.Fatalf("FindStateFiles() error = %v", err)
	}

	want := []string{
		filepath.Join(dir, "a", "terraform.tfstate"),
		filepath.Join(dir, "b", "nested.tfstate"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("FindStateFiles() = %v, want %v", files, want)
	}
}

func TestSummarizeStateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "terraform.tfstate")
	mustWrite(t, path, `{
  "terraform_version":"1.6.6",
  "serial":7,
  "lineage":"abc",
  "resources":[
    {
      "mode":"managed",
      "provider":"provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances":[{},{}]
    },
    {
      "mode":"managed",
      "provider_name":"registry.terraform.io/hashicorp/random",
      "instances":[{}]
    },
    {
      "mode":"data",
      "provider":"provider[\"registry.terraform.io/hashicorp/null\"]",
      "instances":[{}]
    }
  ]
}`)

	summary, err := SummarizeStateFile(path)
	if err != nil {
		t.Fatalf("SummarizeStateFile() error = %v", err)
	}
	if summary.ResourceCount != 3 {
		t.Fatalf("ResourceCount = %d, want 3", summary.ResourceCount)
	}
	wantProviders := []string{"aws", "random"}
	if !reflect.DeepEqual(summary.Providers, wantProviders) {
		t.Fatalf("Providers = %v, want %v", summary.Providers, wantProviders)
	}
	if summary.TerraformVersion != "1.6.6" {
		t.Fatalf("TerraformVersion = %q, want %q", summary.TerraformVersion, "1.6.6")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
