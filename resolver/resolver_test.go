package resolver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sparkrew/rechta/models"
)

// ---------------------------------------------------------------------------
// ParseActionRef tests
// ---------------------------------------------------------------------------

func TestParseActionRef(t *testing.T) {
	tests := []struct {
		input   string
		want    ActionRef
		wantErr bool
	}{
		{
			input: "actions/checkout@v4",
			want:  ActionRef{Owner: "actions", Repo: "checkout", Ref: "v4", RawUses: "actions/checkout@v4"},
		},
		{
			input: "aws-actions/configure-aws-credentials@v4",
			want:  ActionRef{Owner: "aws-actions", Repo: "configure-aws-credentials", Ref: "v4", RawUses: "aws-actions/configure-aws-credentials@v4"},
		},
		{
			input: "org/repo/.github/workflows/ci.yml@main",
			want:  ActionRef{Owner: "org", Repo: "repo", Path: ".github/workflows/ci.yml", Ref: "main", RawUses: "org/repo/.github/workflows/ci.yml@main"},
		},
		{
			input: "owner/repo@b4ffde65f46336ab88eb53be808477a3936bae11",
			want:  ActionRef{Owner: "owner", Repo: "repo", Ref: "b4ffde65f46336ab88eb53be808477a3936bae11", RawUses: "owner/repo@b4ffde65f46336ab88eb53be808477a3936bae11"},
		},
		{input: "./local/action", wantErr: true},
		{input: "no-at-sign", wantErr: true},
		{input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseActionRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseActionRef(%q) expected error, got %+v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseActionRef(%q) unexpected error: %v", tt.input, err)
			}
			if got.Owner != tt.want.Owner || got.Repo != tt.want.Repo || got.Path != tt.want.Path || got.Ref != tt.want.Ref || got.RawUses != tt.want.RawUses {
				t.Errorf("ParseActionRef(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ShouldSkipRef tests
// ---------------------------------------------------------------------------

func TestShouldSkipRef(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"./local/action", true},
		{"docker://alpine:3.18", true},
		{"actions/checkout@v4", false},
		{"org/repo@main", false},
	}
	for _, tt := range tests {
		if got := ShouldSkipRef(tt.input); got != tt.want {
			t.Errorf("ShouldSkipRef(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ExtractActionRefs tests
// ---------------------------------------------------------------------------

func TestExtractActionRefs(t *testing.T) {
	wf := &models.Workflow{
		Jobs: models.Jobs{
			{
				ID:   "build",
				Uses: "org/reusable/.github/workflows/build.yml@main",
			},
			{
				ID: "test",
				Steps: models.Steps{
					{Uses: "actions/checkout@v4"},
					{Uses: "actions/setup-go@v5"},
					{Uses: "actions/checkout@v4"}, // duplicate
					{Uses: "./local/action"},       // should be skipped
					{Uses: "docker://alpine:3.18"}, // should be skipped
					{Run: "go test ./..."},          // no uses
				},
			},
		},
	}

	refs := ExtractActionRefs(wf)

	expected := []string{
		"org/reusable/.github/workflows/build.yml@main",
		"actions/checkout@v4",
		"actions/setup-go@v5",
	}

	if len(refs) != len(expected) {
		t.Fatalf("ExtractActionRefs returned %d refs, want %d: %v", len(refs), len(expected), refs)
	}

	for i, ref := range refs {
		if ref.RawUses != expected[i] {
			t.Errorf("ref[%d].RawUses = %q, want %q", i, ref.RawUses, expected[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Mock GitHub server helpers
// ---------------------------------------------------------------------------

func b64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func jsonMarshal(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// newMockGitHub creates an httptest server that serves canned responses.
// routes maps URL path → handler function.
func newMockGitHub(t *testing.T, routes map[string]http.HandlerFunc) (*httptest.Server, *GitHubClient) {
	t.Helper()

	mux := http.NewServeMux()
	for path, handler := range routes {
		mux.HandleFunc(path, handler)
	}

	server := httptest.NewServer(mux)
	client := NewGitHubClient("test-token", 10)
	client.baseURL = server.URL

	return server, client
}

// ---------------------------------------------------------------------------
// ResolveRef tests
// ---------------------------------------------------------------------------

func TestResolveRef_Tag(t *testing.T) {
	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/actions/checkout/git/refs/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{"abc123def456abc123def456abc123def456abc1", "commit"}}))
		},
	})
	defer server.Close()

	sha, err := client.ResolveRef("actions", "checkout", "v4")
	if err != nil {
		t.Fatalf("ResolveRef error: %v", err)
	}
	if sha != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("ResolveRef = %q, want abc123...", sha)
	}
}

func TestResolveRef_AnnotatedTag(t *testing.T) {
	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/actions/checkout/git/refs/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{"tag_sha_000000000000000000000000000000000", "tag"}}))
		},
		"/repos/actions/checkout/git/tags/tag_sha_000000000000000000000000000000000": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitTag{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{"real_commit_sha_00000000000000000000000000", "commit"}}))
		},
	})
	defer server.Close()

	sha, err := client.ResolveRef("actions", "checkout", "v4")
	if err != nil {
		t.Fatalf("ResolveRef error: %v", err)
	}
	if sha != "real_commit_sha_00000000000000000000000000" {
		t.Errorf("ResolveRef = %q, want real_commit_sha...", sha)
	}
}

