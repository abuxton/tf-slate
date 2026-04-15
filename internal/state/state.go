package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Summary struct {
	Path             string
	ResourceCount    int
	Providers        []string
	TerraformVersion string
	Serial           int
	Lineage          string
}

type rawState struct {
	TerraformVersion string        `json:"terraform_version"`
	Serial           int           `json:"serial"`
	Lineage          string        `json:"lineage"`
	Resources        []rawResource `json:"resources"`
	Values           *rawValues    `json:"values"`
}

type rawResource struct {
	Mode         string            `json:"mode"`
	Provider     string            `json:"provider"`
	ProviderName string            `json:"provider_name"`
	Instances    []json.RawMessage `json:"instances"`
}

type rawValues struct {
	RootModule *rawModule `json:"root_module"`
}

type rawModule struct {
	Resources []rawResource `json:"resources"`
	Child     []rawModule   `json:"child_modules"`
}

func FindStateFiles(root string) ([]string, error) {
	root = filepath.Clean(root)
	var found []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tfstate") {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk filesystem: %w", err)
	}
	sort.Strings(found)
	return found, nil
}

func SummarizeStateFile(path string) (Summary, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, fmt.Errorf("read state file %s: %w", path, err)
	}

	var s rawState
	if err := json.Unmarshal(content, &s); err != nil {
		return Summary{}, fmt.Errorf("parse state file %s: %w", path, err)
	}

	resources := s.Resources
	if len(resources) == 0 && s.Values != nil && s.Values.RootModule != nil {
		resources = flattenResources(*s.Values.RootModule)
	}

	providerSet := map[string]struct{}{}
	resourceCount := 0
	for _, r := range resources {
		if r.Mode == "data" {
			continue
		}
		count := len(r.Instances)
		if count == 0 {
			count = 1
		}
		resourceCount += count

		name := providerName(r.Provider)
		if name == "" {
			name = providerName(r.ProviderName)
		}
		if name != "" {
			providerSet[name] = struct{}{}
		}
	}

	providers := make([]string, 0, len(providerSet))
	for p := range providerSet {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	return Summary{
		Path:             path,
		ResourceCount:    resourceCount,
		Providers:        providers,
		TerraformVersion: s.TerraformVersion,
		Serial:           s.Serial,
		Lineage:          s.Lineage,
	}, nil
}

func flattenResources(m rawModule) []rawResource {
	all := append([]rawResource{}, m.Resources...)
	for _, child := range m.Child {
		all = append(all, flattenResources(child)...)
	}
	return all
}

func providerName(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return ""
	}
	if strings.Contains(provider, "\"") {
		parts := strings.Split(provider, "\"")
		if len(parts) >= 2 {
			provider = parts[1]
		}
	}
	provider = strings.TrimPrefix(provider, "registry.terraform.io/")
	segments := strings.Split(provider, "/")
	if len(segments) == 0 {
		return provider
	}
	return segments[len(segments)-1]
}
