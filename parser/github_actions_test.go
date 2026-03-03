package parser

import (
	"path/filepath"
	"testing"

	"github.com/sparkrew/rechta/models"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

// ---------------------------------------------------------------------------
// ParseWorkflow – file-based
// ---------------------------------------------------------------------------

func TestParseWorkflow_ValidWorkflow(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("valid_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "CI Pipeline" {
		t.Fatalf("expected name 'CI Pipeline', got %q", w.Name)
	}
	if w.Path != testdataPath("valid_workflow.yml") {
		t.Fatalf("expected path set, got %q", w.Path)
	}
	if !w.IsValid() {
		t.Fatal("workflow should be valid")
	}

	// Events
	if len(w.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(w.Events))
	}
	if w.Events[0].Name != "push" {
		t.Fatalf("expected first event 'push', got %q", w.Events[0].Name)
	}
	if len(w.Events[0].Branches) != 2 {
		t.Fatalf("expected 2 push branches, got %d", len(w.Events[0].Branches))
	}
	if len(w.Events[0].Tags) != 1 || w.Events[0].Tags[0] != "v*" {
		t.Fatalf("unexpected push tags: %v", w.Events[0].Tags)
	}
	if w.Events[1].Name != "pull_request" {
		t.Fatalf("expected second event 'pull_request', got %q", w.Events[1].Name)
	}

	// Permissions
	if len(w.Permissions) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(w.Permissions))
	}

	// Env
	if len(w.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(w.Env))
	}

	// Jobs
	if len(w.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(w.Jobs))
	}

	build := w.Jobs[0]
	if build.ID != "build" {
		t.Fatalf("expected job id 'build', got %q", build.ID)
	}
	if len(build.RunsOn) != 1 || build.RunsOn[0] != "ubuntu-latest" {
		t.Fatalf("unexpected runs-on: %v", build.RunsOn)
	}
	if len(build.Steps) != 3 {
		t.Fatalf("expected 3 build steps, got %d", len(build.Steps))
	}
	if build.Steps[0].Uses != "actions/checkout@v4" {
		t.Fatalf("expected checkout step, got %q", build.Steps[0].Uses)
	}
	if build.Steps[0].Action != "actions/checkout" {
		t.Fatalf("expected action 'actions/checkout', got %q", build.Steps[0].Action)
	}
	if build.Steps[2].Shell != "bash" {
		t.Fatalf("expected shell 'bash', got %q", build.Steps[2].Shell)
	}

	test := w.Jobs[1]
	if test.ID != "test" {
		t.Fatalf("expected job id 'test', got %q", test.ID)
	}
	if len(test.Needs) != 1 || test.Needs[0] != "build" {
		t.Fatalf("unexpected needs: %v", test.Needs)
	}
	if len(test.RunsOn) != 2 {
		t.Fatalf("expected 2 runs-on labels, got %d", len(test.RunsOn))
	}
	if test.Container.Image != "golang:1.22" {
		t.Fatalf("expected container golang:1.22, got %q", test.Container.Image)
	}
	if len(test.Environment) != 1 || test.Environment[0].Name != "staging" {
		t.Fatalf("unexpected environment: %v", test.Environment)
	}
	if test.Environment[0].Url != "https://staging.example.com" {
		t.Fatalf("unexpected env url: %q", test.Environment[0].Url)
	}
	if test.Steps[1].WorkingDirectory != "./src" {
		t.Fatalf("expected working-directory './src', got %q", test.Steps[1].WorkingDirectory)
	}
}

func TestParseWorkflow_MatrixWorkflow(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("matrix_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Matrix Build" {
		t.Fatalf("expected name 'Matrix Build', got %q", w.Name)
	}
	if len(w.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(w.Jobs))
	}

	j := w.Jobs[0]
	if len(j.Strategy.Matrix) != 2 {
		t.Fatalf("expected 2 matrix dimensions, got %d", len(j.Strategy.Matrix))
	}
	if len(j.Strategy.Matrix["os"]) != 3 {
		t.Fatalf("expected 3 os values, got %d", len(j.Strategy.Matrix["os"]))
	}
	if len(j.Strategy.Matrix["go"]) != 2 {
		t.Fatalf("expected 2 go versions, got %d", len(j.Strategy.Matrix["go"]))
	}
}

