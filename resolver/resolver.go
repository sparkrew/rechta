package resolver

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sparkrew/rechta/models"
	"github.com/sparkrew/rechta/parser"
)

const DefaultMaxDepth = 10

var actionRefRegex = regexp.MustCompile(`^([^/]+)/([^/@]+)(?:/([^@]+))?@(.+)$`)

// ActionRef represents a parsed GitHub Actions reference from a uses: field.
type ActionRef struct {
	Owner     string `json:"owner,omitempty"`
	Repo      string `json:"repo,omitempty"`
	Path      string `json:"path,omitempty"`
	Ref       string `json:"ref,omitempty"`
	RawUses   string `json:"uses"`
	IsLocal   bool   `json:"is_local,omitempty"`
	LocalPath string `json:"local_path,omitempty"`
}

// FullName returns the action's canonical name (owner/repo or owner/repo/path).
// For local refs it returns the local path.
func (r ActionRef) FullName() string {
	if r.IsLocal {
		return r.LocalPath
	}
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
	Ref            ActionRef         `json:"ref"`
	SHA            string            `json:"sha"`
	Type           ActionType        `json:"type"`
	Children       []*DependencyNode `json:"dependencies,omitempty"`
	AlreadyVisited bool              `json:"already_visited"`
	ContentSHA256  string            `json:"content_sha256,omitempty"`
	ContentPath    string            `json:"content_path,omitempty"`
}

func sha256Hex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
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
	basePath string
}

// NewResolver creates a resolver with the given GitHub client, depth limit,
// and base path for resolving local action references.
func NewResolver(client *GitHubClient, maxDepth int, basePath string) *Resolver {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	return &Resolver{
		client:   client,
		visited:  make(map[string]*DependencyNode),
		maxDepth: maxDepth,
		basePath: basePath,
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
			ContentSHA256:  existing.ContentSHA256,
			ContentPath:    existing.ContentPath,
		}, nil
	}

	if ref.IsLocal {
		return r.resolveLocal(ref, depth)
	}

	fmt.Fprintf(os.Stderr, "  Resolving %s@%s...\n", ref.FullName(), ref.Ref)

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

	deps, actionType, cHash, cPath, err := r.findTransitiveDeps(ref, sha)
	if err != nil {
		node.Type = ActionTypeUnknown
		fmt.Fprintf(os.Stderr, "    Resolved %s -> %s (type: unknown, error fetching config: %v)\n", ref.FullName(), sha[:12], err)
		return node, nil
	}
	node.Type = actionType
	if cHash != "" {
		node.ContentSHA256 = cHash
		node.ContentPath = cPath
	}
	fmt.Fprintf(os.Stderr, "    Resolved %s -> %s (type: %s)\n", ref.FullName(), sha[:12], actionType)

	for _, depRef := range deps {
		child, err := r.resolve(depRef, depth+1)
		if err != nil {
			return nil, fmt.Errorf("resolving transitive dep %s of %s: %w", depRef.RawUses, ref.RawUses, err)
		}
		node.Children = append(node.Children, child)
	}

	return node, nil
}

func (r *Resolver) resolveLocal(ref ActionRef, depth int) (*DependencyNode, error) {
	if r.basePath == "" {
		fmt.Fprintf(os.Stderr, "  Skipping local %s (no repo context)\n", ref.LocalPath)
		node := &DependencyNode{
			Ref:  ref,
			Type: ActionTypeUnknown,
		}
		r.visited[ref.RawUses] = node
		return node, nil
	}

	fmt.Fprintf(os.Stderr, "  Resolving local %s...\n", ref.LocalPath)

	node := &DependencyNode{
		Ref:  ref,
		Type: ActionTypeUnknown,
	}

	r.visited[ref.RawUses] = node

	actionDir := filepath.Join(r.basePath, ref.LocalPath)

	var meta *models.Metadata
	var raw []byte
	var contentRelPath string

	localRoot := filepath.ToSlash(strings.TrimPrefix(ref.LocalPath, "./"))
	for _, filename := range []string{"action.yml", "action.yaml"} {
		absMeta := filepath.Join(actionDir, filename)
		data, readErr := os.ReadFile(absMeta)
		if readErr != nil {
			continue
		}
		m, parseErr := parser.ParseMetadataFromBytes(data, absMeta)
		if parseErr != nil {
			continue
		}
		meta = m
		raw = data
		if localRoot == "" || localRoot == "." {
			contentRelPath = filepath.ToSlash(filename)
		} else {
			contentRelPath = filepath.ToSlash(filepath.Join(localRoot, filename))
		}
		break
	}

	if meta == nil {
		fmt.Fprintf(os.Stderr, "    Resolved %s (type: unknown, could not read action metadata)\n", ref.LocalPath)
		return node, nil
	}

	node.ContentSHA256 = sha256Hex(raw)
	node.ContentPath = contentRelPath

	using := strings.ToLower(meta.Runs.Using)

	switch {
	case using == "composite":
		node.Type = ActionTypeComposite
		for _, step := range meta.Runs.Steps {
			if step.Uses == "" || ShouldSkipRef(step.Uses) {
				continue
			}
			var depRef ActionRef
			if IsLocalRef(step.Uses) {
				depRef = ParseLocalRef(step.Uses)
			} else {
				parsed, parseErr := ParseActionRef(step.Uses)
				if parseErr != nil {
					continue
				}
				depRef = parsed
			}
			child, childErr := r.resolve(depRef, depth+1)
			if childErr != nil {
				return nil, fmt.Errorf("resolving transitive dep %s of %s: %w", depRef.RawUses, ref.RawUses, childErr)
			}
			node.Children = append(node.Children, child)
		}
	case strings.HasPrefix(using, "node"):
		node.Type = ActionTypeNode
	case using == "docker":
		node.Type = ActionTypeDocker
	}

	fmt.Fprintf(os.Stderr, "    Resolved %s (type: %s)\n", ref.LocalPath, node.Type)
	return node, nil
}

