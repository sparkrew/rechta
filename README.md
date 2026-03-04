# rechta

A tool for extracting the dependency tree of GitHub Actions workflows. It parses workflow and action metadata YAML files, resolves references between jobs, steps, reusable workflows, and actions, and builds a structured dependency graph.

**Status:** The parsing layer is implemented; dependency resolution is in progress.

## Build & Run

```bash
go build ./...
```

## Testing

```bash
go test -v -race ./...
```

## Project Structure

```
models/   – Go structs for workflows, jobs, steps, events, permissions, etc.
parser/   – Parses workflow and action metadata YAML files
```

## License

[MIT](LICENSE)
