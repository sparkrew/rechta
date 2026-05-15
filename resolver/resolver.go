package resolver

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sparkrew/rechta/models"
)

const DefaultMaxDepth = 10

var actionRefRegex = regexp.MustCompile(`^([^/]+)/([^/@]+)(?:/([^@]+))?@(.+)$`)

// ActionRef represents a parsed GitHub Actions reference from a uses: field.
type ActionRef struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Path    string `json:"path,omitempty"`
	Ref     string `json:"ref"`
	RawUses string `json:"uses"`
}

// FullName returns the action's canonical name (owner/repo or owner/repo/path).
func (r ActionRef) FullName() string {
	if r.Path != "" {
		return fmt.Sprintf("%s/%s/%s", r.Owner, r.Repo, r.Path)
	}
	return fmt.Sprintf("%s/%s", r.Owner, r.Repo)
}

// ActionType classifies the kind of action detected during resolution.
type ActionType string

const (
	ActionTypeNode      ActionType = "node"
	ActionTypeComposite ActionType = "composite"
	ActionTypeDocker    ActionType = "docker"
	ActionTypeReusable  ActionType = "reusable-workflow"
	ActionTypeUnknown   ActionType = "unknown"
)

// DependencyNode represents a node in the resolved dependency tree.
type DependencyNode struct {
	Ref            ActionRef        `json:"ref"`
	SHA            string           `json:"sha"`
	Type           ActionType       `json:"type"`
	Children       []*DependencyNode `json:"dependencies,omitempty"`
	AlreadyVisited bool             `json:"already_visited,omitempty"`
}

// WorkflowTree groups the dependency tree for a single workflow file.
type WorkflowTree struct {
	Path         string            `json:"path"`
	Dependencies []*DependencyNode `json:"dependencies"`
}

// Resolver performs recursive GitHub Actions dependency resolution.
type Resolver struct {
	client   *GitHubClient
	visited  map[string]*DependencyNode
	maxDepth int
}

// NewResolver creates a resolver with the given GitHub client and depth limit.
func NewResolver(client *GitHubClient, maxDepth int) *Resolver {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	return &Resolver{
		client:   client,
		visited:  make(map[string]*DependencyNode),
		maxDepth: maxDepth,
	}
}

// ResolveAll resolves all action dependencies across multiple workflow files.
// Returns one WorkflowTree per workflow, each containing the direct and transitive deps.
func (r *Resolver) ResolveAll(workflows []*models.Workflow) ([]WorkflowTree, error) {
	var trees []WorkflowTree

	for _, wf := range workflows {
		refs := ExtractActionRefs(wf)
		var nodes []*DependencyNode

		for _, ref := range refs {
			node, err := r.resolve(ref, 0)
			if err != nil {
				return nil, fmt.Errorf("resolving %s: %w", ref.RawUses, err)
			}
			nodes = append(nodes, node)
		}

		trees = append(trees, WorkflowTree{
			Path:         wf.Path,
			Dependencies: nodes,
		})
	}

	return trees, nil
}

func (r *Resolver) resolve(ref ActionRef, depth int) (*DependencyNode, error) {
	if depth > r.maxDepth {
		return nil, fmt.Errorf("max dependency depth %d exceeded", r.maxDepth)
	}

	if existing, ok := r.visited[ref.RawUses]; ok {
		return &DependencyNode{
			Ref:            existing.Ref,
			SHA:            existing.SHA,
			Type:           existing.Type,
			AlreadyVisited: true,
		}, nil
	}

	fmt.Printf("  Resolving %s@%s...\n", ref.FullName(), ref.Ref)

	sha, err := r.client.ResolveRef(ref.Owner, ref.Repo, ref.Ref)
	if err != nil {
		return nil, err
	}

	node := &DependencyNode{
		Ref:  ref,
		SHA:  sha,
		Type: ActionTypeUnknown,
	}

	r.visited[ref.RawUses] = node

	deps, actionType, err := r.findTransitiveDeps(ref, sha)
	if err != nil {
		node.Type = ActionTypeUnknown
		fmt.Printf("    Resolved %s → %s (type: unknown, error fetching config: %v)\n", ref.FullName(), sha[:12], err)
		return node, nil
	}
	node.Type = actionType
	fmt.Printf("    Resolved %s → %s (type: %s)\n", ref.FullName(), sha[:12], actionType)

	for _, depRef := range deps {
		child, err := r.resolve(depRef, depth+1)
		if err != nil {
			return nil, fmt.Errorf("resolving transitive dep %s of %s: %w", depRef.RawUses, ref.RawUses, err)
		}
		node.Children = append(node.Children, child)
	}

	return node, nil
}

