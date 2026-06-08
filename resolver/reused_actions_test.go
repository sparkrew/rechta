package resolver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	if got[0].Ref.RawUses != "actions/checkout@v4" {
		t.Errorf("got[0].Ref.RawUses = %q, want actions/checkout@v4", got[0].Ref.RawUses)
	}
	if got[0].SHA != checkoutSHA {
		t.Errorf("got[0].SHA = %q, want %q", got[0].SHA, checkoutSHA)
	}
	if got[1].Ref.RawUses != "actions/setup-node@v4" {
		t.Errorf("got[1].Ref.RawUses = %q, want actions/setup-node@v4", got[1].Ref.RawUses)
	}
}

func TestEnrichReusedActions(t *testing.T) {
	repoCalls := 0
	contribCalls := 0
	releaseCalls := make(map[string]int)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/actions/checkout":
			repoCalls++
			fmt.Fprint(w, `{"stargazers_count":5800}`)
		case r.URL.Path == "/repos/actions/checkout/contributors":
			contribCalls++
			if strings.Contains(r.URL.RawQuery, "page=2") {
				fmt.Fprint(w, `[{"login":"c3"}]`)
				return
			}
			next := fmt.Sprintf(`<%s/repos/actions/checkout/contributors?per_page=100&page=2>; rel="next"`, server.URL)
			w.Header().Set("Link", next)
			fmt.Fprint(w, `[{"login":"c1"},{"login":"c2"}]`)
		case r.URL.Path == "/repos/actions/checkout/releases/tags/v4":
			releaseCalls["v4"]++
			fmt.Fprint(w, `{"published_at":"2019-08-08T16:45:33Z"}`)
		case r.URL.Path == "/repos/actions/checkout/releases/tags/v3":
			releaseCalls["v3"]++
			fmt.Fprint(w, `{"published_at":"2018-12-04T10:00:00Z"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGitHubClient("test-token", 10)
	client.baseURL = server.URL

	refs := []ResolvedActionRef{
		{Ref: ActionRef{Owner: "actions", Repo: "checkout", Ref: "v4", RawUses: "actions/checkout@v4"}},
		{Ref: ActionRef{Owner: "actions", Repo: "checkout", Ref: "v3", RawUses: "actions/checkout@v3"}},
	}

	actions, err := EnrichReusedActions(client, refs)
	if err != nil {
		t.Fatalf("EnrichReusedActions error: %v", err)
	}

	if repoCalls != 1 {
		t.Errorf("repo API calls = %d, want 1 (cached)", repoCalls)
	}
	if contribCalls != 2 {
		t.Errorf("contributor API calls = %d, want 2 (paginated once)", contribCalls)
	}
	if releaseCalls["v4"] != 1 || releaseCalls["v3"] != 1 {
		t.Errorf("release calls = %v, want v4=1 v3=1", releaseCalls)
	}

	if len(actions) != 2 {
		t.Fatalf("want 2 actions, got %d", len(actions))
	}

	want := []ReusedAction{
		{Uses: "actions/checkout@v4", Contributors: 3, Stars: 5800, ReleasedOn: "2019-08-08"},
		{Uses: "actions/checkout@v3", Contributors: 3, Stars: 5800, ReleasedOn: "2018-12-04"},
	}
	for i, w := range want {
		if actions[i] != w {
			t.Errorf("actions[%d] = %+v, want %+v", i, actions[i], w)
		}
	}
}

func TestEnrichReusedActions_MissingReleaseOmitsReleasedOn(t *testing.T) {
	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/actions/checkout": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"stargazers_count":100}`)
		},
		"/repos/actions/checkout/contributors": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"login":"c1"}]`)
		},
		"/repos/actions/checkout/releases/tags/main": func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		},
	})
	defer server.Close()

	refs := []ResolvedActionRef{
		{Ref: ActionRef{Owner: "actions", Repo: "checkout", Ref: "main", RawUses: "actions/checkout@main"}},
	}
	actions, err := EnrichReusedActions(client, refs)
	if err != nil {
		t.Fatalf("EnrichReusedActions error: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("want 1 action, got %d", len(actions))
	}
	if actions[0].ReleasedOn != "" {
		t.Errorf("ReleasedOn = %q, want empty", actions[0].ReleasedOn)
	}

	b, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "released_on") {
		t.Errorf("JSON should omit released_on, got %s", string(b))
	}
}

func TestGetReleasedOn_ByCommitSHA(t *testing.T) {
	commitSHA := "cccccccccccccccccccccccccccccccccccccccc"

	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/actions/checkout/releases/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		},
		"/repos/actions/checkout/git/refs/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{commitSHA, "commit"}}))
		},
		"/repos/actions/checkout/releases": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"tag_name":"v4.2.2","published_at":"2024-03-15T12:00:00Z","draft":false}]`)
		},
		"/repos/actions/checkout/git/refs/tags/v4.2.2": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, jsonMarshal(gitRef{Object: struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			}{commitSHA, "commit"}}))
		},
	})
	defer server.Close()

	got, err := client.GetReleasedOn("actions", "checkout", "v4", commitSHA)
	if err != nil {
		t.Fatalf("GetReleasedOn error: %v", err)
	}
	if got != "2024-03-15" {
		t.Errorf("got %q, want 2024-03-15", got)
	}
}

