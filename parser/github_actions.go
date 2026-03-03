package parser

import (
	"fmt"
	"os"

	"github.com/sparkrew/rechta/models"
	"gopkg.in/yaml.v3"
)

// ParseWorkflow reads a GitHub Actions workflow YAML file and returns a
// parsed Workflow. The returned Workflow.Path is set to filePath.
func ParseWorkflow(filePath string) (*models.Workflow, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file: %w", err)
	}
	return ParseWorkflowFromBytes(data, filePath)
}

// ParseWorkflowFromBytes parses a GitHub Actions workflow from raw YAML bytes.
// path is stored in the returned Workflow.Path for downstream consumers.
func ParseWorkflowFromBytes(data []byte, path string) (*models.Workflow, error) {
	var w models.Workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("unmarshaling workflow YAML: %w", err)
	}
	w.Path = path

	if !w.IsValid() {
		return nil, fmt.Errorf("invalid workflow %q: must have at least one event and one job", path)
	}
	return &w, nil
}

// ParseMetadata reads a GitHub Actions action.yml/action.yaml metadata file
// and returns a parsed Metadata. The returned Metadata.Path is set to filePath.
func ParseMetadata(filePath string) (*models.Metadata, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading metadata file: %w", err)
	}
	return ParseMetadataFromBytes(data, filePath)
}

// ParseMetadataFromBytes parses a GitHub Actions action metadata from raw YAML
// bytes. path is stored in the returned Metadata.Path for downstream consumers.
func ParseMetadataFromBytes(data []byte, path string) (*models.Metadata, error) {
	var m models.Metadata
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata YAML: %w", err)
	}
	m.Path = path

	if !m.IsValid() {
		return nil, fmt.Errorf("invalid action metadata %q: runs.using must be set", path)
	}
	return &m, nil
}