func (r *Resolver) findTransitiveDeps(ref ActionRef, sha string) ([]ActionRef, ActionType, error) {
	isReusableWorkflow := strings.HasSuffix(ref.Path, ".yml") || strings.HasSuffix(ref.Path, ".yaml")

	if isReusableWorkflow {
		wf, err := r.client.FetchWorkflowConfig(ref.Owner, ref.Repo, sha, ref.Path)
		if err != nil {
			return nil, ActionTypeReusable, err
		}
		deps := ExtractActionRefs(wf)
		return deps, ActionTypeReusable, nil
	}

	meta, err := r.client.FetchActionConfig(ref.Owner, ref.Repo, sha, ref.Path)
	if err != nil {
		return nil, ActionTypeUnknown, err
	}
	if meta == nil {
		return nil, ActionTypeUnknown, nil
	}

	using := strings.ToLower(meta.Runs.Using)

	switch {
	case using == "composite":
		var deps []ActionRef
		for _, step := range meta.Runs.Steps {
			if step.Uses == "" || ShouldSkipRef(step.Uses) {
				continue
			}
			parsed, err := ParseActionRef(step.Uses)
			if err != nil {
				continue
			}
			deps = append(deps, parsed)
		}
		return deps, ActionTypeComposite, nil

	case strings.HasPrefix(using, "node"):
		return nil, ActionTypeNode, nil

	case using == "docker":
		return nil, ActionTypeDocker, nil

	default:
		return nil, ActionTypeUnknown, nil
	}
}

// ParseActionRef parses a GitHub Actions uses: string into an ActionRef.
func ParseActionRef(uses string) (ActionRef, error) {
	match := actionRefRegex.FindStringSubmatch(uses)
	if match == nil {
		return ActionRef{}, fmt.Errorf("invalid action reference format: %q", uses)
	}
	return ActionRef{
		Owner:   match[1],
		Repo:    match[2],
		Path:    match[3],
		Ref:     match[4],
		RawUses: uses,
	}, nil
}

// ShouldSkipRef returns true if the uses: reference should be skipped
// (local actions and docker:// references).
func ShouldSkipRef(uses string) bool {
	return strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://")
}

// ExtractActionRefs extracts all unique external action references from a workflow.
func ExtractActionRefs(wf *models.Workflow) []ActionRef {
	seen := make(map[string]bool)
	var refs []ActionRef

	for _, job := range wf.Jobs {
		// Job-level uses (reusable workflow calls)
		if job.Uses != "" && !ShouldSkipRef(job.Uses) && !seen[job.Uses] {
			seen[job.Uses] = true
			parsed, err := ParseActionRef(job.Uses)
			if err == nil {
				refs = append(refs, parsed)
			}
		}

		// Step-level uses
		for _, step := range job.Steps {
			if step.Uses == "" || ShouldSkipRef(step.Uses) || seen[step.Uses] {
				continue
			}
			seen[step.Uses] = true
			parsed, err := ParseActionRef(step.Uses)
			if err == nil {
				refs = append(refs, parsed)
			}
		}
	}

	return refs
}

// DiscoverWorkflows finds all .yml and .yaml files in a directory (non-recursive).
func DiscoverWorkflows(dir string) ([]string, error) {
	ymlFiles, err := filepath.Glob(filepath.Join(dir, "*.yml"))
	if err != nil {
		return nil, err
	}
	yamlFiles, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	return append(ymlFiles, yamlFiles...), nil
}
