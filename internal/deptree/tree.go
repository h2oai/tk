package deptree

import (
	"fmt"
	"sort"
	"strings"

	"github.com/h2oai/tk/internal/ticket"
)

// Node represents a node in the dependency tree
type Node struct {
	ID           string
	Status       ticket.Status
	Title        string
	Deps         []string
	MaxDepth     int // Maximum depth at which this node appears
	SubtreeDepth int // Maximum depth in this node's subtree
}

// Tree represents a dependency tree
type Tree struct {
	root    string
	nodes   map[string]*Node
	full    bool
	printed map[string]bool
}

// Build constructs a dependency tree from the given tickets
func Build(tickets map[string]*ticket.Ticket, rootID string, full bool) *Tree {
	// Convert tickets to nodes
	nodes := make(map[string]*Node)
	for id, t := range tickets {
		nodes[id] = &Node{
			ID:       id,
			Status:   t.Status,
			Title:    t.Title,
			Deps:     t.Deps,
			MaxDepth: -1, // Will be computed
		}
	}

	tree := &Tree{
		root:    rootID,
		nodes:   nodes,
		full:    full,
		printed: make(map[string]bool),
	}

	tree.computeMaxDepths()
	tree.computeSubtreeDepths()

	return tree
}

// computeMaxDepths computes the maximum depth at which each node appears
// using an iterative approach with a stack
func (t *Tree) computeMaxDepths() {
	type stackItem struct {
		id    string
		depth int
		path  string
	}

	stack := []stackItem{{t.root, 0, ":"}}

	for len(stack) > 0 {
		// Pop from stack
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		node, ok := t.nodes[item.id]
		if !ok {
			continue
		}

		// Cycle detection: check if already in path
		pathKey := ":" + item.id + ":"
		if strings.Contains(item.path, pathKey) {
			continue
		}

		// Update max depth
		if item.depth > node.MaxDepth {
			node.MaxDepth = item.depth
		}

		// Push children (in reverse order so they process in order)
		newPath := item.path + item.id + ":"
		for i := len(node.Deps) - 1; i >= 0; i-- {
			dep := node.Deps[i]
			if dep != "" {
				stack = append(stack, stackItem{dep, item.depth + 1, newPath})
			}
		}
	}
}

// computeSubtreeDepths computes the subtree depth for each node
// using iterative post-order traversal
func (t *Tree) computeSubtreeDepths() {
	type stackItem struct {
		id    string
		path  string
		phase int // 0 = first visit, 1 = second visit
	}

	computed := make(map[string]bool)
	stack := []stackItem{{t.root, ":", 0}}

	for len(stack) > 0 {
		item := &stack[len(stack)-1]

		node, ok := t.nodes[item.id]
		if !ok {
			stack = stack[:len(stack)-1]
			continue
		}

		// Cycle detection
		pathKey := ":" + item.id + ":"
		if strings.Contains(item.path, pathKey) {
			stack = stack[:len(stack)-1]
			continue
		}

		if item.phase == 0 {
			// First visit: push children
			item.phase = 1
			newPath := item.path + item.id + ":"

			for i := len(node.Deps) - 1; i >= 0; i-- {
				dep := node.Deps[i]
				if dep != "" && !computed[dep] {
					stack = append(stack, stackItem{dep, newPath, 0})
				}
			}
		} else {
			// Second visit: compute subtree depth
			maxSub := node.MaxDepth
			for _, dep := range node.Deps {
				if depNode, ok := t.nodes[dep]; ok {
					if depNode.SubtreeDepth > maxSub {
						maxSub = depNode.SubtreeDepth
					}
				}
			}
			node.SubtreeDepth = maxSub
			computed[item.id] = true
			stack = stack[:len(stack)-1]
		}
	}
}

// Render prints the dependency tree
func (t *Tree) Render() {
	root, ok := t.nodes[t.root]
	if !ok {
		return
	}

	// Print root
	fmt.Printf("%s [%s] %s\n", root.ID, root.Status, root.Title)
	t.printed[root.ID] = true

	// Render children
	t.renderChildren(t.root, "", ":"+t.root+":", 0)
}

func (t *Tree) renderChildren(id, prefix, path string, depth int) {
	node, ok := t.nodes[id]
	if !ok {
		return
	}

	// Collect printable children
	var children []string
	for _, dep := range node.Deps {
		if dep == "" {
			continue
		}
		depNode, ok := t.nodes[dep]
		if !ok {
			continue
		}
		// Skip if in path (cycle)
		pathKey := ":" + dep + ":"
		if strings.Contains(path, pathKey) {
			continue
		}
		// In normal mode, skip if already printed or not at max depth
		if !t.full {
			if t.printed[dep] {
				continue
			}
			if depth+1 != depNode.MaxDepth {
				continue
			}
		}
		children = append(children, dep)
	}

	if len(children) == 0 {
		return
	}

	// Sort by subtree depth (shallowest first), then by ID
	sort.Slice(children, func(i, j int) bool {
		iNode := t.nodes[children[i]]
		jNode := t.nodes[children[j]]
		if iNode.SubtreeDepth != jNode.SubtreeDepth {
			return iNode.SubtreeDepth < jNode.SubtreeDepth
		}
		return children[i] < children[j]
	})

	// Render each child
	for i, child := range children {
		childNode := t.nodes[child]

		// Determine connector
		var connector string
		if i == len(children)-1 {
			connector = "└── "
		} else {
			connector = "├── "
		}

		// Print child
		fmt.Printf("%s%s%s [%s] %s\n", prefix, connector, childNode.ID, childNode.Status, childNode.Title)

		if !t.full {
			t.printed[child] = true
		}

		// Compute new prefix
		var newPrefix string
		if i == len(children)-1 {
			newPrefix = prefix + "    "
		} else {
			newPrefix = prefix + "│   "
		}

		// Recurse
		t.renderChildren(child, newPrefix, path+child+":", depth+1)
	}
}
