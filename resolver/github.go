package resolver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sparkrew/rechta/models"
	"github.com/sparkrew/rechta/parser"
)

const defaultBaseURL = "https://api.github.com"

var sha40Re = regexp.MustCompile(`^[a-f0-9]{40}$`)

type GitHubClient struct {
	baseURL         string
	token           string
	httpClient      *http.Client
	sem             chan struct{}
	releaseSHAIndex map[string]map[string]string // owner/repo -> commit SHA -> YYYY-MM-DD
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
	Tagger struct {
		Date string `json:"date"`
	} `json:"tagger"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

// releaseListItem is one entry from GET /repos/{owner}/{repo}/releases.
type releaseListItem struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
}

// fileContent is the response from /repos/{owner}/{repo}/contents/{path}.
type fileContent struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// dirEntry is one item from a directory listing via the Contents API.
type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

// repoInfo is the response from GET /repos/{owner}/{repo}.
type repoInfo struct {
	DefaultBranch   string `json:"default_branch"`
	StargazersCount int    `json:"stargazers_count"`
}

// releaseInfo is the response from GET /repos/{owner}/{repo}/releases/tags/{tag}.
type releaseInfo struct {
	PublishedAt string `json:"published_at"`
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
// For paths ending in .yml/.yaml (reusable workflows), it returns zero values so the caller
// can fall through to FetchWorkflowConfig.
// On success, raw is the exact decoded bytes of the metadata file used and contentRelPath is
// the repository-relative path to that file (forward slashes).
func (c *GitHubClient) FetchActionConfig(owner, repo, sha string, path string) (meta *models.Metadata, raw []byte, contentRelPath string) {
	if strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
		return nil, nil, ""
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

		m, err := parser.ParseMetadataFromBytes(data, fmt.Sprintf("%s/%s@%s/%s", owner, repo, sha[:12], filePath))
		if err != nil {
			continue
		}
		return m, data, filepath.ToSlash(filePath)
	}

	return nil, nil, ""
}

// FetchWorkflowConfig fetches and parses a reusable workflow YAML from a repository.
// raw is the decoded file bytes used for hashing; contentRelPath is repo-relative (forward slashes).
func (c *GitHubClient) FetchWorkflowConfig(owner, repo, sha, path string) (*models.Workflow, []byte, string, error) {
	data, err := c.fetchFileContent(owner, repo, sha, path)
	if err != nil {
		return nil, nil, "", err
	}

	wf, err := parser.ParseWorkflowFromBytes(data, fmt.Sprintf("%s/%s@%s/%s", owner, repo, sha[:12], path))
	if err != nil {
		return nil, nil, filepath.ToSlash(path), err
	}
	return wf, data, filepath.ToSlash(path), nil
}

// GetDefaultBranch returns the repository's default branch name.
func (c *GitHubClient) GetDefaultBranch(owner, repo string) (string, error) {
	info, err := c.getRepoInfo(owner, repo)
	if err != nil {
		return "", err
	}
	if info.DefaultBranch == "" {
		return "", fmt.Errorf("empty default branch for %s/%s", owner, repo)
	}
	return info.DefaultBranch, nil
}

// GetRepoStars returns the repository stargazers count.
func (c *GitHubClient) GetRepoStars(owner, repo string) (int, error) {
	info, err := c.getRepoInfo(owner, repo)
	if err != nil {
		return 0, err
	}
	return info.StargazersCount, nil
}

func (c *GitHubClient) getRepoInfo(owner, repo string) (*repoInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)
	var info repoInfo
	if err := c.getJSON(url, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// CountContributors returns the total number of contributors for a repository.
func (c *GitHubClient) CountContributors(owner, repo string) (int, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contributors?per_page=100", c.baseURL, owner, repo)
	total := 0

	for url != "" {
		var page []json.RawMessage
		headers, err := c.getJSONResponse(url, &page)
		if err != nil {
			return total, err
		}
		total += len(page)
		url = parseNextLink(headers.Get("Link"))
	}

	return total, nil
}

// GetReleasePublishedDate returns the publish date (YYYY-MM-DD) for an exact GitHub Release tag.
func (c *GitHubClient) GetReleasePublishedDate(owner, repo, tag string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.baseURL, owner, repo, tag)
	var rel releaseInfo
	if err := c.getJSON(url, &rel); err != nil {
		return "", err
	}
	return formatPublishedDate(rel.PublishedAt)
}

// GetReleasedOn returns the publish date (YYYY-MM-DD) for an action reference.
// It tries, in order: exact GitHub Release tag, annotated git tag date, then a
// release whose tag resolves to the given commit SHA (covers major-version refs like @v4).
func (c *GitHubClient) GetReleasedOn(owner, repo, ref, commitSHA string) (string, error) {
	if date, err := c.GetReleasePublishedDate(owner, repo, ref); err == nil {
		return date, nil
	}
	if date, err := c.getAnnotatedTagDate(owner, repo, ref); err == nil {
		return date, nil
	}
	if commitSHA != "" {
		if date, err := c.getReleaseDateByCommitSHA(owner, repo, commitSHA); err == nil {
			return date, nil
		}
	}
	return "", fmt.Errorf("no release date found for %s/%s@%s", owner, repo, ref)
}

func formatPublishedDate(publishedAt string) (string, error) {
	if len(publishedAt) < 10 {
		return "", fmt.Errorf("empty or invalid published_at %q", publishedAt)
	}
	return publishedAt[:10], nil
}

func (c *GitHubClient) getAnnotatedTagDate(owner, repo, tag string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs/tags/%s", c.baseURL, owner, repo, tag)
	var gr gitRef
	if err := c.getJSON(url, &gr); err != nil {
		return "", err
	}
	if gr.Object.Type != "tag" {
		return "", fmt.Errorf("%q is not an annotated tag", tag)
	}

	url = fmt.Sprintf("%s/repos/%s/%s/git/tags/%s", c.baseURL, owner, repo, gr.Object.SHA)
	var gt gitTag
	if err := c.getJSON(url, &gt); err != nil {
		return "", err
	}
	return formatPublishedDate(gt.Tagger.Date)
}

func (c *GitHubClient) getReleaseDateByCommitSHA(owner, repo, sha string) (string, error) {
	if c.releaseSHAIndex == nil {
		c.releaseSHAIndex = make(map[string]map[string]string)
	}
	repoKey := owner + "/" + repo
	if _, ok := c.releaseSHAIndex[repoKey]; !ok {
		index, err := c.buildReleaseSHAIndex(owner, repo)
		if err != nil {
			return "", err
		}
		c.releaseSHAIndex[repoKey] = index
	}
	if date, ok := c.releaseSHAIndex[repoKey][sha]; ok {
		return date, nil
	}
	return "", fmt.Errorf("no release for commit %s", sha[:12])
}

func (c *GitHubClient) buildReleaseSHAIndex(owner, repo string) (map[string]string, error) {
	index := make(map[string]string)
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", c.baseURL, owner, repo)

	for url != "" {
		var page []releaseListItem
		headers, err := c.getJSONResponse(url, &page)
		if err != nil {
			return index, err
		}
		for _, rel := range page {
			if rel.Draft || rel.TagName == "" || rel.PublishedAt == "" {
				continue
			}
			tagSHA, err := c.ResolveRef(owner, repo, rel.TagName)
			if err != nil {
				continue
			}
			if _, exists := index[tagSHA]; exists {
				continue
			}
			date, err := formatPublishedDate(rel.PublishedAt)
			if err != nil {
				continue
			}
			index[tagSHA] = date
		}
		url = parseNextLink(headers.Get("Link"))
	}

	return index, nil
}

func parseNextLink(link string) string {
	if link == "" {
		return ""
	}
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

// ListYAMLFiles lists .yml and .yaml files in a directory at a given ref SHA.
func (c *GitHubClient) ListYAMLFiles(owner, repo, sha, dir string) ([]string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", c.baseURL, owner, repo, dir, sha)
	var entries []dirEntry
	if err := c.getJSON(url, &entries); err != nil {
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		if strings.HasSuffix(e.Name, ".yml") || strings.HasSuffix(e.Name, ".yaml") {
			paths = append(paths, e.Path)
		}
	}
	return paths, nil
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
	_, err := c.getJSONResponse(url, dest)
	return err
}

func (c *GitHubClient) getJSONResponse(url string, dest interface{}) (http.Header, error) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found: %s", url)
	}
	if resp.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "rate limit") {
			return nil, &rateLimitError{msg: "GitHub API rate limit exceeded"}
		}
		return nil, fmt.Errorf("forbidden: %s: %s", url, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s: %s", resp.StatusCode, url, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return nil, err
	}
	return resp.Header, nil
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