func TestParseWorkflow_ReusableWorkflow(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("reusable_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}

	if len(w.Events) != 1 || w.Events[0].Name != "workflow_call" {
		t.Fatalf("expected workflow_call event, got %v", w.Events)
	}
	wc := w.Events[0]
	if len(wc.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(wc.Inputs))
	}
	if wc.Inputs[0].Name != "environment" || !wc.Inputs[0].Required {
		t.Fatalf("unexpected input[0]: %+v", wc.Inputs[0])
	}
	if wc.Inputs[1].Name != "ref" || wc.Inputs[1].Required {
		t.Fatalf("unexpected input[1]: %+v", wc.Inputs[1])
	}
	if len(wc.Outputs) != 1 || wc.Outputs[0].Name != "deploy_url" {
		t.Fatalf("unexpected outputs: %v", wc.Outputs)
	}
	if len(wc.Secrets) != 1 || wc.Secrets[0].Name != "deploy_token" {
		t.Fatalf("unexpected secrets: %v", wc.Secrets)
	}

	if len(w.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(w.Jobs))
	}
	j := w.Jobs[0]
	if len(j.Outputs) != 1 || j.Outputs[0].Name != "url" {
		t.Fatalf("unexpected job outputs: %v", j.Outputs)
	}
	if j.Steps[0].WithRef != "${{ inputs.ref }}" {
		t.Fatalf("expected with.ref from inputs, got %q", j.Steps[0].WithRef)
	}
}

func TestParseWorkflow_WorkflowRun(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("workflow_run.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Post-CI" {
		t.Fatalf("expected name 'Post-CI', got %q", w.Name)
	}
	if len(w.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(w.Events))
	}
	wr := w.Events[0]
	if wr.Name != "workflow_run" {
		t.Fatalf("expected workflow_run, got %q", wr.Name)
	}
	if len(wr.Workflows) != 2 || wr.Workflows[0] != "CI Pipeline" || wr.Workflows[1] != "Nightly" {
		t.Fatalf("unexpected workflows: %v", wr.Workflows)
	}
	if len(wr.Branches) != 1 || wr.Branches[0] != "main" {
		t.Fatalf("unexpected branches: %v", wr.Branches)
	}
	if len(wr.Types) != 1 || wr.Types[0] != "completed" {
		t.Fatalf("unexpected types: %v", wr.Types)
	}
}

func TestParseWorkflow_ComplexWorkflow(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("complex_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Full Featured" {
		t.Fatalf("expected name 'Full Featured', got %q", w.Name)
	}

	// 4 events: push, pull_request, schedule, workflow_dispatch
	if len(w.Events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(w.Events))
	}
	names := make(map[string]bool)
	for _, e := range w.Events {
		names[e.Name] = true
	}
	for _, want := range []string{"push", "pull_request", "schedule", "workflow_dispatch"} {
		if !names[want] {
			t.Fatalf("missing event %q", want)
		}
	}

	// Push event path filters
	for _, e := range w.Events {
		if e.Name == "push" {
			if len(e.Paths) != 2 {
				t.Fatalf("expected 2 push paths, got %d", len(e.Paths))
			}
			break
		}
	}

	// Schedule cron
	for _, e := range w.Events {
		if e.Name == "schedule" {
			if len(e.Cron) != 1 || e.Cron[0] != "0 0 * * *" {
				t.Fatalf("unexpected cron: %v", e.Cron)
			}
			break
		}
	}

	// Permissions: read-all expands to all scopes
	if len(w.Permissions) != len(models.AllScopes) {
		t.Fatalf("expected %d permissions (read-all), got %d", len(models.AllScopes), len(w.Permissions))
	}

	// 5 jobs
	if len(w.Jobs) != 5 {
		t.Fatalf("expected 5 jobs, got %d", len(w.Jobs))
	}

	jobByID := make(map[string]models.Job)
	for _, j := range w.Jobs {
		jobByID[j.ID] = j
	}

	// lint job
	lint := jobByID["lint"]
	if len(lint.Permissions) != 1 || lint.Permissions[0].Scope != "contents" {
		t.Fatalf("unexpected lint permissions: %v", lint.Permissions)
	}

	// build job depends on lint
	build := jobByID["build"]
	if len(build.Needs) != 1 || build.Needs[0] != "lint" {
		t.Fatalf("unexpected build needs: %v", build.Needs)
	}
	if len(build.Outputs) != 1 || build.Outputs[0].Name != "image_tag" {
		t.Fatalf("unexpected build outputs: %v", build.Outputs)
	}

	// test job has strategy matrix
	test := jobByID["test"]
	if len(test.Strategy.Matrix["go"]) != 2 {
		t.Fatalf("expected 2 go versions in test matrix, got %d", len(test.Strategy.Matrix["go"]))
	}
	if test.Container.Image != "golang:1.22" {
		t.Fatalf("unexpected test container: %q", test.Container.Image)
	}

	// deploy job depends on build and test
	deploy := jobByID["deploy"]
	if len(deploy.Needs) != 2 {
		t.Fatalf("expected 2 deploy needs, got %d", len(deploy.Needs))
	}
	if len(deploy.Environment) != 1 || deploy.Environment[0].Name != "production" {
		t.Fatalf("unexpected deploy environment: %v", deploy.Environment)
	}
	if len(deploy.Permissions) != 2 {
		t.Fatalf("expected 2 deploy permissions, got %d", len(deploy.Permissions))
	}

	// call-reusable job uses a reusable workflow
	reusable := jobByID["call-reusable"]
	if reusable.Uses != "org/repo/.github/workflows/shared.yml@main" {
		t.Fatalf("unexpected reusable uses: %q", reusable.Uses)
	}
	if len(reusable.Secrets) != 1 || reusable.Secrets[0].Name != models.AllSecrets {
		t.Fatalf("expected inherited secrets, got %v", reusable.Secrets)
	}
}

