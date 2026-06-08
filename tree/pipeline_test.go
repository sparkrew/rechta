package tree

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sparkrew/rechta/models"
	"github.com/sparkrew/rechta/parser"
	"github.com/sparkrew/rechta/resolver"
)

const updateGoldenEnv = "UPDATE_GOLDEN"

func TestPipeline_ResolveAndPrintJSON(t *testing.T) {
	trees := resolvePipelineFixture(t)

	var buf bytes.Buffer
	if err := PrintJSON(trees, &buf); err != nil {
		t.Fatalf("PrintJSON: %v", err)
	}

	assertGolden(t, "dependency-tree.json", normalizeJSON(buf.Bytes()))
}

func TestPipeline_ResolveAndPrintText(t *testing.T) {
	trees := resolvePipelineFixture(t)

	var buf bytes.Buffer
	PrintText(trees, &buf)

	assertGolden(t, "dependency-tree.txt", buf.Bytes())
}

func TestPipeline_ResolveAndPrintHTML(t *testing.T) {
	trees := resolvePipelineFixture(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "report.html")
	if err := PrintHTML(trees, path); err != nil {
		t.Fatalf("PrintHTML: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read HTML: %v", err)
	}
	assertGolden(t, "dependency-tree.html", b)
}

func resolvePipelineFixture(t *testing.T) []resolver.WorkflowTree {
	t.Helper()

	compositeYAML := `name: 'My Composite'
runs:
  using: composite
  steps:
    - uses: actions/cache@v3
    - uses: actions/setup-node@v4
    - run: echo "hello"
`
	checkoutYAML := "name: Checkout\nruns:\n  using: node20\n  main: index.js\n"
	cacheYAML := "name: Cache\nruns:\n  using: node20\n  main: index.js\n"
	setupNodeYAML := "name: Setup Node\nruns:\n  using: node20\n  main: index.js\n"

	compositeSHA := "aaaa000000000000000000000000000000000000"
	checkoutSHA := "dddd000000000000000000000000000000000000"
	cacheSHA := "bbbb000000000000000000000000000000000000"
	setupNodeSHA := "cccc000000000000000000000000000000000000"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/actions/checkout/git/refs/tags/v4":
			fmt.Fprint(w, mockJSON(gitRefResponse{SHA: checkoutSHA, Type: "commit"}))
		case "/repos/actions/checkout/contents/action.yml":
			fmt.Fprint(w, mockFileContent(checkoutYAML))
		case "/repos/org/composite/git/refs/tags/v1":
			fmt.Fprint(w, mockJSON(gitRefResponse{SHA: compositeSHA, Type: "commit"}))
		case "/repos/org/composite/contents/action.yml":
			if r.URL.Query().Get("ref") != compositeSHA {
				http.NotFound(w, r)
				return
			}
			fmt.Fprint(w, mockFileContent(compositeYAML))
		case "/repos/actions/cache/git/refs/tags/v3":
			fmt.Fprint(w, mockJSON(gitRefResponse{SHA: cacheSHA, Type: "commit"}))
		case "/repos/actions/cache/contents/action.yml":
			fmt.Fprint(w, mockFileContent(cacheYAML))
		case "/repos/actions/setup-node/git/refs/tags/v4":
			fmt.Fprint(w, mockJSON(gitRefResponse{SHA: setupNodeSHA, Type: "commit"}))
		case "/repos/actions/setup-node/contents/action.yml":
			fmt.Fprint(w, mockFileContent(setupNodeYAML))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := resolver.NewGitHubClient("test-token", 10)
	client.SetBaseURLForTest(server.URL)

	workflowsDir := filepath.Join("testdata", "workflows")
	files, err := resolver.DiscoverWorkflows(workflowsDir)
	if err != nil {
		t.Fatalf("DiscoverWorkflows: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("want 2 workflow files, got %d", len(files))
	}

	var workflows []*models.Workflow
	for _, f := range files {
		wf, err := parser.ParseWorkflow(f)
		if err != nil {
			t.Fatalf("ParseWorkflow(%s): %v", f, err)
		}
		workflows = append(workflows, wf)
	}

	res := resolver.NewResolver(client, 10, ".")
	trees, err := res.ResolveAll(workflows)
	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if len(trees) != 2 {
		t.Fatalf("want 2 trees, got %d", len(trees))
	}
	return trees
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	goldenPath := filepath.Join("testdata", "golden", name)
	if os.Getenv(updateGoldenEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", goldenPath, err)
		}
		t.Logf("updated golden file %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with %s=1 to create)", goldenPath, err, updateGoldenEnv)
	}

	if string(want) != string(got) {
		t.Errorf("output mismatch for %s\n--- want (%s)\n+++\n got\n---\n%s\n+++\n%s",
			name, goldenPath, string(want), string(got))
	}
}

func normalizeJSON(b []byte) []byte {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return b
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return b
	}
	return append(out, '\n')
}

type gitRefResponse struct {
	SHA  string
	Type string
}

func mockJSON(v any) string {
	switch x := v.(type) {
	case gitRefResponse:
		return fmt.Sprintf(`{"object":{"sha":%q,"type":%q}}`, x.SHA, x.Type)
	default:
		return "{}"
	}
}

func mockFileContent(content string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	return fmt.Sprintf(`{"encoding":"base64","content":%q}`, encoded)
}
