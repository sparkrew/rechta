package models

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// StringList
// ---------------------------------------------------------------------------

func TestStringList_Scalar(t *testing.T) {
	var sl StringList
	if err := yaml.Unmarshal([]byte(`ubuntu-latest`), &sl); err != nil {
		t.Fatal(err)
	}
	if len(sl) != 1 || sl[0] != "ubuntu-latest" {
		t.Fatalf("expected [ubuntu-latest], got %v", sl)
	}
}

func TestStringList_Sequence(t *testing.T) {
	var sl StringList
	if err := yaml.Unmarshal([]byte(`[self-hosted, linux, x64]`), &sl); err != nil {
		t.Fatal(err)
	}
	if len(sl) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sl))
	}
	want := []string{"self-hosted", "linux", "x64"}
	for i, v := range want {
		if sl[i] != v {
			t.Errorf("index %d: expected %q, got %q", i, v, sl[i])
		}
	}
}

func TestStringList_InvalidNode(t *testing.T) {
	var sl StringList
	err := yaml.Unmarshal([]byte(`{key: val}`), &sl)
	if err == nil {
		t.Fatal("expected error for mapping node")
	}
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

func TestEvents_Scalar(t *testing.T) {
	var ev Events
	if err := yaml.Unmarshal([]byte(`push`), &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev) != 1 || ev[0].Name != "push" {
		t.Fatalf("expected single push event, got %v", ev)
	}
}

func TestEvents_Sequence(t *testing.T) {
	var ev Events
	if err := yaml.Unmarshal([]byte(`[push, pull_request]`), &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev) != 2 {
		t.Fatalf("expected 2 events, got %d", len(ev))
	}
	if ev[0].Name != "push" || ev[1].Name != "pull_request" {
		t.Fatalf("unexpected event names: %v, %v", ev[0].Name, ev[1].Name)
	}
}

func TestEvents_MappingWithBranches(t *testing.T) {
	input := `
push:
  branches: [main, develop]
  tags: [v*]
pull_request:
  branches: [main]
`
	var ev Events
	if err := yaml.Unmarshal([]byte(input), &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev) != 2 {
		t.Fatalf("expected 2 events, got %d", len(ev))
	}

	push := ev[0]
	if push.Name != "push" {
		t.Fatalf("expected push, got %s", push.Name)
	}
	if len(push.Branches) != 2 || push.Branches[0] != "main" || push.Branches[1] != "develop" {
		t.Fatalf("unexpected push branches: %v", push.Branches)
	}
	if len(push.Tags) != 1 || push.Tags[0] != "v*" {
		t.Fatalf("unexpected push tags: %v", push.Tags)
	}

	pr := ev[1]
	if pr.Name != "pull_request" {
		t.Fatalf("expected pull_request, got %s", pr.Name)
	}
	if len(pr.Branches) != 1 || pr.Branches[0] != "main" {
		t.Fatalf("unexpected PR branches: %v", pr.Branches)
	}
}

func TestEvents_Schedule(t *testing.T) {
	input := `
schedule:
  - cron: "0 0 * * *"
  - cron: "30 5 * * 1,3"
`
	var ev Events
	if err := yaml.Unmarshal([]byte(input), &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev) != 1 {
		t.Fatalf("expected 1 schedule event, got %d", len(ev))
	}
	if ev[0].Name != "schedule" {
		t.Fatalf("expected schedule, got %s", ev[0].Name)
	}
	if len(ev[0].Cron) != 2 {
		t.Fatalf("expected 2 cron entries, got %d", len(ev[0].Cron))
	}
	if ev[0].Cron[0] != "0 0 * * *" {
		t.Fatalf("unexpected cron[0]: %q", ev[0].Cron[0])
	}
	if ev[0].Cron[1] != "30 5 * * 1,3" {
		t.Fatalf("unexpected cron[1]: %q", ev[0].Cron[1])
	}
}

func TestEvents_WorkflowCall(t *testing.T) {
	input := `
workflow_call:
  inputs:
    environment:
      description: "Target env"
      required: true
      type: string
  outputs:
    result:
      description: "Build result"
      value: "success"
  secrets:
    token:
      required: true
`
	var ev Events
	if err := yaml.Unmarshal([]byte(input), &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev) != 1 || ev[0].Name != "workflow_call" {
		t.Fatalf("expected workflow_call, got %v", ev)
	}
	wc := ev[0]
	if len(wc.Inputs) != 1 || wc.Inputs[0].Name != "environment" {
		t.Fatalf("unexpected inputs: %v", wc.Inputs)
	}
	if wc.Inputs[0].Type != "string" {
		t.Fatalf("expected input type string, got %q", wc.Inputs[0].Type)
	}
	if !wc.Inputs[0].Required {
		t.Fatal("expected input to be required")
	}
	if len(wc.Outputs) != 1 || wc.Outputs[0].Name != "result" {
		t.Fatalf("unexpected outputs: %v", wc.Outputs)
	}
	if len(wc.Secrets) != 1 || wc.Secrets[0].Name != "token" {
		t.Fatalf("unexpected secrets: %v", wc.Secrets)
	}
}

func TestEvents_WorkflowRun(t *testing.T) {
	input := `
workflow_run:
  workflows: [CI, Deploy]
  branches: [main]
  types: [completed]
`
	var ev Events
	if err := yaml.Unmarshal([]byte(input), &ev); err != nil {
		t.Fatal(err)
	}
	wr := ev[0]
	if wr.Name != "workflow_run" {
		t.Fatalf("expected workflow_run, got %s", wr.Name)
	}
	if len(wr.Workflows) != 2 || wr.Workflows[0] != "CI" || wr.Workflows[1] != "Deploy" {
		t.Fatalf("unexpected workflows: %v", wr.Workflows)
	}
	if len(wr.Types) != 1 || wr.Types[0] != "completed" {
		t.Fatalf("unexpected types: %v", wr.Types)
	}
}

func TestEvents_MappingWithNullValue(t *testing.T) {
	input := `
push:
pull_request:
`
	var ev Events
	if err := yaml.Unmarshal([]byte(input), &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev) != 2 {
		t.Fatalf("expected 2 events, got %d", len(ev))
	}
	if ev[0].Name != "push" || ev[1].Name != "pull_request" {
		t.Fatalf("unexpected names: %v, %v", ev[0].Name, ev[1].Name)
	}
}

func TestEvents_PathFilters(t *testing.T) {
	input := `
push:
  paths: ["src/**"]
  tags-ignore: ["beta*"]
`
	var ev Events
	if err := yaml.Unmarshal([]byte(input), &ev); err != nil {
		t.Fatal(err)
	}
	push := ev[0]
	if len(push.Paths) != 1 || push.Paths[0] != "src/**" {
		t.Fatalf("unexpected paths: %v", push.Paths)
	}
}

func TestEvents_HyphenatedFieldsNeedYAMLTags(t *testing.T) {
	// NOTE: paths-ignore, branches-ignore, tags-ignore use hyphens in YAML
	// but the Event struct lacks yaml tags for these, so they decode as empty.
	// This test documents that limitation.
	input := `
push:
  paths-ignore: ["docs/**"]
  branches-ignore: [release]
`
	var ev Events
	if err := yaml.Unmarshal([]byte(input), &ev); err != nil {
		t.Fatal(err)
	}
	push := ev[0]
	if len(push.PathsIgnore) != 0 {
		t.Fatalf("expected PathsIgnore to be empty (no yaml tag), got %v", push.PathsIgnore)
	}
	if len(push.BranchesIgnore) != 0 {
		t.Fatalf("expected BranchesIgnore to be empty (no yaml tag), got %v", push.BranchesIgnore)
	}
}

// ---------------------------------------------------------------------------
// Permissions
// ---------------------------------------------------------------------------

func TestPermissions_ReadAll(t *testing.T) {
	var perms Permissions
	if err := yaml.Unmarshal([]byte(`read-all`), &perms); err != nil {
		t.Fatal(err)
	}
	if len(perms) != len(AllScopes) {
		t.Fatalf("expected %d scopes, got %d", len(AllScopes), len(perms))
	}
	for _, p := range perms {
		if p.Permission != PermissionRead {
			t.Fatalf("expected read for scope %s, got %s", p.Scope, p.Permission)
		}
	}
}

func TestPermissions_WriteAll(t *testing.T) {
	var perms Permissions
	if err := yaml.Unmarshal([]byte(`write-all`), &perms); err != nil {
		t.Fatal(err)
	}
	if len(perms) != len(AllScopes) {
		t.Fatalf("expected %d scopes, got %d", len(AllScopes), len(perms))
	}
	for _, p := range perms {
		if p.Permission != PermissionWrite {
			t.Fatalf("expected write for scope %s, got %s", p.Scope, p.Permission)
		}
	}
}

func TestPermissions_Mapping(t *testing.T) {
	input := `
contents: read
packages: write
id-token: none
`
	var perms Permissions
	if err := yaml.Unmarshal([]byte(input), &perms); err != nil {
		t.Fatal(err)
	}
	if len(perms) != 3 {
		t.Fatalf("expected 3 permissions, got %d", len(perms))
	}

	expected := map[string]string{
		"contents": "read",
		"packages": "write",
		"id-token": "none",
	}
	for _, p := range perms {
		want, ok := expected[p.Scope]
		if !ok {
			t.Fatalf("unexpected scope %s", p.Scope)
		}
		if p.Permission != want {
			t.Fatalf("scope %s: expected %s, got %s", p.Scope, want, p.Permission)
		}
	}
}

func TestPermissions_InvalidScalar(t *testing.T) {
	var perms Permissions
	err := yaml.Unmarshal([]byte(`invalid-value`), &perms)
	if err == nil {
		t.Fatal("expected error for invalid permission scalar")
	}
}

// ---------------------------------------------------------------------------
// JobRunsOn
// ---------------------------------------------------------------------------

func TestJobRunsOn_Scalar(t *testing.T) {
	var ro JobRunsOn
	if err := yaml.Unmarshal([]byte(`ubuntu-latest`), &ro); err != nil {
		t.Fatal(err)
	}
	if len(ro) != 1 || ro[0] != "ubuntu-latest" {
		t.Fatalf("expected [ubuntu-latest], got %v", ro)
	}
}

func TestJobRunsOn_Sequence(t *testing.T) {
	var ro JobRunsOn
	if err := yaml.Unmarshal([]byte(`[self-hosted, linux]`), &ro); err != nil {
		t.Fatal(err)
	}
	if len(ro) != 2 || ro[0] != "self-hosted" || ro[1] != "linux" {
		t.Fatalf("expected [self-hosted linux], got %v", ro)
	}
}

func TestJobRunsOn_MappingGroupLabels(t *testing.T) {
	input := `
group: large-runners
labels: [ubuntu-latest, x64]
`
	var ro JobRunsOn
	if err := yaml.Unmarshal([]byte(input), &ro); err != nil {
		t.Fatal(err)
	}
	if len(ro) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(ro), ro)
	}
	if ro[0] != "group:large-runners" {
		t.Fatalf("expected group:large-runners, got %s", ro[0])
	}
	if ro[1] != "label:ubuntu-latest" || ro[2] != "label:x64" {
		t.Fatalf("unexpected labels: %v", ro[1:])
	}
}