func TestParseWorkflow_Anchors(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("anchors_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(w.Jobs))
	}

	buildEnv := w.Jobs[0].Env
	testEnv := w.Jobs[1].Env
	if len(buildEnv) != 2 || len(testEnv) != 2 {
		t.Fatalf("expected 2 envs each; build=%d, test=%d", len(buildEnv), len(testEnv))
	}
	for i, e := range buildEnv {
		if e.Name != testEnv[i].Name || e.Value != testEnv[i].Value {
			t.Fatalf("env mismatch at %d: build=%v, test=%v", i, e, testEnv[i])
		}
	}

	if w.Jobs[0].Steps[0].Uses != "actions/checkout@v4" {
		t.Fatalf("expected build checkout step, got %q", w.Jobs[0].Steps[0].Uses)
	}
	if w.Jobs[1].Steps[0].Uses != "actions/checkout@v4" {
		t.Fatalf("expected test checkout step via anchor, got %q", w.Jobs[1].Steps[0].Uses)
	}
}

func TestParseWorkflow_Minimal(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("minimal_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !w.IsValid() {
		t.Fatal("minimal workflow should be valid")
	}
	if w.Name != "" {
		t.Fatalf("expected empty name for minimal workflow, got %q", w.Name)
	}
	if len(w.Events) != 1 || w.Events[0].Name != "push" {
		t.Fatalf("unexpected events: %v", w.Events)
	}
	if len(w.Jobs) != 1 || w.Jobs[0].ID != "j" {
		t.Fatalf("unexpected jobs: %v", w.Jobs)
	}
}

// ---------------------------------------------------------------------------
// ParseWorkflow – error cases
// ---------------------------------------------------------------------------

