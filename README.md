# rechta

A CLI tool that generates the full dependency tree of GitHub Actions workflows, including transitive dependencies from composite actions, reusable workflows, and local (internal) actions.

Given a directory of workflow files, a single workflow file, or a GitHub repository URL, rechta resolves every `uses:` reference via the GitHub API and the local filesystem, detects composite actions and reusable workflows, recursively discovers their nested dependencies, and outputs a complete dependency tree.

## Installation

### Pre-built binaries (recommended)

Download the latest binary for your platform from [GitHub Releases](https://github.com/sparkrew/rechta/releases/latest).

**Linux (amd64):**

```bash
curl -L -o rechta https://github.com/sparkrew/rechta/releases/latest/download/rechta-linux-amd64
```
```bash
chmod +x rechta
```
```bash
sudo mv rechta /usr/local/bin/
```

**Linux (arm64):**

```bash
curl -L -o rechta https://github.com/sparkrew/rechta/releases/latest/download/rechta-linux-arm64
```
```bash
chmod +x rechta
```
```bash
sudo mv rechta /usr/local/bin/
```

**macOS (Apple Silicon):**

```bash
curl -L -o rechta https://github.com/sparkrew/rechta/releases/latest/download/rechta-darwin-arm64
```
```bash
chmod +x rechta
```
```bash
sudo mv rechta /usr/local/bin/
```

**macOS (Intel):**

```bash
curl -L -o rechta https://github.com/sparkrew/rechta/releases/latest/download/rechta-darwin-amd64
```
```bash
chmod +x rechta
```
```bash
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
```
```bash
cd rechta
```
```bash
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
| `--url` | `-u` | | GitHub repository URL (overrides `-w` and `-f`) |
| `--output` | `-o` | | Save output to a file (see below) |
| `--token` | `-t` | `$GITHUB_TOKEN` env | GitHub API token for authentication |
| `--format` | | `json` | Output format: `txt`, `json`, or `html` |
| `--reused-actions` | | `false` | Output a flat JSON list of unique external actions with repo metadata |
| `--depth` | | `10` | Maximum transitive dependency depth |

For `txt` and `json`, output is printed to the terminal. The `-o` flag additionally saves it to a file:

- `-o` (no value): saves to `./dependency-tree.json` (or `./dependency-tree.txt` with `-format txt`)
- `-o path/to/file.json`: saves to the specified path

For `html`, output is written only to a file (nothing on stdout). The default path is `./dependency-tree.html`, or the path given with `-o`.

With `-reused-actions`, output is always a flat JSON array (the `-format` flag is ignored). Use `-o` to save to `./reused-actions.json` by default.

### Authentication

rechta calls the GitHub REST API to resolve action references. Without a token, you are limited to **60 requests per hour**. Set a token to get **5,000 requests per hour**:

```bash
export GITHUB_TOKEN=ghp_your_token_here
```
```bash
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

**Unique reused actions with repo metadata:**

```bash
rechta -w .github/workflows -reused-actions
```

Produces a flat JSON array of every unique external `uses` reference (direct and transitive), with repository metrics:

```json
[
  {
    "uses": "actions/checkout@v4",
    "contributors": 123,
    "stars": 5800,
    "released_on": "2019-08-08"
  }
]
```

- Local actions (`./path`) are excluded.
- `contributors` and `stars` are fetched once per repository (shared across versions).
- `released_on` is the publish date of the referenced version (`YYYY-MM-DD`). Lookup order: exact GitHub Release tag, annotated git tag date, then the GitHub Release whose tag resolves to the action's commit SHA (covers major-version refs like `@v4`). Omitted when none apply.

This mode makes additional GitHub API calls (~2 per unique repository plus 1 per unique `uses` for `released_on`). A `GITHUB_TOKEN` is strongly recommended.

**JSON output (default):**

```bash
rechta -w .github/workflows
```

Each dependency object includes:

- `already_visited` — `false` when the action was fully resolved in this run; `true` when the same `uses` reference was seen earlier (deduplicated stub from cache, no nested dependencies re-resolved).
- `content_sha256` — lowercase hex SHA-256 of the YAML file used for analysis (the same bytes returned by the GitHub API or read from disk): `action.yml` / `action.yaml` for actions, or the workflow file path for reusable workflows. Omitted when the file could not be loaded.
- `content_path` — full path to the analyzed metadata file: `{uses}/{file}` (forward slashes), e.g. `actions/checkout@v4/action.yml`, `./my-local-action/action.yml`, `org/repo/.github/workflows/reuse.yml@main/.github/workflows/reuse.yml`.
- For **node** actions, only the metadata file is hashed (`action.yml`), not `index.js` or other runtime sources.

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
          "sha": "34e114876b0b11c390a56381ad16ebd13914f8d5",
          "type": "node",
          "already_visited": false,
          "content_sha256": "5349b6eea0a1797a9a993c48db9d95b33d27ee1b6227ceced0c9cbf8e655c939",
          "content_path": "actions/checkout@v4/action.yml"
        },
        {
          "ref": {
            "owner": "actions",
            "repo": "setup-go",
            "ref": "v5",
            "uses": "actions/setup-go@v5"
          },
          "sha": "40f1582b2485089dde7abd97c1529aa768e1baff",
          "type": "node",
          "already_visited": true,
          "content_sha256": "ebb00c4462c87740b2cd811941f13ecb03a9d1b417a0db745c5c2df858cdebea",
          "content_path": "actions/checkout@v4/action.yml"
        },
        {
          "ref": {
            "uses": "./my-local-action",
            "is_local": true,
            "local_path": "./my-local-action"
          },
          "sha": "",
          "type": "composite",
          "already_visited": false,
          "content_sha256": "...",
          "content_path": "./my-local-action/action.yml",
          "dependencies": [...]
        }
      ]
    }
  ]
}
```

**Interactive HTML report:**

```bash
rechta -w .github/workflows -format html
# writes ./dependency-tree.html — open in your browser
```

```bash
rechta -w .github/workflows -format html -o reports/deps.html
```

The HTML file is self-contained (no CDN): collapsible trees per workflow, type badges, short SHAs, deduplication markers, and a details panel with full SHA, content hash, and GitHub links. Progress messages go to stderr.

**Save output to a file:**

```bash
rechta -w .github/workflows -o                # saves to ./dependency-tree.json
```
```bash
rechta -w .github/workflows -o tree.json      # saves to ./tree.json
```
```bash
rechta -w .github/workflows -format txt -o    # saves to ./dependency-tree.txt
```

For `txt` and `json`, output is always printed to the terminal as well. You can also use shell redirection (`> file.json`) since progress messages go to stderr.

**Limit depth:**

```bash
rechta -w .github/workflows -depth 1
```

**Point at any repository:**

```bash
rechta -w /path/to/any-repo/.github/workflows
```

**Analyze a remote GitHub repository (latest default branch):**

```bash
rechta -u https://github.com/owner/repo
```

**Analyze a specific version (tag, branch, or commit):**

```bash
rechta -u https://github.com/owner/repo/tree/v1.0.0
rechta -u https://github.com/owner/repo/tree/main
rechta -u https://github.com/owner/repo/commit/abc123def456...
rechta -u https://github.com/owner/repo/releases/tag/v1.0.0
```

**Single workflow file from a remote repository:**

```bash
rechta -u https://github.com/owner/repo/blob/main/.github/workflows/ci.yml
```

When using `-u`, workflows and local actions (`./path`) are fetched via the GitHub API at the resolved commit SHA. A `GITHUB_TOKEN` is strongly recommended for remote analysis.

## What it detects

| Action type | Resolved? | Transitive deps? |
|-------------|-----------|-------------------|
| Standard actions (`owner/repo@ref`) | Yes | N/A (leaf node) |
| Composite actions (`runs.using: composite`) | Yes | Yes -- parses `steps[].uses` recursively |
| Reusable workflows (`owner/repo/.github/workflows/x.yml@ref`) | Yes | Yes -- parses `jobs.*.uses` and `jobs.*.steps[].uses` |
| Local actions (`./path`) | Yes (directory mode) | Yes -- reads `action.yml` from filesystem, walks transitive deps |
| Docker actions (`docker://`) | Skipped | Skipped -- container registries have their own integrity mechanisms |