// ---------------------------------------------------------------------------
// JobSecrets
// ---------------------------------------------------------------------------

func TestJobSecrets_Inherit(t *testing.T) {
	var sec JobSecrets
	if err := yaml.Unmarshal([]byte(`inherit`), &sec); err != nil {
		t.Fatal(err)
	}
	if len(sec) != 1 || sec[0].Name != AllSecrets || sec[0].Value != "inherit" {
		t.Fatalf("expected inherit sentinel, got %v", sec)
	}
}

func TestJobSecrets_Mapping(t *testing.T) {
	input := `
TOKEN: "${{ secrets.GITHUB_TOKEN }}"
DEPLOY_KEY: "${{ secrets.DEPLOY_KEY }}"
`
	var sec JobSecrets
	if err := yaml.Unmarshal([]byte(input), &sec); err != nil {
		t.Fatal(err)
	}
	if len(sec) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(sec))
	}
	if sec[0].Name != "TOKEN" || sec[0].Value != "${{ secrets.GITHUB_TOKEN }}" {
		t.Fatalf("unexpected secret[0]: %v", sec[0])
	}
	if sec[1].Name != "DEPLOY_KEY" || sec[1].Value != "${{ secrets.DEPLOY_KEY }}" {
		t.Fatalf("unexpected secret[1]: %v", sec[1])
	}
}

