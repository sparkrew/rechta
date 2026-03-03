# Parser Tests

Integration tests for the `parser` package. Run with:

```bash
go test -v ./parser/
```

## Testdata Fixtures

All fixtures live in `parser/testdata/`.

### Workflow files

| File | What it covers |
|---|---|
| `valid_workflow.yml` | Push/PR events with branch and tag filters, permissions, env vars, job dependencies (`needs`), `runs-on` scalar and sequence, container, environment with URL, `working-directory`, step `shell` |
| `matrix_workflow.yml` | Strategy matrix with two dimensions (os, go) |
| `reusable_workflow.yml` | `workflow_call` event with typed inputs, outputs, and secrets; job outputs; `with.ref` extraction |
| `workflow_run.yml` | `workflow_run` event referencing upstream workflows by name, with branch and type filters |
| `complex_workflow.yml` | All features combined: 4 event types (push, pull_request, schedule, workflow_dispatch), `read-all` permissions, path filters, cron, multi-job DAG, matrix strategy, container mapping, environment scalar, reusable workflow call with `secrets: inherit`, job-level permissions |
| `anchors_workflow.yml` | YAML anchors and aliases for env maps and steps |
| `minimal_workflow.yml` | Smallest valid workflow (no name, single event, single job) |
| `invalid_workflow.yml` | Valid YAML but not a workflow (no events or jobs) |
| `invalid_yaml.yml` | Malformed YAML |

### Action metadata files

| File | What it covers |
|---|---|
| `node_action.yml` | Node20 action with inputs, outputs, `main`, `post` |
| `composite_action.yml` | Composite action with mixed steps (uses, run, `with.script`) |
| `docker_action.yml` | Docker action with image, entrypoint, args |
| `invalid_action.yml` | Missing `runs.using` (fails validation) |

## Test Categories

### ParseWorkflow (file-based) -- 7 tests

Parse each workflow fixture from disk and verify the full structure: events, permissions, env, jobs, steps, needs, runs-on, container, environment, strategy, outputs, and reusable workflow calls.

### ParseWorkflow (error cases) -- 3 tests

File not found, invalid YAML syntax, and structurally invalid workflow (no events/jobs).

### ParseWorkflowFromBytes -- 3 tests

Same parse logic but from `[]byte` input. Verifies `Path` is set from the caller-supplied argument.

### Line tracking -- 1 test

Every job and step has a non-zero `Line` and a `Lines["start"]` entry.

### Action parsing -- 1 test

`step.Action` is extracted from `step.Uses` by stripping the `@version` suffix.

### ParseMetadata (file-based) -- 3 tests

Node, composite, and Docker action metadata. Verifies `runs.using`, `runs.main`, `runs.image`, `runs.entrypoint`, `runs.steps`, inputs, and outputs.

### ParseMetadata (error cases) -- 3 tests

File not found, invalid YAML, and missing `runs.using`.

### ParseMetadataFromBytes -- 3 tests

Same as file-based but from `[]byte`. Verifies path propagation and validation.

### Dependency extraction readiness -- 2 tests

Verify that all fields needed for downstream dependency tree construction are populated:
- Job DAG via `job.Needs`
- Action references via `step.Uses` / `step.Action`
- Reusable workflow calls via `job.Uses`
- Container images via `job.Container.Image`
- Cross-workflow triggers via `workflow_run.Workflows`

## Known Limitations

Hyphenated YAML keys like `post-if`, `pre-entrypoint`, `paths-ignore`, and `branches-ignore` don't decode because their corresponding struct fields have JSON tags but no YAML tags. The tests document this by asserting the fields remain empty.