**Note:** Local actions are fully resolved in directory mode (`-w`) and when using a GitHub URL (`-u`). In single-file mode (`-f`), they appear in the tree but their metadata is not read.

## How it works

1. Discovers `.yml` and `.yaml` files in the workflows directory (or parses a single file via `-f`)
2. Parses each workflow and extracts all `uses:` references from jobs and steps
3. For each remote reference, resolves the tag/branch to a commit SHA via the GitHub Git Data API
4. Fetches the action's `action.yml` (or reusable workflow YAML) at that SHA via the Contents API
5. For each local reference (`./path`), reads `action.yml`/`action.yaml` from the filesystem
6. Records `content_sha256` (SHA-256 of those YAML bytes) and `content_path` (full `{uses}/{file}` path)
7. If the action is composite, extracts nested `uses:` references from its steps and recurses
8. Deduplicates by raw `uses:` string across all workflows (`already_visited: true` on later occurrences)
9. Enforces a configurable depth limit (default 10, matching the GitHub Actions runner)

## Project structure

```
cmd/rechta/   -- CLI entry point
resolver/     -- GitHub API client and recursive dependency resolution
tree/         -- Text, JSON, and HTML output formatters
models/       -- Go structs for workflow and action metadata YAML
parser/       -- YAML parsing for workflow files and action metadata
```

## Testing

```bash
go test -v -race ./...
```

## License

[MIT](LICENSE)