// ---------------------------------------------------------------------------
// JobContainer
// ---------------------------------------------------------------------------

func TestJobContainer_Scalar(t *testing.T) {
	var c JobContainer
	if err := yaml.Unmarshal([]byte(`alpine:latest`), &c); err != nil {
		t.Fatal(err)
	}
	if c.Image != "alpine:latest" {
		t.Fatalf("expected alpine:latest, got %s", c.Image)
	}
}

func TestJobContainer_Mapping(t *testing.T) {
	input := `
image: node:18
`
	var c JobContainer
	if err := yaml.Unmarshal([]byte(input), &c); err != nil {
		t.Fatal(err)
	}
	if c.Image != "node:18" {
		t.Fatalf("expected node:18, got %s", c.Image)
	}
}

// ---------------------------------------------------------------------------
// JobEnvironments
// ---------------------------------------------------------------------------

func TestJobEnvironments_Scalar(t *testing.T) {
	var env JobEnvironments
	if err := yaml.Unmarshal([]byte(`production`), &env); err != nil {
		t.Fatal(err)
	}
	if len(env) != 1 || env[0].Name != "production" {
		t.Fatalf("expected [production], got %v", env)
	}
}

func TestJobEnvironments_Mapping(t *testing.T) {
	input := `
name: staging
url: "https://staging.example.com"
`
	var env JobEnvironments
	if err := yaml.Unmarshal([]byte(input), &env); err != nil {
		t.Fatal(err)
	}
	if len(env) != 1 || env[0].Name != "staging" || env[0].Url != "https://staging.example.com" {
		t.Fatalf("unexpected environment: %v", env[0])
	}
}