func TestResolveRef_Branch(t *testing.T) {
	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/org/repo/git/refs/tags/main": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"/repos/org/repo/git/refs/heads/main": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{"branch_sha_0000000000000000000000000000000", "commit"}}))
		},
	})
	defer server.Close()

	sha, err := client.ResolveRef("org", "repo", "main")
	if err != nil {
		t.Fatalf("ResolveRef error: %v", err)
	}
	if sha != "branch_sha_0000000000000000000000000000000" {
		t.Errorf("ResolveRef = %q, want branch_sha...", sha)
	}
}

func TestResolveRef_SHA(t *testing.T) {
	client := NewGitHubClient("", 10)
	sha, err := client.ResolveRef("any", "repo", "b4ffde65f46336ab88eb53be808477a3936bae11")
	if err != nil {
		t.Fatalf("ResolveRef error: %v", err)
	}
	if sha != "b4ffde65f46336ab88eb53be808477a3936bae11" {
		t.Errorf("ResolveRef = %q, want input SHA", sha)
	}
}

// ---------------------------------------------------------------------------
// Full resolution tests with composite action
// ---------------------------------------------------------------------------

func TestResolve_CompositeAction(t *testing.T) {
	actionYAML := `name: 'My Composite'
description: 'A composite action'
runs:
  using: composite
  steps:
    - uses: actions/cache@v3
    - uses: actions/setup-node@v4
    - run: echo "hello"
`

	commitSHA := "aaaa000000000000000000000000000000000000"
	cacheSHA := "bbbb000000000000000000000000000000000000"
	nodeSHA := "cccc000000000000000000000000000000000000"

	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/org/composite/git/refs/tags/v1": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{commitSHA, "commit"}}))
		},
		"/repos/org/composite/contents/action.yml": func(w http.ResponseWriter, r *http.Request) {
			ref := r.URL.Query().Get("ref")
			if ref != commitSHA {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode(actionYAML),
			}))
		},
		// Transitive dep: actions/cache@v3
		"/repos/actions/cache/git/refs/tags/v3": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{cacheSHA, "commit"}}))
		},
		"/repos/actions/cache/contents/action.yml": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode("name: Cache\nruns:\n  using: node20\n  main: index.js\n"),
			}))
		},
		// Transitive dep: actions/setup-node@v4
		"/repos/actions/setup-node/git/refs/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{nodeSHA, "commit"}}))
		},
		"/repos/actions/setup-node/contents/action.yml": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode("name: Setup Node\nruns:\n  using: node20\n  main: index.js\n"),
			}))
		},
	})
	defer server.Close()

	resolver := NewResolver(client, 10)
	wf := &models.Workflow{
		Path: ".github/workflows/test.yml",
		Jobs: models.Jobs{
			{
				ID: "test",
				Steps: models.Steps{
					{Uses: "org/composite@v1"},
				},
			},
		},
	}

	trees, err := resolver.ResolveAll([]*models.Workflow{wf})
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	if len(trees) != 1 {
		t.Fatalf("expected 1 tree, got %d", len(trees))
	}

	tree := trees[0]
	if tree.Path != ".github/workflows/test.yml" {
		t.Errorf("tree.Path = %q", tree.Path)
	}
	if len(tree.Dependencies) != 1 {
		t.Fatalf("expected 1 root dep, got %d", len(tree.Dependencies))
	}

	root := tree.Dependencies[0]
	if root.SHA != commitSHA {
		t.Errorf("root SHA = %q, want %q", root.SHA, commitSHA)
	}
	if root.Type != ActionTypeComposite {
		t.Errorf("root Type = %q, want composite", root.Type)
	}
	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(root.Children))
	}

	if root.Children[0].Ref.RawUses != "actions/cache@v3" {
		t.Errorf("child[0] = %q", root.Children[0].Ref.RawUses)
	}
	if root.Children[0].SHA != cacheSHA {
		t.Errorf("child[0] SHA = %q", root.Children[0].SHA)
	}
	if root.Children[1].Ref.RawUses != "actions/setup-node@v4" {
		t.Errorf("child[1] = %q", root.Children[1].Ref.RawUses)
	}
}

