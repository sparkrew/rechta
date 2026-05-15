package resolver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/sparkrew/rechta/models"
	"github.com/sparkrew/rechta/parser"
)

const defaultBaseURL = "https://api.github.com"

var sha40Re = regexp.MustCompile(`^[a-f0-9]{40}$`)

type GitHubClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	sem        chan struct{}
}

func NewGitHubClient(token string, maxConcurrent int) *GitHubClient {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &GitHubClient{
		baseURL:    defaultBaseURL,
		token:      token,
		httpClient: &http.Client{},
		sem:        make(chan struct{}, maxConcurrent),
	}
}

// gitRef is the response from /repos/{owner}/{repo}/git/refs/{type}/{name}.
type gitRef struct {
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

// gitTag is the response from /repos/{owner}/{repo}/git/tags/{sha} for annotated tags.
type gitTag struct {
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

// fileContent is the response from /repos/{owner}/{repo}/contents/{path}.
type fileContent struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// ResolveRef resolves a git ref (tag, branch, or SHA) to a full commit SHA.
// Resolution order: if already a 40-char SHA, return as-is; try tag, then branch.
func (c *GitHubClient) ResolveRef(owner, repo, ref string) (string, error) {
	if sha40Re.MatchString(ref) {
		return ref, nil
	}

	sha, err := c.resolveTag(owner, repo, ref)
	if err == nil {
		return sha, nil
	}
	if isRateLimitError(err) {
		return "", err
	}

	sha, err = c.resolveBranch(owner, repo, ref)
	if err == nil {
		return sha, nil
	}

	return "", fmt.Errorf("could not resolve ref %q for %s/%s", ref, owner, repo)
}

func (c *GitHubClient) resolveTag(owner, repo, tag string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs/tags/%s", c.baseURL, owner, repo, tag)
	var gr gitRef
	if err := c.getJSON(url, &gr); err != nil {
		return "", err
	}
	if gr.Object.Type == "tag" {
		return c.dereferenceAnnotatedTag(owner, repo, gr.Object.SHA)
	}
	return gr.Object.SHA, nil
}

func (c *GitHubClient) dereferenceAnnotatedTag(owner, repo, sha string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/tags/%s", c.baseURL, owner, repo, sha)
	var gt gitTag
	if err := c.getJSON(url, &gt); err != nil {
		return "", err
	}
	return gt.Object.SHA, nil
}

func (c *GitHubClient) resolveBranch(owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", c.baseURL, owner, repo, branch)
	var gr gitRef
	if err := c.getJSON(url, &gr); err != nil {
		return "", err
	}
	return gr.Object.SHA, nil
}

// FetchActionConfig fetches and parses action.yml/action.yaml from a repository at a given SHA.
// For paths ending in .yml/.yaml (reusable workflows), it returns nil so the caller
// can fall through to FetchWorkflowConfig.
func (c *GitHubClient) FetchActionConfig(owner, repo, sha string, path string) (*models.Metadata, error) {
	if strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
		return nil, nil
	}

	for _, filename := range []string{"action.yml", "action.yaml"} {
		filePath := filename
		if path != "" {
			filePath = path + "/" + filename
		}

		data, err := c.fetchFileContent(owner, repo, sha, filePath)
		if err != nil {
			continue
		}

		meta, err := parser.ParseMetadataFromBytes(data, fmt.Sprintf("%s/%s@%s/%s", owner, repo, sha[:12], filePath))
		if err != nil {
			continue
		}
		return meta, nil
	}

	return nil, nil
}

// FetchWorkflowConfig fetches and parses a reusable workflow YAML from a repository.
func (c *GitHubClient) FetchWorkflowConfig(owner, repo, sha, path string) (*models.Workflow, error) {
	data, err := c.fetchFileContent(owner, repo, sha, path)
	if err != nil {
		return nil, err
	}

	wf, err := parser.ParseWorkflowFromBytes(data, fmt.Sprintf("%s/%s@%s/%s", owner, repo, sha[:12], path))
	if err != nil {
		return nil, err
	}
	return wf, nil
}

func (c *GitHubClient) fetchFileContent(owner, repo, sha, path string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", c.baseURL, owner, repo, path, sha)
	var fc fileContent
	if err := c.getJSON(url, &fc); err != nil {
		return nil, err
	}

	if fc.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding %q for %s", fc.Encoding, path)
	}

	cleaned := strings.ReplaceAll(fc.Content, "\n", "")
	data, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("decoding base64 content for %s: %w", path, err)
	}
	return data, nil
}

func (c *GitHubClient) getJSON(url string, dest interface{}) error {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found: %s", url)
	}
	if resp.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "rate limit") {
			return &rateLimitError{msg: "GitHub API rate limit exceeded"}
		}
		return fmt.Errorf("forbidden: %s: %s", url, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s: %s", resp.StatusCode, url, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}

type rateLimitError struct {
	msg string
}

func (e *rateLimitError) Error() string {
	return e.msg
}

func isRateLimitError(err error) bool {
	_, ok := err.(*rateLimitError)
	return ok
}