// ---------------------------------------------------------------------------
// Envs
// ---------------------------------------------------------------------------

func TestEnvs_Mapping(t *testing.T) {
	input := `
CI: "true"
NODE_ENV: production
`
	var e Envs
	if err := yaml.Unmarshal([]byte(input), &e); err != nil {
		t.Fatal(err)
	}
	if len(e) != 2 {
		t.Fatalf("expected 2 envs, got %d", len(e))
	}
	if e[0].Name != "CI" || e[0].Value != "true" {
		t.Fatalf("unexpected env[0]: %v", e[0])
	}
	if e[1].Name != "NODE_ENV" || e[1].Value != "production" {
		t.Fatalf("unexpected env[1]: %v", e[1])
	}
}

func TestEnvs_ExpressionScalar(t *testing.T) {
	var e Envs
	if err := yaml.Unmarshal([]byte(`"${{ fromJson(needs.setup.outputs.env) }}"`), &e); err != nil {
		t.Fatal(err)
	}
	if len(e) != 1 || e[0].Value != "${{ fromJson(needs.setup.outputs.env) }}" {
		t.Fatalf("expected expression env, got %v", e)
	}
	if e[0].Name != "" {
		t.Fatalf("expected empty name for expression env, got %q", e[0].Name)
	}
}

// ---------------------------------------------------------------------------
// Inputs
// ---------------------------------------------------------------------------

func TestInputs_Mapping(t *testing.T) {
	input := `
name:
  description: "The person's name"
  required: true
  type: string
age:
  description: "Age"
  required: false
  type: number
`
	var inp Inputs
	if err := yaml.Unmarshal([]byte(input), &inp); err != nil {
		t.Fatal(err)
	}
	if len(inp) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(inp))
	}
	if inp[0].Name != "name" || !inp[0].Required || inp[0].Type != "string" {
		t.Fatalf("unexpected input[0]: %+v", inp[0])
	}
	if inp[1].Name != "age" || inp[1].Required || inp[1].Type != "number" {
		t.Fatalf("unexpected input[1]: %+v", inp[1])
	}
}

// ---------------------------------------------------------------------------
// Outputs
// ---------------------------------------------------------------------------

func TestOutputs_ScalarValue(t *testing.T) {
	input := `
result: "success"
`
	var out Outputs
	if err := yaml.Unmarshal([]byte(input), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Name != "result" || out[0].Value != "success" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestOutputs_MappingValue(t *testing.T) {
	input := `
build_id:
  description: "The build identifier"
  value: "${{ steps.build.outputs.id }}"
`
	var out Outputs
	if err := yaml.Unmarshal([]byte(input), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 output, got %d", len(out))
	}
	if out[0].Name != "build_id" || out[0].Value != "${{ steps.build.outputs.id }}" {
		t.Fatalf("unexpected output: %+v", out[0])
	}
}

// ---------------------------------------------------------------------------
// Strategy
// ---------------------------------------------------------------------------

func TestStrategy_Matrix(t *testing.T) {
	input := `
matrix:
  os: [ubuntu-latest, windows-latest]
  node: ["16", "18", "20"]
`
	var s Strategy
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Matrix) != 2 {
		t.Fatalf("expected 2 matrix dimensions, got %d", len(s.Matrix))
	}
	if len(s.Matrix["os"]) != 2 {
		t.Fatalf("expected 2 os entries, got %d", len(s.Matrix["os"]))
	}
	if len(s.Matrix["node"]) != 3 {
		t.Fatalf("expected 3 node entries, got %d", len(s.Matrix["node"]))
	}
}

func TestStrategy_MatrixWithIncludes(t *testing.T) {
	input := `
matrix:
  include:
    - os: macos-latest
      node: "20"
`
	var s Strategy
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatal(err)
	}
	items := s.Matrix["include"]
	if len(items) != 1 {
		t.Fatalf("expected 1 include, got %d", len(items))
	}
}