func TestParseWorkflow_FileNotFound(t *testing.T) {
	_, err := ParseWorkflow(testdataPath("nonexistent.yml"))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseWorkflow_InvalidYAML(t *testing.T) {
	_, err := ParseWorkflow(testdataPath("invalid_yaml.yml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseWorkflow_InvalidWorkflow(t *testing.T) {
	_, err := ParseWorkflow(testdataPath("invalid_workflow.yml"))
	if err == nil {
		t.Fatal("expected error for workflow with no events/jobs")
	}
}

// ---------------------------------------------------------------------------
// ParseWorkflowFromBytes
// ---------------------------------------------------------------------------

func TestParseWorkflowFromBytes_Valid(t *testing.T) {
	data := []byte(`
name: Bytes Test
on: push
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello
`)
	w, err := ParseWorkflowFromBytes(data, "test/path.yml")
	if err != nil {
		t.Fatal(err)
	}
	if w.Path != "test/path.yml" {
		t.Fatalf("expected path 'test/path.yml', got %q", w.Path)
	}
	if w.Name != "Bytes Test" {
		t.Fatalf("expected name 'Bytes Test', got %q", w.Name)
	}
	if !w.IsValid() {
		t.Fatal("workflow should be valid")
	}
}

func TestParseWorkflowFromBytes_InvalidYAML(t *testing.T) {
	_, err := ParseWorkflowFromBytes([]byte("<not yaml>"), "bad.yml")
	if err == nil {
		t.Fatal("expected error for invalid YAML bytes")
	}
}

func TestParseWorkflowFromBytes_InvalidWorkflow(t *testing.T) {
	_, err := ParseWorkflowFromBytes([]byte("foo: bar"), "bad.yml")
	if err == nil {
		t.Fatal("expected error for invalid workflow bytes")
	}
}

// ---------------------------------------------------------------------------
// ParseWorkflow – line tracking
// ---------------------------------------------------------------------------

func TestParseWorkflow_LineTracking(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("valid_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}

	for _, j := range w.Jobs {
		if j.Line == 0 {
			t.Fatalf("job %q has zero line", j.ID)
		}
		if _, ok := j.Lines["start"]; !ok {
			t.Fatalf("job %q missing lines[start]", j.ID)
		}
	}

	for _, j := range w.Jobs {
		for i, s := range j.Steps {
			if s.Line == 0 {
				t.Fatalf("job %q step %d has zero line", j.ID, i)
			}
			if _, ok := s.Lines["start"]; !ok {
				t.Fatalf("job %q step %d missing lines[start]", j.ID, i)
			}
		}
	}
}

func TestParseWorkflow_StepActionParsed(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("valid_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}

	step := w.Jobs[0].Steps[0]
	if step.Uses != "actions/checkout@v4" {
		t.Fatalf("expected uses 'actions/checkout@v4', got %q", step.Uses)
	}
	if step.Action != "actions/checkout" {
		t.Fatalf("expected action 'actions/checkout', got %q", step.Action)
	}

	setup := w.Jobs[0].Steps[1]
	if setup.Action != "actions/setup-go" {
		t.Fatalf("expected action 'actions/setup-go', got %q", setup.Action)
	}
}

// ---------------------------------------------------------------------------
// ParseMetadata – file-based
// ---------------------------------------------------------------------------

func TestParseMetadata_NodeAction(t *testing.T) {
	m, err := ParseMetadata(testdataPath("node_action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Path != testdataPath("node_action.yml") {
		t.Fatalf("expected path set, got %q", m.Path)
	}
	if m.Name != "Setup Go" {
		t.Fatalf("expected name 'Setup Go', got %q", m.Name)
	}
	if m.Author != "GitHub" {
		t.Fatalf("expected author 'GitHub', got %q", m.Author)
	}
	if !m.IsValid() {
		t.Fatal("metadata should be valid")
	}
	if m.Runs.Using != "node20" {
		t.Fatalf("expected using 'node20', got %q", m.Runs.Using)
	}
	if m.Runs.Main != "dist/setup/index.js" {
		t.Fatalf("unexpected main: %q", m.Runs.Main)
	}
	if m.Runs.Post != "dist/cache-save/index.js" {
		t.Fatalf("unexpected post: %q", m.Runs.Post)
	}
	// PostIf has json tag "post-if" but no yaml tag, so the hyphenated key
	// doesn't decode. This documents the same limitation as Event hyphenated fields.
	if m.Runs.PostIf != "" {
		t.Fatalf("expected PostIf to be empty (no yaml tag), got %q", m.Runs.PostIf)
	}

	if len(m.Inputs) != 3 {
		t.Fatalf("expected 3 inputs, got %d", len(m.Inputs))
	}
	if len(m.Outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(m.Outputs))
	}
}

func TestParseMetadata_CompositeAction(t *testing.T) {
	m, err := ParseMetadata(testdataPath("composite_action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Runs.Using != "composite" {
		t.Fatalf("expected using 'composite', got %q", m.Runs.Using)
	}
	if len(m.Runs.Steps) != 3 {
		t.Fatalf("expected 3 composite steps, got %d", len(m.Runs.Steps))
	}

	if m.Runs.Steps[0].Uses != "actions/checkout@v4" {
		t.Fatalf("unexpected step 0 uses: %q", m.Runs.Steps[0].Uses)
	}
	if m.Runs.Steps[1].Run == "" {
		t.Fatal("expected step 1 to have a run command")
	}
	if m.Runs.Steps[1].Shell != "bash" {
		t.Fatalf("expected step 1 shell 'bash', got %q", m.Runs.Steps[1].Shell)
	}
	if m.Runs.Steps[2].WithScript == "" {
		t.Fatal("expected step 2 to have with.script set")
	}

	if len(m.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(m.Inputs))
	}
	if len(m.Outputs) != 1 || m.Outputs[0].Name != "url" {
		t.Fatalf("unexpected outputs: %v", m.Outputs)
	}
}

func TestParseMetadata_DockerAction(t *testing.T) {
	m, err := ParseMetadata(testdataPath("docker_action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Runs.Using != "docker" {
		t.Fatalf("expected using 'docker', got %q", m.Runs.Using)
	}
	if m.Runs.Image != "docker://alpine:3.19" {
		t.Fatalf("unexpected image: %q", m.Runs.Image)
	}
	if m.Runs.Entrypoint != "/entrypoint.sh" {
		t.Fatalf("unexpected entrypoint: %q", m.Runs.Entrypoint)
	}
	// PreEntrypoint has json tag "pre-entrypoint" but no yaml tag, so the
	// hyphenated key doesn't decode (same limitation as Event hyphenated fields).
	if m.Runs.PreEntrypoint != "" {
		t.Fatalf("expected PreEntrypoint to be empty (no yaml tag), got %q", m.Runs.PreEntrypoint)
	}
	if len(m.Runs.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(m.Runs.Args))
	}
	if len(m.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(m.Inputs))
	}
}

// ---------------------------------------------------------------------------
// ParseMetadata – error cases
// ---------------------------------------------------------------------------

func TestParseMetadata_FileNotFound(t *testing.T) {
	_, err := ParseMetadata(testdataPath("nonexistent_action.yml"))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseMetadata_InvalidAction(t *testing.T) {
	_, err := ParseMetadata(testdataPath("invalid_action.yml"))
	if err == nil {
		t.Fatal("expected error for action missing runs.using")
	}
}

func TestParseMetadata_InvalidYAML(t *testing.T) {
	_, err := ParseMetadata(testdataPath("invalid_yaml.yml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML metadata")
	}
}

// ---------------------------------------------------------------------------
// ParseMetadataFromBytes
// ---------------------------------------------------------------------------

func TestParseMetadataFromBytes_Valid(t *testing.T) {
	data := []byte(`
name: "Test Action"
description: "A test"
runs:
  using: "node20"
  main: "index.js"
`)
	m, err := ParseMetadataFromBytes(data, "actions/test/action.yml")
	if err != nil {
		t.Fatal(err)
	}
	if m.Path != "actions/test/action.yml" {
		t.Fatalf("expected path set, got %q", m.Path)
	}
	if m.Name != "Test Action" {
		t.Fatalf("expected name 'Test Action', got %q", m.Name)
	}
	if m.Runs.Using != "node20" {
		t.Fatalf("expected using 'node20', got %q", m.Runs.Using)
	}
}

func TestParseMetadataFromBytes_InvalidYAML(t *testing.T) {
	_, err := ParseMetadataFromBytes([]byte("<broken>"), "bad.yml")
	if err == nil {
		t.Fatal("expected error for invalid YAML bytes")
	}
}

func TestParseMetadataFromBytes_MissingUsing(t *testing.T) {
	data := []byte(`
name: "No Using"
runs:
  main: "index.js"
`)
	_, err := ParseMetadataFromBytes(data, "bad.yml")
	if err == nil {
		t.Fatal("expected error for metadata without runs.using")
	}
}

// ---------------------------------------------------------------------------
// Integration: dependency extraction readiness
// ---------------------------------------------------------------------------

func TestParseWorkflow_DependencyFields(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("complex_workflow.yml"))
	if err != nil {
		t.Fatal(err)
	}

	jobByID := make(map[string]models.Job)
	for _, j := range w.Jobs {
		jobByID[j.ID] = j
	}

	// Job DAG via Needs
	deploy := jobByID["deploy"]
	needsSet := make(map[string]bool)
	for _, n := range deploy.Needs {
		needsSet[n] = true
	}
	if !needsSet["build"] || !needsSet["test"] {
		t.Fatalf("deploy should need build and test, got %v", deploy.Needs)
	}

	// Action references from steps
	var actions []string
	for _, j := range w.Jobs {
		for _, s := range j.Steps {
			if s.Uses != "" {
				actions = append(actions, s.Uses)
			}
		}
	}
	if len(actions) == 0 {
		t.Fatal("expected at least one action reference")
	}
	found := false
	for _, a := range actions {
		if a == "actions/checkout@v4" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected actions/checkout@v4 among action refs")
	}

	// Reusable workflow call
	reusable := jobByID["call-reusable"]
	if reusable.Uses == "" {
		t.Fatal("expected call-reusable job to have Uses set")
	}

	// Container dependency
	test := jobByID["test"]
	if test.Container.Image == "" {
		t.Fatal("expected test job to have a container image")
	}
}

func TestParseWorkflow_WorkflowRunCrossWorkflowTrigger(t *testing.T) {
	w, err := ParseWorkflow(testdataPath("workflow_run.yml"))
	if err != nil {
		t.Fatal(err)
	}

	wr := w.Events[0]
	if wr.Name != "workflow_run" {
		t.Fatalf("expected workflow_run event, got %q", wr.Name)
	}
	if len(wr.Workflows) == 0 {
		t.Fatal("expected workflow_run to reference upstream workflows")
	}
}
