package tree

import "github.com/sparkrew/rechta/resolver"

// WalkDependencies visits each dependency node in pre-order depth-first traversal.
func WalkDependencies(nodes []*resolver.DependencyNode, fn func(node *resolver.DependencyNode, depth int)) {
	for _, n := range nodes {
		walkNode(n, 0, fn)
	}
}

func walkNode(node *resolver.DependencyNode, depth int, fn func(node *resolver.DependencyNode, depth int)) {
	if node == nil {
		return
	}
	fn(node, depth)
	for _, child := range node.Children {
		walkNode(child, depth+1, fn)
	}
}