// ---------------------------------------------------------------------------
// Steps (UnmarshalYAML)
// ---------------------------------------------------------------------------

func TestStep_UsesAction(t *testing.T) {
	input := `
- uses: actions/checkout@v4
  with:
    ref: main
`
	var steps Steps
	if err := yaml.Unmarshal([]byte(input), &steps); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	s := steps[0]
	if s.Uses != "actions/checkout@v4" {
		t.Fatalf("expected uses actions/checkout@v4, got %q", s.Uses)
	}
	if s.Action != "actions/checkout" {
		t.Fatalf("expected action actions/checkout, got %q", s.Action)
	}
	if s.WithRef != "main" {
		t.Fatalf("expected with.ref=main, got %q", s.WithRef)
	}
	if _, ok := s.Lines["uses"]; !ok {
		t.Fatal("expected lines[uses] to be set")
	}
	if _, ok := s.Lines["with_ref"]; !ok {
		t.Fatal("expected lines[with_ref] to be set")
	}
}

func TestStep_RunCommand(t *testing.T) {
	input := `
- name: Build
  run: npm run build
  shell: bash
  working-directory: ./frontend
`
	var steps Steps
	if err := yaml.Unmarshal([]byte(input), &steps); err != nil {
		t.Fatal(err)
	}
	s := steps[0]
	if s.Name != "Build" {
		t.Fatalf("expected name Build, got %q", s.Name)
	}
	if s.Run != "npm run build" {
		t.Fatalf("expected run command, got %q", s.Run)
	}
	if s.Shell != "bash" {
		t.Fatalf("expected shell bash, got %q", s.Shell)
	}
	if s.WorkingDirectory != "./frontend" {
		t.Fatalf("expected working-directory ./frontend, got %q", s.WorkingDirectory)
	}
	if _, ok := s.Lines["run"]; !ok {
		t.Fatal("expected lines[run] to be set")
	}
}

func TestStep_WithScript(t *testing.T) {
	input := `
- uses: actions/github-script@v7
  with:
    script: |
      console.log("hello")
`
	var steps Steps
	if err := yaml.Unmarshal([]byte(input), &steps); err != nil {
		t.Fatal(err)
	}
	s := steps[0]
	if s.WithScript == "" {
		t.Fatal("expected with.script to be set")
	}
	if _, ok := s.Lines["with_script"]; !ok {
		t.Fatal("expected lines[with_script] to be set")
	}
}

func TestStep_LineTracking(t *testing.T) {
	input := `
- id: step1
  name: First
  if: always()
  run: echo hello
`
	var steps Steps
	if err := yaml.Unmarshal([]byte(input), &steps); err != nil {
		t.Fatal(err)
	}
	s := steps[0]
	if s.Line == 0 {
		t.Fatal("expected non-zero line")
	}
	if _, ok := s.Lines["start"]; !ok {
		t.Fatal("expected lines[start] to be set")
	}
	if _, ok := s.Lines["if"]; !ok {
		t.Fatal("expected lines[if] to be set")
	}
	if _, ok := s.Lines["run"]; !ok {
		t.Fatal("expected lines[run] to be set")
	}
}

func TestStep_ActionWithoutVersion(t *testing.T) {
	input := `
- uses: ./local-action
`
	var steps Steps
	if err := yaml.Unmarshal([]byte(input), &steps); err != nil {
		t.Fatal(err)
	}
	if steps[0].Action != "./local-action" {
		t.Fatalf("expected local action path, got %q", steps[0].Action)
	}
}

func TestStep_WithEnv(t *testing.T) {
	input := `
- run: echo $TOKEN
  env:
    TOKEN: "${{ secrets.TOKEN }}"
`
	var steps Steps
	if err := yaml.Unmarshal([]byte(input), &steps); err != nil {
		t.Fatal(err)
	}
	if len(steps[0].Env) != 1 || steps[0].Env[0].Name != "TOKEN" {
		t.Fatalf("unexpected env: %v", steps[0].Env)
	}
}

// ---------------------------------------------------------------------------
// Jobs (UnmarshalYAML)
// ---------------------------------------------------------------------------

