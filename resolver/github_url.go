package resolver

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/sparkrew/rechta/models"
)

const defaultWorkflowsDir = ".github/workflows"

// RemoteRepo identifies a GitHub repository at a pinned commit SHA.
// When set on a Resolver, local action references are fetched from this repo.
type RemoteRepo struct {
	Owner string
	Repo  string
	SHA   string
}

// GitHubURL is the parsed result of a github.com repository URL.
type GitHubURL struct {
	Owner        string
	Repo         string
	Ref          string // empty = repository default branch
	WorkflowPath string // repo-relative path; empty = all workflows in .github/workflows
	IsSingleFile bool
}

// ParseGitHubURL parses a GitHub repository URL into owner, repo, ref, and
// optional workflow path. Supported forms include:
//
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo/tree/ref
//   - https://github.com/owner/repo/tree/ref/.github/workflows/ci.yml
//   - https://github.com/owner/repo/blob/ref/.github/workflows/ci.yml
//   - https://github.com/owner/repo/commit/sha
//   - https://github.com/owner/repo/releases/tag/v1.0.0
//   - git@github.com:owner/repo.git
func ParseGitHubURL(rawURL string) (GitHubURL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return GitHubURL{}, fmt.Errorf("empty URL")
	}

	if strings.HasPrefix(rawURL, "git@github.com:") {
		path := strings.TrimPrefix(rawURL, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return GitHubURL{}, fmt.Errorf("invalid git SSH URL: %q", rawURL)
		}
		return GitHubURL{Owner: parts[0], Repo: parts[1]}, nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return GitHubURL{}, fmt.Errorf("parsing URL: %w", err)
	}

	host := strings.ToLower(u.Host)
	if host != "github.com" && host != "www.github.com" {
		return GitHubURL{}, fmt.Errorf("unsupported host %q (only github.com is supported)", u.Host)
	}

	path := strings.Trim(u.Path, "/")
	if path == "" {
		return GitHubURL{}, fmt.Errorf("missing owner/repo in URL")
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return GitHubURL{}, fmt.Errorf("invalid GitHub URL path: %q", u.Path)
	}

	parsed := GitHubURL{
		Owner: parts[0],
		Repo:  strings.TrimSuffix(parts[1], ".git"),
	}

	if len(parts) == 2 {
		return parsed, nil
	}

	switch parts[2] {
	case "tree":
		return parseTreeURL(parsed, parts[3:])
	case "blob":
		return parseBlobURL(parsed, parts[3:])
	case "commit":
		if len(parts) < 4 || parts[3] == "" {
			return GitHubURL{}, fmt.Errorf("missing commit SHA in URL")
		}
		parsed.Ref = parts[3]
		return parsed, nil
	case "releases":
		if len(parts) < 5 || parts[3] != "tag" || parts[4] == "" {
			return GitHubURL{}, fmt.Errorf("invalid releases URL (expected /releases/tag/name)")
		}
		parsed.Ref = strings.Join(parts[4:], "/")
		return parsed, nil
	default:
		return GitHubURL{}, fmt.Errorf("unsupported GitHub URL path: %q", u.Path)
	}
}

func parseTreeURL(parsed GitHubURL, remainder []string) (GitHubURL, error) {
	if len(remainder) == 0 {
		return GitHubURL{}, fmt.Errorf("missing ref in tree URL")
	}

	full := strings.Join(remainder, "/")
	if wfPath, ref, ok := splitRefAndWorkflowPath(full); ok {
		parsed.Ref = ref
		parsed.WorkflowPath = wfPath
		parsed.IsSingleFile = strings.HasSuffix(wfPath, ".yml") || strings.HasSuffix(wfPath, ".yaml")
		return parsed, nil
	}

	parsed.Ref = full
	return parsed, nil
}

