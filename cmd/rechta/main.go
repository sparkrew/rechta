package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sparkrew/rechta/models"
	"github.com/sparkrew/rechta/parser"
	"github.com/sparkrew/rechta/resolver"
	"github.com/sparkrew/rechta/tree"
)

const (
	defaultOutputJSON = "dependency-tree.json"
	defaultOutputTxt  = "dependency-tree.txt"
	defaultOutputHTML = "dependency-tree.html"
)

func main() {
	workflows := flag.String("workflows", ".github/workflows", "Path to workflows directory")
	file := flag.String("file", "", "Path to a single workflow file (overrides -workflows)")
	token := flag.String("token", "", "GitHub token (default: GITHUB_TOKEN env var)")
	format := flag.String("format", "json", "Output format: txt, json, or html (default: json)")
	depth := flag.Int("depth", resolver.DefaultMaxDepth, "Maximum transitive dependency depth")
	flag.StringVar(workflows, "w", ".github/workflows", "Path to workflows directory (shorthand)")
	flag.StringVar(file, "f", "", "Path to a single workflow file (shorthand)")
	flag.StringVar(token, "t", "", "GitHub token (shorthand)")

	saveOutput := false
	outputPath := ""
	flag.Func("output", "Save output to file. Without a path: saves to ./dependency-tree.{json|txt|html}", func(val string) error {
		saveOutput = true
		outputPath = val
		return nil
	})
	flag.Func("o", "Save output to file (shorthand for -output)", func(val string) error {
		saveOutput = true
		outputPath = val
		return nil
	})

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "rechta - GitHub Actions dependency tree generator\n\n")
		fmt.Fprintf(os.Stderr, "Usage: rechta [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  rechta -w .github/workflows -format txt\n")
		fmt.Fprintf(os.Stderr, "  rechta -w .github/workflows -format html\n")
		fmt.Fprintf(os.Stderr, "  rechta -f .github/workflows/ci.yml\n")
		fmt.Fprintf(os.Stderr, "  rechta -o                              # saves to ./dependency-tree.json\n")
		fmt.Fprintf(os.Stderr, "  rechta -o my-tree.json                 # saves to ./my-tree.json\n")
		fmt.Fprintf(os.Stderr, "\nSet GITHUB_TOKEN to avoid API rate limits (60 req/hr unauthenticated).\n")
		fmt.Fprintf(os.Stderr, "\nNote: local action references (./path) are only resolved when using\n")
		fmt.Fprintf(os.Stderr, "directory mode (-w), not single-file mode (-f).\n")
	}

	flag.Parse()

	ghToken := *token
	if ghToken == "" {
		ghToken = os.Getenv("GITHUB_TOKEN")
	}

	if *format != "txt" && *format != "json" && *format != "html" {
		fmt.Fprintf(os.Stderr, "Error: unsupported format %q (use \"txt\", \"json\", or \"html\")\n", *format)
		os.Exit(1)
	}

	var wfs []*models.Workflow
	var basePath string

	if *file != "" {
		wf, err := parser.ParseWorkflow(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing workflow %s: %v\n", *file, err)
			os.Exit(1)
		}
		wfs = append(wfs, wf)
		fmt.Fprintf(os.Stderr, "Parsed workflow: %s\n\n", *file)
	} else {
		files, err := resolver.DiscoverWorkflows(*workflows)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering workflows in %s: %v\n", *workflows, err)
			os.Exit(1)
		}
		if len(files) == 0 {
			fmt.Fprintf(os.Stderr, "No workflow files found in %s\n", *workflows)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "Found %d workflow file(s) in %s\n", len(files), *workflows)

		for _, f := range files {
			wf, err := parser.ParseWorkflow(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", f, err)
				continue
			}
			wfs = append(wfs, wf)
		}

		if len(wfs) == 0 {
			fmt.Fprintf(os.Stderr, "No valid workflow files found\n")
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "Parsed %d valid workflow(s)\n\n", len(wfs))
		basePath = "."
	}

	client := resolver.NewGitHubClient(ghToken, 10)
	res := resolver.NewResolver(client, *depth, basePath)

	trees, err := res.ResolveAll(wfs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving dependencies: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr)

	switch *format {
	case "html":
		path := outputPath
		if path == "" {
			path = defaultOutputHTML
		}
		if err := tree.PrintHTML(trees, path); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Wrote %s — open in your browser to explore the dependency tree.\n", path)
	case "txt":
		tree.PrintText(trees, os.Stdout)
		if saveOutput {
			writeTextOrJSONOutput(trees, *format, outputPath)
		}
	default:
		if err := tree.PrintJSON(trees, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
			os.Exit(1)
		}
		if saveOutput {
			writeTextOrJSONOutput(trees, *format, outputPath)
		}
	}
}

func writeTextOrJSONOutput(trees []resolver.WorkflowTree, format, outputPath string) {
	if outputPath == "" {
		if format == "txt" {
			outputPath = defaultOutputTxt
		} else {
			outputPath = defaultOutputJSON
		}
	}

	f, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file %s: %v\n", outputPath, err)
		os.Exit(1)
	}
	defer f.Close()

	switch format {
	case "txt":
		tree.PrintText(trees, f)
	default:
		if err := tree.PrintJSON(trees, f); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "Output saved to %s\n", outputPath)
}