// ---------------------------------------------------------------------------
// Deduplication test
// ---------------------------------------------------------------------------

func TestResolve_Deduplication(t *testing.T) {
	callCount := 0
	checkoutSHA := "dddd000000000000000000000000000000000000"

	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/actions/checkout/git/refs/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			callCount++
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{checkoutSHA, "commit"}}))
		},
		"/repos/actions/checkout/contents/action.yml": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode("name: Checkout\nruns:\n  using: node20\n  main: index.js\n"),
			}))
		},
	})
	defer server.Close()

	resolver := NewResolver(client, 10)
	wf := &models.Workflow{
		Path: "test.yml",
		Jobs: models.Jobs{
			{
				ID: "job1",
				Steps: models.Steps{
					{Uses: "actions/checkout@v4"},
				},
			},
			{
				ID: "job2",
				Steps: models.Steps{
					{Uses: "actions/checkout@v4"},
				},
			},
		},
	}

	trees, err := resolver.ResolveAll([]*models.Workflow{wf})
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	// ExtractActionRefs deduplicates, so only 1 dep
	if len(trees[0].Dependencies) != 1 {
		t.Fatalf("expected 1 dep (deduped), got %d", len(trees[0].Dependencies))
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call for tag resolution, got %d", callCount)
	}
}

// ---------------------------------------------------------------------------
// Depth limit test
// ---------------------------------------------------------------------------

func TestResolve_DepthLimit(t *testing.T) {
	// Create a chain: action-0 → action-1 → action-2 → ... → action-N
	// Each is composite and references the next.
	const depth = 12

	routes := make(map[string]http.HandlerFunc)
	for i := 0; i < depth; i++ {
		idx := i
		sha := fmt.Sprintf("%040d", idx)

		routes[fmt.Sprintf("/repos/org/action-%d/git/refs/tags/v1", idx)] = func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{sha, "commit"}}))
		}

		var yamlContent string
		if idx < depth-1 {
			yamlContent = fmt.Sprintf("name: Action %d\nruns:\n  using: composite\n  steps:\n    - uses: org/action-%d@v1\n", idx, idx+1)
		} else {
			yamlContent = fmt.Sprintf("name: Action %d\nruns:\n  using: node20\n  main: index.js\n", idx)
		}

		content := yamlContent
		routes[fmt.Sprintf("/repos/org/action-%d/contents/action.yml", idx)] = func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode(content),
			}))
		}
	}

	server, client := newMockGitHub(t, routes)
	defer server.Close()

	resolver := NewResolver(client, 3)
	wf := &models.Workflow{
		Path: "test.yml",
		Jobs: models.Jobs{
			{
				ID:    "test",
				Steps: models.Steps{{Uses: "org/action-0@v1"}},
			},
		},
	}

	_, err := resolver.ResolveAll([]*models.Workflow{wf})
	if err == nil {
		t.Fatal("expected depth limit error, got nil")
	}
	if !strings.Contains(err.Error(), "max dependency depth") {
		t.Errorf("expected depth limit error, got: %v", err)
	}
}
