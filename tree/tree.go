package tree

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/sparkrew/rechta/resolver"
)

// PrintText renders workflow dependency trees in a human-readable format
// with Unicode box-drawing characters.
func PrintText(trees []resolver.WorkflowTree, w io.Writer) {
	for i, wt := range trees {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s\n", wt.Path)

		for j, dep := range wt.Dependencies {
			isLast := j == len(wt.Dependencies)-1
			printNode(w, dep, "", isLast)
		}
	}
}

func printNode(w io.Writer, node *resolver.DependencyNode, prefix string, last bool) {
	branch := "+-- "
	if last {
		branch = "`-- "
	}

	var label string
	if node.Ref.IsLocal {
		label = fmt.Sprintf("%s (local)", node.Ref.LocalPath)
	} else {
		shortSHA := node.SHA
		if len(shortSHA) > 12 {
			shortSHA = shortSHA[:12]
		}
		label = fmt.Sprintf("%s@%s (%s)", node.Ref.FullName(), node.Ref.Ref, shortSHA)
	}
	if node.AlreadyVisited {
		label += " *"
	}

	fmt.Fprintf(w, "%s%s%s\n", prefix, branch, label)

	childPrefix := prefix + "|   "
	if last {
		childPrefix = prefix + "    "
	}

	for k, child := range node.Children {
		childLast := k == len(node.Children)-1
		printNode(w, child, childPrefix, childLast)
	}
}

// PrintJSON renders workflow dependency trees as formatted JSON.
func PrintJSON(trees []resolver.WorkflowTree, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Workflows []resolver.WorkflowTree `json:"workflows"`
	}{Workflows: trees})
}

// ReusedActionEntry is one unique external action reference in -reused-actions output.
type ReusedActionEntry struct {
	Uses string `json:"uses"`
}

// PrintReusedActionsJSON renders a flat list of unique reused actions as JSON.
func PrintReusedActionsJSON(uses []string, w io.Writer) error {
	entries := make([]ReusedActionEntry, len(uses))
	for i, u := range uses {
		entries[i] = ReusedActionEntry{Uses: u}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}