func (r *Resolver) findTransitiveDeps(ref ActionRef, sha string) ([]ActionRef, ActionType, string, string, error) {
	isReusableWorkflow := strings.HasSuffix(ref.Path, ".yml") || strings.HasSuffix(ref.Path, ".yaml")

	if isReusableWorkflow {
		wf, raw, relPath, err := r.client.FetchWorkflowConfig(ref.Owner, ref.Repo, sha, ref.Path)
		if err != nil {
			return nil, ActionTypeReusable, "", "", err
		}
		deps := ExtractActionRefs(wf)
		hash := sha256Hex(raw)
		return deps, ActionTypeReusable, hash, relPath, nil
	}

	meta, raw, relPath := r.client.FetchActionConfig(ref.Owner, ref.Repo, sha, ref.Path)
	if meta == nil {
		return nil, ActionTypeUnknown, "", "", nil
	}

	hash := sha256Hex(raw)

	using := strings.ToLower(meta.Runs.Using)

	switch {
	case using == "composite":
		var deps []ActionRef
		for _, step := range meta.Runs.Steps {
			if step.Uses == "" || ShouldSkipRef(step.Uses) {
				continue
			}
			if IsLocalRef(step.Uses) {
				deps = append(deps, ParseLocalRef(step.Uses))
			} else {
				parsed, err := ParseActionRef(step.Uses)
				if err != nil {
					continue
				}
				deps = append(deps, parsed)
			}
		}
		return deps, ActionTypeComposite, hash, relPath, nil

	case strings.HasPrefix(using, "node"):
		return nil, ActionTypeNode, hash, relPath, nil

	case using == "docker":
		return nil, ActionTypeDocker, hash, relPath, nil

	default:
		return nil, ActionTypeUnknown, hash, relPath, nil
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

// ParseLocalRef creates an ActionRef for a local path reference (./path).
func ParseLocalRef(uses string) ActionRef {
	return ActionRef{
		RawUses:   uses,
		IsLocal:   true,
		LocalPath: uses,
	}
}

// ShouldSkipRef returns true if the uses: reference should be skipped
// (docker:// references).
func ShouldSkipRef(uses string) bool {
	return strings.HasPrefix(uses, "docker://")
}

// IsLocalRef returns true if the uses: reference points to a local action (./path).
func IsLocalRef(uses string) bool {
	return strings.HasPrefix(uses, "./")
}

// ExtractActionRefs extracts all unique action references from a workflow,
// including local (./path) references.
func ExtractActionRefs(wf *models.Workflow) []ActionRef {
	seen := make(map[string]bool)
	var refs []ActionRef

	for _, job := range wf.Jobs {
		// Job-level uses (reusable workflow calls)
		if job.Uses != "" && !ShouldSkipRef(job.Uses) && !seen[job.Uses] {
			seen[job.Uses] = true
			if IsLocalRef(job.Uses) {
				refs = append(refs, ParseLocalRef(job.Uses))
			} else {
				parsed, err := ParseActionRef(job.Uses)
				if err == nil {
					refs = append(refs, parsed)
				}
			}
		}

		// Step-level uses
		for _, step := range job.Steps {
			if step.Uses == "" || ShouldSkipRef(step.Uses) || seen[step.Uses] {
				continue
			}
			seen[step.Uses] = true
			if IsLocalRef(step.Uses) {
				refs = append(refs, ParseLocalRef(step.Uses))
			} else {
				parsed, err := ParseActionRef(step.Uses)
				if err == nil {
					refs = append(refs, parsed)
				}
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
