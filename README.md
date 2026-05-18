# rechta

A CLI tool that generates the full dependency tree of GitHub Actions workflows, including transitive dependencies from composite actions, reusable workflows, and local (internal) actions.

Given a directory of workflow files (or a single workflow file), rechta resolves every `uses:` reference via the GitHub API and the local filesystem, detects composite actions and reusable workflows, recursively discovers their nested dependencies, and outputs a complete dependency tree.

## Installation

### Pre-built binaries (recommended)

Download the latest binary for your platform from [GitHub Releases](https://github.com/sparkrew/rechta/releases/latest).

**Linux (amd64):**

```bash
curl -L -o rechta https://github.com/sparkrew/rechta/releases/latest/download/rechta-linux-amd64
chmod +x rechta
sudo mv rechta /usr/local/bin/
```

**Linux (arm64):**

```bash
curl -L -o rechta https://github.com/sparkrew/rechta/releases/latest/download/rechta-linux-arm64
chmod +x rechta
sudo mv rechta /usr/local/bin/
```

**macOS (Apple Silicon):**

```bash
curl -L -o rechta https://github.com/sparkrew/rechta/releases/latest/download/rechta-darwin-arm64
chmod +x rechta
sudo mv rechta /usr/local/bin/
```

**macOS (Intel):**

```bash
curl -L -o rechta https://github.com/sparkrew/rechta/releases/latest/download/rechta-darwin-amd64
chmod +x rechta
sudo mv rechta /usr/local/bin/
```

**Windows (amd64):**

Download [`rechta-windows-amd64.exe`](https://github.com/sparkrew/rechta/releases/latest/download/rechta-windows-amd64.exe) from the releases page and place it somewhere in your `PATH`.

Or via PowerShell:

```powershell
Invoke-WebRequest -Uri "https://github.com/sparkrew/rechta/releases/latest/download/rechta-windows-amd64.exe" -OutFile rechta.exe
```

### Via `go install`

Requires Go 1.25+:

```bash
go install github.com/sparkrew/rechta/cmd/rechta@latest
```

### Build from source

```bash
git clone https://github.com/sparkrew/rechta.git
cd rechta
go build -o rechta ./cmd/rechta/
```

## Usage

```bash
rechta [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--workflows` | `-w` | `.github/workflows` | Path to the workflows directory |
| `--file` | `-f` | | Path to a single workflow file (overrides `-w`) |
| `--token` | `-t` | `$GITHUB_TOKEN` env | GitHub API token for authentication |
| `--format` | | `json` | Output format: `txt` or `json` |
| `--depth` | | `10` | Maximum transitive dependency depth |

### Authentication

rechta calls the GitHub REST API to resolve action references. Without a token, you are limited to **60 requests per hour**. Set a token to get **5,000 requests per hour**:

```bash
export GITHUB_TOKEN=ghp_your_token_here
rechta -w .github/workflows
```

A personal access token with no special scopes is sufficient for public repositories.

On Windows:

```powershell
$env:GITHUB_TOKEN = "ghp_your_token_here"
.\rechta.exe -w .github\workflows
```

### Examples

**Text tree (all workflows in a directory):**

```bash
rechta -w .github/workflows -format txt
```

```
.github/workflows/ci.yml
+-- actions/checkout@v4 (34e114876b0b)
+-- ./my-local-action (local)
|   `-- actions/github-script@v7 (60a0d83039c7)
+-- actions/setup-go@v5 (40f1582b2485)
+-- codecov/codecov-action@v5 (75cd11691c0f)
|   `-- actions/github-script@60a0d83039c7... (60a0d83039c7)
`-- golangci/golangci-lint-action@v9 (1e7e51e771db)
```

Each line shows `action@ref (short-sha)` for remote actions or `./path (local)` for internal actions. Indented entries are transitive dependencies pulled in by composite actions. A `*` marker means the action was already resolved from a previous workflow (deduplicated).

**Single workflow file:**

```bash
rechta -f .github/workflows/ci.yml
```

When using `-f`, local action references (`./path`) are listed but not resolved (no repo context is available to read their `action.yml`).

**JSON output (default):**

```bash
rechta -w .github/workflows
```

```json
{
  "workflows": [
    {
      "path": ".github/workflows/ci.yml",
      "dependencies": [
        {
          "ref": {
            "owner": "actions",
            "repo": "checkout",
            "ref": "v4",
            "uses": "actions/checkout@v4"
          },
          "sha": "34e114876b0b...",
          "type": "node"
        },
        {
          "ref": {
            "uses": "./my-local-action",
            "is_local": true,
            "local_path": "./my-local-action"
          },
          "sha": "",
          "type": "composite",
          "dependencies": [...]
        }
      ]
    }
  ]
}
```

**Save output to a file:**

```bash
rechta -w .github/workflows > tree.txt
rechta -w .github/workflows > tree.json
```

Progress messages go to stderr, so redirecting stdout gives you clean output.

**Limit depth:**

```bash
rechta -w .github/workflows -depth 1
```

**Point at any repository:**

```bash
rechta -w /path/to/any-repo/.github/workflows
```

## What it detects

| Action type | Resolved? | Transitive deps? |
|-------------|-----------|-------------------|
| Standard actions (`owner/repo@ref`) | Yes | N/A (leaf node) |
| Composite actions (`runs.using: composite`) | Yes | Yes -- parses `steps[].uses` recursively |
| Reusable workflows (`owner/repo/.github/workflows/x.yml@ref`) | Yes | Yes -- parses `jobs.*.uses` and `jobs.*.steps[].uses` |
| Local actions (`./path`) | Yes (directory mode) | Yes -- reads `action.yml` from filesystem, walks transitive deps |
| Docker actions (`docker://`) | Skipped | Skipped -- container registries have their own integrity mechanisms |

**Note:** Local actions are only fully resolved in directory mode (`-w`). In single-file mode (`-f`), they appear in the tree but their metadata is not read (no filesystem context).

## How it works

1. Discovers `.yml` and `.yaml` files in the workflows directory (or parses a single file via `-f`)
2. Parses each workflow and extracts all `uses:` references from jobs and steps
3. For each remote reference, resolves the tag/branch to a commit SHA via the GitHub Git Data API
4. Fetches the action's `action.yml` (or reusable workflow YAML) at that SHA via the Contents API
5. For each local reference (`./path`), reads `action.yml`/`action.yaml` from the filesystem
6. If the action is composite, extracts nested `uses:` references from its steps and recurses
7. Deduplicates by raw `uses:` string across all workflows
8. Enforces a configurable depth limit (default 10, matching the GitHub Actions runner)

## Project structure

```
cmd/rechta/   -- CLI entry point
resolver/     -- GitHub API client and recursive dependency resolution
tree/         -- Text and JSON output formatters
models/       -- Go structs for workflow and action metadata YAML
parser/       -- YAML parsing for workflow files and action metadata
```

## Testing

```bash
go test -v -race ./...
```

## License

[MIT](LICENSE)
