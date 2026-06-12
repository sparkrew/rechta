package resolver

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/sparkrew/rechta/models"
)

func TestUniqueExternalActions_ExcludesLocalAndSorts(t *testing.T) {
	tmpDir := t.TempDir()

	actionDir := filepath.Join(tmpDir, "my-action")
	if err := os.MkdirAll(actionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	actionYAML := []byte(`name: Composite
runs:
  using: composite
  steps:
    - uses: actions/setup-node@v4
`)
	if err := os.WriteFile(filepath.Join(actionDir, "action.yml"), actionYAML, 0o644); err != nil {
		t.Fatal(err)
	}

	checkoutSHA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	setupSHA := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/actions/checkout/git/refs/tags/v4": func(w http.ResponseWriter, r *http.Request) {
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
		"/repos/actions/setup-node/git/refs/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{setupSHA, "commit"}}))
		},
		"/repos/actions/setup-node/contents/action.yml": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(fileContent{
				Encoding: "base64",
				Content:  b64Encode("name: Setup Node\nruns:\n  using: node20\n  main: index.js\n"),
			}))
		},
	})
	defer server.Close()

	res := NewResolver(client, 10, tmpDir)
	wf := &models.Workflow{
		Path: ".github/workflows/ci.yml",
		Jobs: models.Jobs{{
			ID:    "build",
			Steps: models.Steps{{Uses: "actions/checkout@v4"}, {Uses: "./my-action"}},
		}},
	}

	if _, err := res.ResolveAll([]*models.Workflow{wf}); err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	got := res.UniqueExternalActions()
	if len(got) != 2 {
		t.Fatalf("want 2 external actions, got %d: %+v", len(got), got)
	}
	if got[0] != "actions/checkout@v4" {
		t.Errorf("got[0] = %q, want actions/checkout@v4", got[0])
	}
	if got[1] != "actions/setup-node@v4" {
		t.Errorf("got[1] = %q, want actions/setup-node@v4", got[1])
	}
}