func TestJobs_BasicMapping(t *testing.T) {
	input := `
build:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - run: make build
test:
  needs: build
  runs-on: ubuntu-latest
  steps:
    - run: make test
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	build := jobs[0]
	if build.ID != "build" {
		t.Fatalf("expected build, got %s", build.ID)
	}
	if len(build.RunsOn) != 1 || build.RunsOn[0] != "ubuntu-latest" {
		t.Fatalf("unexpected runs-on: %v", build.RunsOn)
	}
	if len(build.Steps) != 2 {
		t.Fatalf("expected 2 build steps, got %d", len(build.Steps))
	}

	test := jobs[1]
	if test.ID != "test" {
		t.Fatalf("expected test, got %s", test.ID)
	}
	if len(test.Needs) != 1 || test.Needs[0] != "build" {
		t.Fatalf("unexpected needs: %v", test.Needs)
	}
}

func TestJobs_LineTracking(t *testing.T) {
	input := `
deploy:
  if: github.ref == 'refs/heads/main'
  runs-on: ubuntu-latest
  steps:
    - run: deploy
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	j := jobs[0]
	if j.Line == 0 {
		t.Fatal("expected non-zero line for job")
	}
	if _, ok := j.Lines["start"]; !ok {
		t.Fatal("expected lines[start]")
	}
	if _, ok := j.Lines["runs_on"]; !ok {
		t.Fatal("expected lines[runs_on]")
	}
	if _, ok := j.Lines["if"]; !ok {
		t.Fatal("expected lines[if]")
	}
}

func TestJobs_ReusableWorkflow(t *testing.T) {
	input := `
call-ci:
  uses: org/repo/.github/workflows/ci.yml@main
  secrets: inherit
  with:
    env: staging
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	j := jobs[0]
	if j.Uses != "org/repo/.github/workflows/ci.yml@main" {
		t.Fatalf("unexpected uses: %q", j.Uses)
	}
	if len(j.Secrets) != 1 || j.Secrets[0].Name != AllSecrets {
		t.Fatalf("unexpected secrets: %v", j.Secrets)
	}
	if len(j.With) != 1 || j.With[0].Name != "env" || j.With[0].Value != "staging" {
		t.Fatalf("unexpected with: %v", j.With)
	}
}

func TestJobs_MultipleNeeds(t *testing.T) {
	input := `
deploy:
  needs: [build, test, lint]
  runs-on: ubuntu-latest
  steps:
    - run: deploy
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs[0].Needs) != 3 {
		t.Fatalf("expected 3 needs, got %d", len(jobs[0].Needs))
	}
}

func TestJobs_ContainerAndEnvironment(t *testing.T) {
	input := `
e2e:
  runs-on: ubuntu-latest
  container: cypress/browsers:latest
  environment:
    name: staging
    url: "https://staging.example.com"
  steps:
    - run: cypress run
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	j := jobs[0]
	if j.Container.Image != "cypress/browsers:latest" {
		t.Fatalf("unexpected container: %q", j.Container.Image)
	}
	if len(j.Environment) != 1 || j.Environment[0].Name != "staging" {
		t.Fatalf("unexpected environment: %v", j.Environment)
	}
	if j.Environment[0].Url != "https://staging.example.com" {
		t.Fatalf("unexpected env url: %q", j.Environment[0].Url)
	}
}

func TestJobs_Strategy(t *testing.T) {
	input := `
test:
  runs-on: ubuntu-latest
  strategy:
    matrix:
      go: ["1.21", "1.22"]
  steps:
    - run: go test ./...
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	j := jobs[0]
	if len(j.Strategy.Matrix) != 1 {
		t.Fatalf("expected 1 matrix dimension, got %d", len(j.Strategy.Matrix))
	}
	if len(j.Strategy.Matrix["go"]) != 2 {
		t.Fatalf("expected 2 go versions, got %d", len(j.Strategy.Matrix["go"]))
	}
}