func parseBlobURL(parsed GitHubURL, remainder []string) (GitHubURL, error) {
	if len(remainder) < 2 {
		return GitHubURL{}, fmt.Errorf("invalid blob URL (expected /blob/ref/path)")
	}

	parsed.Ref = remainder[0]
	parsed.WorkflowPath = strings.Join(remainder[1:], "/")
	parsed.IsSingleFile = true

	if !strings.HasSuffix(parsed.WorkflowPath, ".yml") && !strings.HasSuffix(parsed.WorkflowPath, ".yaml") {
		return GitHubURL{}, fmt.Errorf("blob URL must point to a .yml or .yaml workflow file")
	}

	return parsed, nil
}

// splitRefAndWorkflowPath splits "ref/.github/workflows/ci.yml" or
// "ref/.github/workflows" into ref and workflow path when the suffix is
// clearly a workflow file or directory.
func splitRefAndWorkflowPath(full string) (workflowPath, ref string, ok bool) {
	const marker = ".github/workflows"
	idx := strings.Index(full, marker)
	if idx < 0 {
		return "", "", false
	}

	if idx == 0 {
		return "", "", false
	}

	if full[idx-1] != '/' {
		return "", "", false
	}

	ref = full[:idx-1]
	workflowPath = full[idx:]
	if workflowPath == marker {
		return workflowPath, ref, true
	}

	if strings.HasSuffix(workflowPath, ".yml") || strings.HasSuffix(workflowPath, ".yaml") {
		return workflowPath, ref, true
	}

	return "", "", false
}

// LoadWorkflowsFromURL fetches workflow YAML from a GitHub repository URL.
// When the URL has no ref, the repository default branch is used.
func LoadWorkflowsFromURL(client *GitHubClient, rawURL string) ([]*models.Workflow, *RemoteRepo, error) {
	parsed, err := ParseGitHubURL(rawURL)
	if err != nil {
		return nil, nil, err
	}

	ref := parsed.Ref
	if ref == "" {
		ref, err = client.GetDefaultBranch(parsed.Owner, parsed.Repo)
		if err != nil {
			return nil, nil, fmt.Errorf("getting default branch for %s/%s: %w", parsed.Owner, parsed.Repo, err)
		}
	}

	sha, err := client.ResolveRef(parsed.Owner, parsed.Repo, ref)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving ref %q for %s/%s: %w", ref, parsed.Owner, parsed.Repo, err)
	}

	remote := &RemoteRepo{
		Owner: parsed.Owner,
		Repo:  parsed.Repo,
		SHA:   sha,
	}

	if parsed.IsSingleFile {
		wf, err := fetchRemoteWorkflow(client, parsed.Owner, parsed.Repo, sha, parsed.WorkflowPath)
		if err != nil {
			return nil, nil, err
		}
		return []*models.Workflow{wf}, remote, nil
	}

	dir := defaultWorkflowsDir
	if parsed.WorkflowPath != "" {
		dir = parsed.WorkflowPath
	}

	paths, err := client.ListYAMLFiles(parsed.Owner, parsed.Repo, sha, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("listing workflows in %s@%s: %w", dir, ref, err)
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("no workflow files found in %s/%s at ref %q", parsed.Owner, parsed.Repo, ref)
	}

	var workflows []*models.Workflow
	for _, path := range paths {
		wf, err := fetchRemoteWorkflow(client, parsed.Owner, parsed.Repo, sha, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", path, err)
			continue
		}
		workflows = append(workflows, wf)
	}

	if len(workflows) == 0 {
		return nil, nil, fmt.Errorf("no valid workflow files found in %s/%s at ref %q", parsed.Owner, parsed.Repo, ref)
	}

	return workflows, remote, nil
}

func fetchRemoteWorkflow(client *GitHubClient, owner, repo, sha, path string) (*models.Workflow, error) {
	wf, _, _, err := client.FetchWorkflowConfig(owner, repo, sha, path)
	if err != nil {
		return nil, fmt.Errorf("fetching workflow %s: %w", path, err)
	}
	wf.Path = path
	return wf, nil
}
