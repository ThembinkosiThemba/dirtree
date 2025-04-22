package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TreeNode represents a file or directory in the tree
type TreeNode struct {
	Name     string
	IsDir    bool
	Children []*TreeNode
}

func main() {
	// Parse command line arguments
	dirPath := flag.String("path", ".", "Directory path to generate tree from")
	outputFile := flag.String("output", "tree_output.md", "Output file path")
	flag.Parse()

	// Create root node
	root := &TreeNode{
		Name:  filepath.Base(*dirPath),
		IsDir: true,
	}

	// Walk through directory
	err := filepath.WalkDir(*dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == *dirPath {
			return nil
		}

		// Get relative path from the root
		relPath, err := filepath.Rel(*dirPath, path)
		if err != nil {
			return err
		}

		// Add node to the tree
		addNodeToTree(root, relPath, d.IsDir())
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Sort children at each level
	sortTree(root)

	// Generate tree representation
	treeStr := generateTree(root, "", true)

	// Print to terminal
	fmt.Println(treeStr)

	// Save to file
	err = os.WriteFile(*outputFile, []byte("```\n"+treeStr+"```\n"), 0644)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Tree structure saved to %s\n", *outputFile)
}

// addNodeToTree adds a node to the tree based on its path
func addNodeToTree(root *TreeNode, path string, isDir bool) {
	parts := strings.Split(path, string(os.PathSeparator))
	current := root

	// Navigate through each part of the path
	for i, part := range parts {
		isLastPart := i == len(parts)-1
		found := false

		// Check if this part already exists in the children
		for _, child := range current.Children {
			if child.Name == part {
				current = child
				found = true
				break
			}
		}

		// If not found, create a new node
		if !found {
			newNode := &TreeNode{
				Name:  part,
				IsDir: !isLastPart || isDir,
			}
			current.Children = append(current.Children, newNode)
			current = newNode
		}
	}
}

// sortTree sorts the children of each node alphabetically, with directories first
func sortTree(node *TreeNode) {
	// Sort children
	sort.Slice(node.Children, func(i, j int) bool {
		// If one is a directory and the other is not, directory comes first
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		// Otherwise, sort alphabetically
		return node.Children[i].Name < node.Children[j].Name
	})

	// Recursively sort children's children
	for _, child := range node.Children {
		sortTree(child)
	}
}

// generateTree creates a string representation of the tree
func generateTree(node *TreeNode, prefix string, isLast bool) string {
	var result strings.Builder

	// Add the current node
	if node.Name != "" { // Skip for artificial root
		result.WriteString(prefix)
		
		if isLast {
			result.WriteString("└── ")
			prefix += "    "
		} else {
			result.WriteString("├── ")
			prefix += "│   "
		}
		
		result.WriteString(node.Name)
		if node.IsDir {
			result.WriteString("/")
		}
		result.WriteString("\n")
	}

	// Process children
	for i, child := range node.Children {
		isChildLast := i == len(node.Children)-1
		result.WriteString(generateTree(child, prefix, isChildLast))
	}

	return result.String()
}