func TestJobs_Outputs(t *testing.T) {
	input := `
prepare:
  runs-on: ubuntu-latest
  outputs:
    version: "${{ steps.ver.outputs.version }}"
  steps:
    - id: ver
      run: echo "version=1.0" >> $GITHUB_OUTPUT
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs[0].Outputs) != 1 || jobs[0].Outputs[0].Name != "version" {
		t.Fatalf("unexpected outputs: %v", jobs[0].Outputs)
	}
}

// ---------------------------------------------------------------------------
// Full Workflow
// ---------------------------------------------------------------------------

func TestWorkflow_FullParse(t *testing.T) {
	input := `
name: CI
on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read
  packages: write

env:
  GO_VERSION: "1.22"

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build
        run: go build ./...
  test:
    needs: build
    runs-on: [self-hosted, linux]
    container: golang:1.22
    steps:
      - uses: actions/checkout@v4
      - run: go test ./...
`
	var w Workflow
	if err := yaml.Unmarshal([]byte(input), &w); err != nil {
		t.Fatal(err)
	}
	if w.Name != "CI" {
		t.Fatalf("expected name CI, got %q", w.Name)
	}
	if !w.IsValid() {
		t.Fatal("workflow should be valid")
	}
	if len(w.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(w.Events))
	}
	if len(w.Permissions) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(w.Permissions))
	}
	if len(w.Env) != 1 || w.Env[0].Name != "GO_VERSION" {
		t.Fatalf("unexpected env: %v", w.Env)
	}
	if len(w.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(w.Jobs))
	}

	build := w.Jobs[0]
	if build.ID != "build" {
		t.Fatalf("expected build job, got %s", build.ID)
	}
	if len(build.Steps) != 2 {
		t.Fatalf("expected 2 build steps, got %d", len(build.Steps))
	}

	test := w.Jobs[1]
	if test.ID != "test" {
		t.Fatalf("expected test job, got %s", test.ID)
	}
	if test.Container.Image != "golang:1.22" {
		t.Fatalf("unexpected container: %q", test.Container.Image)
	}
	if len(test.RunsOn) != 2 {
		t.Fatalf("expected 2 runs-on labels, got %d", len(test.RunsOn))
	}
}

func TestWorkflow_IsValid(t *testing.T) {
	empty := Workflow{}
	if empty.IsValid() {
		t.Fatal("empty workflow should not be valid")
	}

	noEvents := Workflow{Jobs: Jobs{{ID: "j"}}}
	if noEvents.IsValid() {
		t.Fatal("workflow without events should not be valid")
	}

	noJobs := Workflow{Events: Events{{Name: "push"}}}
	if noJobs.IsValid() {
		t.Fatal("workflow without jobs should not be valid")
	}
}

// ---------------------------------------------------------------------------
// Metadata (action.yml)
// ---------------------------------------------------------------------------

func TestMetadata_Parse(t *testing.T) {
	input := `
name: "My Action"
description: "A custom action"
author: "Test"
inputs:
  token:
    description: "GitHub token"
    required: true
outputs:
  result:
    description: "The result"
    value: "done"
runs:
  using: "node20"
  main: "dist/index.js"
  pre: "dist/setup.js"
  post: "dist/cleanup.js"
`
	var m Metadata
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "My Action" {
		t.Fatalf("expected name, got %q", m.Name)
	}
	if m.Author != "Test" {
		t.Fatalf("expected author Test, got %q", m.Author)
	}
	if !m.IsValid() {
		t.Fatal("metadata should be valid")
	}
	if m.Runs.Using != "node20" {
		t.Fatalf("expected using node20, got %q", m.Runs.Using)
	}
	if m.Runs.Main != "dist/index.js" {
		t.Fatalf("unexpected main: %q", m.Runs.Main)
	}
	if m.Runs.Pre != "dist/setup.js" {
		t.Fatalf("unexpected pre: %q", m.Runs.Pre)
	}
	if m.Runs.Post != "dist/cleanup.js" {
		t.Fatalf("unexpected post: %q", m.Runs.Post)
	}
	if len(m.Inputs) != 1 || m.Inputs[0].Name != "token" {
		t.Fatalf("unexpected inputs: %v", m.Inputs)
	}
	if len(m.Outputs) != 1 || m.Outputs[0].Name != "result" {
		t.Fatalf("unexpected outputs: %v", m.Outputs)
	}
}

func TestMetadata_CompositeAction(t *testing.T) {
	input := `
name: "Composite"
description: "A composite action"
runs:
  using: "composite"
  steps:
    - run: echo "step 1"
      shell: bash
    - uses: actions/checkout@v4
`
	var m Metadata
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}
	if m.Runs.Using != "composite" {
		t.Fatalf("expected composite, got %q", m.Runs.Using)
	}
	if len(m.Runs.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(m.Runs.Steps))
	}
	if m.Runs.Steps[0].Run != "echo \"step 1\"" {
		t.Fatalf("unexpected run: %q", m.Runs.Steps[0].Run)
	}
	if m.Runs.Steps[1].Uses != "actions/checkout@v4" {
		t.Fatalf("unexpected uses: %q", m.Runs.Steps[1].Uses)
	}
}

func TestMetadata_DockerAction(t *testing.T) {
	input := `
name: "Docker Action"
description: "Runs in Docker"
runs:
  using: "docker"
  image: "Dockerfile"
  entrypoint: "/entrypoint.sh"
  args:
    - "--flag"
    - "value"
`
	var m Metadata
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}
	if m.Runs.Image != "Dockerfile" {
		t.Fatalf("unexpected image: %q", m.Runs.Image)
	}
	if m.Runs.Entrypoint != "/entrypoint.sh" {
		t.Fatalf("unexpected entrypoint: %q", m.Runs.Entrypoint)
	}
	if len(m.Runs.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(m.Runs.Args))
	}
}

func TestMetadata_IsValid(t *testing.T) {
	empty := Metadata{}
	if empty.IsValid() {
		t.Fatal("empty metadata should not be valid")
	}
}

// ---------------------------------------------------------------------------
// YAML Anchors and Aliases
// ---------------------------------------------------------------------------

func TestWorkflow_YAMLAnchors(t *testing.T) {
	input := `
name: Anchored CI
on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    env: &shared-env
      CI: "true"
      NODE_ENV: production
    steps:
      - run: echo build
  test:
    runs-on: ubuntu-latest
    env: *shared-env
    steps:
      - run: echo test
`
	var w Workflow
	if err := yaml.Unmarshal([]byte(input), &w); err != nil {
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
}

func TestWorkflow_AnchoredSteps(t *testing.T) {
	input := `
name: Anchor Steps
on: push

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - &checkout
        uses: actions/checkout@v4
      - run: make build
  test:
    runs-on: ubuntu-latest
    steps:
      - *checkout
      - run: make test
`
	var w Workflow
	if err := yaml.Unmarshal([]byte(input), &w); err != nil {
		t.Fatal(err)
	}
	buildSteps := w.Jobs[0].Steps
	testSteps := w.Jobs[1].Steps

	if buildSteps[0].Uses != "actions/checkout@v4" {
		t.Fatalf("build step[0] uses: %q", buildSteps[0].Uses)
	}
	if testSteps[0].Uses != "actions/checkout@v4" {
		t.Fatalf("test step[0] uses: %q", testSteps[0].Uses)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestWorkflow_MinimalValid(t *testing.T) {
	input := `
on: push
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo ok
`
	var w Workflow
	if err := yaml.Unmarshal([]byte(input), &w); err != nil {
		t.Fatal(err)
	}
	if !w.IsValid() {
		t.Fatal("minimal workflow should be valid")
	}
}

func TestWorkflow_EmptyPermissions(t *testing.T) {
	input := `
on: push
permissions: {}
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo ok
`
	var w Workflow
	if err := yaml.Unmarshal([]byte(input), &w); err != nil {
		t.Fatal(err)
	}
	if len(w.Permissions) != 0 {
		t.Fatalf("expected 0 permissions, got %d", len(w.Permissions))
	}
}

func TestJobs_JobPermissions(t *testing.T) {
	input := `
deploy:
  runs-on: ubuntu-latest
  permissions:
    contents: read
    id-token: write
  steps:
    - run: deploy
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs[0].Permissions) != 2 {
		t.Fatalf("expected 2 job permissions, got %d", len(jobs[0].Permissions))
	}
}

