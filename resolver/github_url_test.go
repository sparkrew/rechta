package resolver

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/sparkrew/rechta/models"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		input        string
		wantOwner    string
		wantRepo     string
		wantRef      string
		wantWFPath   string
		wantSingle   bool
		wantErr      bool
	}{
		{
			input:     "https://github.com/actions/checkout",
			wantOwner: "actions",
			wantRepo:  "checkout",
		},
		{
			input:     "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			input:     "git@github.com:owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			input:     "https://github.com/owner/repo/tree/main",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRef:   "main",
		},
		{
			input:     "https://github.com/owner/repo/tree/v1.0.0",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRef:   "v1.0.0",
		},
		{
			input:      "https://github.com/owner/repo/tree/main/.github/workflows/ci.yml",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantRef:    "main",
			wantWFPath: ".github/workflows/ci.yml",
			wantSingle: true,
		},
		{
			input:     "https://github.com/owner/repo/tree/feature/my-branch",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRef:   "feature/my-branch",
		},
		{
			input:      "https://github.com/owner/repo/blob/v2.0.0/.github/workflows/deploy.yml",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantRef:    "v2.0.0",
			wantWFPath: ".github/workflows/deploy.yml",
			wantSingle: true,
		},
		{
			input:     "https://github.com/owner/repo/commit/abc123def456abc123def456abc123def456abc1",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRef:   "abc123def456abc123def456abc123def456abc1",
		},
		{
			input:     "https://github.com/owner/repo/releases/tag/v1.2.3",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRef:   "v1.2.3",
		},
		{
			input:   "https://gitlab.com/owner/repo",
			wantErr: true,
		},
		{
			input:   "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseGitHubURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseGitHubURL(%q) expected error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseGitHubURL(%q) error: %v", tt.input, err)
			}
			if got.Owner != tt.wantOwner || got.Repo != tt.wantRepo || got.Ref != tt.wantRef ||
				got.WorkflowPath != tt.wantWFPath || got.IsSingleFile != tt.wantSingle {
				t.Errorf("ParseGitHubURL(%q) = %+v, want owner=%q repo=%q ref=%q path=%q single=%v",
					tt.input, got, tt.wantOwner, tt.wantRepo, tt.wantRef, tt.wantWFPath, tt.wantSingle)
			}
		})
	}
}

func TestLoadWorkflowsFromURL_DefaultBranch(t *testing.T) {
	commitSHA := "aaaa000000000000000000000000000000000000"
	wfYAML := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`

	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/owner/repo": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(repoInfo{DefaultBranch: "main"}))
		},
		"/repos/owner/repo/git/refs/tags/main": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
		"/repos/owner/repo/git/refs/heads/main": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{commitSHA, "commit"}}))
		},
		"/repos/owner/repo/contents/.github/workflows": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("ref") != commitSHA {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			fmt.Fprint(w, jsonMarshal([]dirEntry{
				{Name: "ci.yml", Path: ".github/workflows/ci.yml", Type: "file"},
			}))
		},
		"/repos/owner/repo/contents/.github/workflows/ci.yml": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("ref") != commitSHA {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode(wfYAML),
			}))
		},
	})
	defer server.Close()

	wfs, remote, err := LoadWorkflowsFromURL(client, "https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("LoadWorkflowsFromURL error: %v", err)
	}
	if remote.Owner != "owner" || remote.Repo != "repo" || remote.SHA != commitSHA {
		t.Errorf("remote = %+v", remote)
	}
	if len(wfs) != 1 || wfs[0].Path != ".github/workflows/ci.yml" {
		t.Fatalf("workflows = %+v", wfs)
	}
}

func TestLoadWorkflowsFromURL_SpecificRef(t *testing.T) {
	commitSHA := "bbbb000000000000000000000000000000000000"
	wfYAML := `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`

	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/owner/repo/git/refs/tags/v1.0.0": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{commitSHA, "commit"}}))
		},
		"/repos/owner/repo/contents/.github/workflows": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal([]dirEntry{
				{Name: "ci.yml", Path: ".github/workflows/ci.yml", Type: "file"},
			}))
		},
		"/repos/owner/repo/contents/.github/workflows/ci.yml": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode(wfYAML),
			}))
		},
	})
	defer server.Close()

	wfs, remote, err := LoadWorkflowsFromURL(client, "https://github.com/owner/repo/tree/v1.0.0")
	if err != nil {
		t.Fatalf("LoadWorkflowsFromURL error: %v", err)
	}
	if remote.SHA != commitSHA {
		t.Errorf("remote.SHA = %q, want %q", remote.SHA, commitSHA)
	}
	if len(wfs) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(wfs))
	}
}

func TestResolve_LocalActionRemote(t *testing.T) {
	commitSHA := "cccc000000000000000000000000000000000000"
	actionYAML := `name: Local Composite
runs:
  using: composite
  steps:
    - uses: actions/checkout@v4
`

	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/owner/repo/contents/my-action/action.yml": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("ref") != commitSHA {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode(actionYAML),
			}))
		},
		"/repos/actions/checkout/git/refs/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{"dddd000000000000000000000000000000000000", "commit"}}))
		},
		"/repos/actions/checkout/contents/action.yml": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode("name: Checkout\nruns:\n  using: node20\n  main: index.js\n"),
			}))
		},
	})
	defer server.Close()

	remote := &RemoteRepo{Owner: "owner", Repo: "repo", SHA: commitSHA}
	res := NewResolverWithRemote(client, 10, "", remote)

	wf := &models.Workflow{
		Path: ".github/workflows/ci.yml",
		Jobs: models.Jobs{
			{
				ID: "test",
				Steps: models.Steps{
					{Uses: "./my-action"},
				},
			},
		},
	}

	trees, err := res.ResolveAll([]*models.Workflow{wf})
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	root := trees[0].Dependencies[0]
	if !root.Ref.IsLocal {
		t.Fatal("expected local ref")
	}
	if root.Type != ActionTypeComposite {
		t.Errorf("root.Type = %q, want composite", root.Type)
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.Children))
	}
}