func TestGetReleasePublishedDate(t *testing.T) {
	server, client := newMockGitHub(t, map[string]http.HandlerFunc{
		"/repos/actions/checkout/releases/tags/v4": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"published_at":"2019-08-08T16:45:33Z"}`)
		},
	})
	defer server.Close()

	got, err := client.GetReleasePublishedDate("actions", "checkout", "v4")
	if err != nil {
		t.Fatalf("GetReleasePublishedDate error: %v", err)
	}
	if got != "2019-08-08" {
		t.Errorf("got %q, want 2019-08-08", got)
	}
}

func TestCountContributors_Pagination(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "page=2") {
			fmt.Fprint(w, `[{"login":"u3"}]`)
			return
		}
		next := fmt.Sprintf(`<%s/repos/o/r/contributors?per_page=100&page=2>; rel="next"`, server.URL)
		w.Header().Set("Link", next)
		fmt.Fprint(w, `[{"login":"u1"},{"login":"u2"}]`)
	}))
	defer server.Close()

	client := NewGitHubClient("token", 10)
	client.baseURL = server.URL

	count, err := client.CountContributors("o", "r")
	if err != nil {
		t.Fatalf("CountContributors error: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestGetJSONResponse_ReturnsLinkHeader(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://example.com/next>; rel="next"`)
		fmt.Fprint(w, `[]`)
	}))
	defer server.Close()

	client := NewGitHubClient("token", 10)
	client.baseURL = server.URL

	var dest []json.RawMessage
	headers, err := client.getJSONResponse(server.URL+"/repos/o/r/contributors", &dest)
	if err != nil {
		t.Fatalf("getJSONResponse error: %v", err)
	}
	if got := headers.Get("Link"); got == "" {
		t.Fatal("expected Link header in response")
	}
	if got := parseNextLink(headers.Get("Link")); got != "https://example.com/next" {
		t.Errorf("parseNextLink() = %q", got)
	}
}

func TestParseNextLink(t *testing.T) {
	link := `<https://api.github.com/repos/o/r/contributors?page=2>; rel="next", <https://api.github.com/repos/o/r/contributors?page=5>; rel="last"`
	got := parseNextLink(link)
	want := "https://api.github.com/repos/o/r/contributors?page=2"
	if got != want {
		t.Errorf("parseNextLink() = %q, want %q", got, want)
	}
	if parseNextLink("") != "" {
		t.Error("parseNextLink empty should return empty")
	}
}