func TestJobs_EnvironmentScalar(t *testing.T) {
	input := `
deploy:
  runs-on: ubuntu-latest
  environment: production
  steps:
    - run: deploy
`
	var jobs Jobs
	if err := yaml.Unmarshal([]byte(input), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs[0].Environment) != 1 || jobs[0].Environment[0].Name != "production" {
		t.Fatalf("unexpected environment: %v", jobs[0].Environment)
	}
}

func TestStep_MultipleWith(t *testing.T) {
	input := `
- uses: actions/setup-node@v4
  with:
    node-version: "20"
    cache: npm
`
	var steps Steps
	if err := yaml.Unmarshal([]byte(input), &steps); err != nil {
		t.Fatal(err)
	}
	if len(steps[0].With) != 2 {
		t.Fatalf("expected 2 with entries, got %d", len(steps[0].With))
	}
}

func TestEvents_MixedComplex(t *testing.T) {
	input := `
push:
  branches: [main]
schedule:
  - cron: "0 0 * * *"
workflow_dispatch:
`
	var ev Events
	if err := yaml.Unmarshal([]byte(input), &ev); err != nil {
		t.Fatal(err)
	}
	if len(ev) != 3 {
		t.Fatalf("expected 3 events, got %d", len(ev))
	}
	if ev[0].Name != "push" {
		t.Fatalf("expected push, got %s", ev[0].Name)
	}
	if ev[1].Name != "schedule" {
		t.Fatalf("expected schedule, got %s", ev[1].Name)
	}
	if ev[2].Name != "workflow_dispatch" {
		t.Fatalf("expected workflow_dispatch, got %s", ev[2].Name)
	}
}
