package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Logger struct {
	Verbose bool
}

var log Logger

// CodeNode represents a node in the code structure tree
type CodeNode struct {
	Name       string
	Type       string // "package", "function", "method", "interface", etc.
	FilePath   string
	Children   []*CodeNode
	CalledBy   []*CodeNode
	Calls      []*CodeNode
	Implements string
	Receiver   string // For methods
}

// TreeNode represents a file or directory in the tree
type TreeNode struct {
	Name     string
	IsDir    bool
	Children []*TreeNode
}

type TreeRenderer interface {
	RenderNode(output *strings.Builder, prefix string, isLast bool)
	GetChildren() []TreeRenderer
	GetName() string
}

// Global map to store all functions for easier lookup during relationship building
var allNodes map[string]*CodeNode

func main() {
	// Parsing command line flags
	repoPath := flag.String("path", ".", "Path to the Go repository to analyze")
	outputFile := flag.String("output", "code_structure.md", "Output file path")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")

	flag.Parse()

	log = Logger{Verbose: *verbose}

	log.Info("Starting code structure analysis for: %s", *repoPath)

	allNodes = make(map[string]*CodeNode)

	stats := generateProjectStats(*repoPath)

	log.Info("Identifying module info...")
	moduleInfo, err := findModuleInfo(*repoPath)
	if err != nil {
		fmt.Printf("Error finding module info: %v\n", err)
	}

	// Step 2: Build project structure
	log.Info("Building project structure...")

	dirRoot, codeRoot, err := buildProjectStructure(*repoPath)
	if err != nil {
		fmt.Printf("Error building project structure: %v\n", err)
		os.Exit(1)
	}

	log.Info("Finding and identifying main packages (entry points)...")
	mainPackages, err := findMainPackages(*repoPath)
	if err != nil {
		fmt.Printf("Error finding main packages: %v\n", err)
	}

	// Step 4: Analyze function calls and build relationships
	log.Info("Analysing function calls...")
	callCounts := analyzeFunctionCalls(*repoPath)

	// Step 5: Generate and output the report
	log.Info("Creating report structure...")
	treeOutput := generateStructureDoc(codeRoot, dirRoot, moduleInfo, mainPackages, callCounts, stats)

	err = os.WriteFile(*outputFile, []byte(treeOutput), 0644)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		os.Exit(1)
	}

	log.Info(fmt.Sprintf("Code structure saved to %s", *outputFile))
}

func generateProjectStats(repoPath string) map[string]int {
	stats := map[string]int{
		"totalFiles":  0,
		"goFiles":     0,
		"packages":    0,
		"functions":   0,
		"methods":     0,
		"structs":     0,
		"interfaces":  0,
		"loc":         0, // lines of code
		"directories": 0,
		"testFiles":   0,
		"nonGoFiles":  0,
	}

	// Track unique packages
	uniquePackages := make(map[string]bool)

	// Walk through the repository
	filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip root directory itself
		if path == repoPath {
			return nil
		}

		// Skip .git directories
		if d.IsDir() && (d.Name() == ".git" || strings.Contains(path, "/.git/")) {
			return filepath.SkipDir
		}

		// Skip vendor directories
		if d.IsDir() && (d.Name() == "vendor" || strings.Contains(path, "/vendor/")) {
			return filepath.SkipDir
		}

		// Count directories
		if d.IsDir() {
			stats["directories"]++
			return nil
		}

		// Count files
		stats["totalFiles"]++

		// Analyze Go files
		if strings.HasSuffix(path, ".go") {
			stats["goFiles"]++

			// Check if it's a test file
			if strings.HasSuffix(path, "_test.go") {
				stats["testFiles"]++
			}

			// Parse file to count code elements and LOC
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err == nil {
				// Count package
				packageName := file.Name.Name
				packagePath := filepath.Dir(path)
				packageKey := packagePath + ":" + packageName
				if !uniquePackages[packageKey] {
					uniquePackages[packageKey] = true
					stats["packages"]++
				}

				// Count LOC (approximately)
				content, err := os.ReadFile(path)
				if err == nil {
					stats["loc"] += len(strings.Split(string(content), "\n"))
				}

				// Count declarations
				for _, decl := range file.Decls {
					switch d := decl.(type) {
					case *ast.FuncDecl:
						if d.Recv != nil {
							stats["methods"]++
						} else {
							stats["functions"]++
						}
					case *ast.GenDecl:
						for _, spec := range d.Specs {
							if typeSpec, ok := spec.(*ast.TypeSpec); ok {
								switch typeSpec.Type.(type) {
								case *ast.StructType:
									stats["structs"]++
								case *ast.InterfaceType:
									stats["interfaces"]++
								}
							}
						}
					}
				}
			}
		} else {
			stats["nonGoFiles"]++
		}

		return nil
	})

	return stats
}

// Info logs informational messages
func (l *Logger) Info(format string, args ...interface{}) {
	fmt.Printf("INFO: "+format+"\n", args...)
}

// Debug logs debug messages only when verbose mode is enabled
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.Verbose {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

// Error logs error messages
func (l *Logger) Error(format string, args ...interface{}) {
	fmt.Printf("ERROR: "+format+"\n", args...)
}

func (n *CodeNode) RenderNode(output *strings.Builder, prefix string, isLast bool) {
	if n.Name == "" {
		return
	}

	nodePrefix := prefix
	if isLast {
		output.WriteString(prefix + "└── ")
		nodePrefix += "    "
	} else {
		output.WriteString(prefix + "├── ")
		nodePrefix += "│   "
	}

	// Format based on node type
	switch n.Type {
	case "repository":
		output.WriteString(fmt.Sprintf("%s/\n", n.Name))
	case "package":
		output.WriteString(fmt.Sprintf("%s (%s)\n", n.Name, n.FilePath))
	case "function":
		output.WriteString(fmt.Sprintf("func %s()\n", n.Name))
	case "method":
		output.WriteString(fmt.Sprintf("func (%s) %s()\n", n.Receiver, n.Name))
	case "struct":
		output.WriteString(fmt.Sprintf("struct %s\n", n.Name))
	case "interface":
		output.WriteString(fmt.Sprintf("interface %s\n", n.Name))
	default:
		output.WriteString(fmt.Sprintf("%s (%s)\n", n.Name, n.Type))
	}

	// Processing children
	renderChildren(output, n.GetChildren(), nodePrefix)
}

func (n *CodeNode) GetChildren() []TreeRenderer {
	result := make([]TreeRenderer, len(n.Children))
	for i, child := range n.Children {
		result[i] = child
	}
	return result
}

func (n *CodeNode) GetName() string {
	return n.Name
}

func (n *TreeNode) RenderNode(output *strings.Builder, prefix string, isLast bool) {
	if n.Name == "" {
		return
	}

	nodePrefix := prefix
	if isLast {
		output.WriteString(prefix + "└── ")
		nodePrefix += "    "
	} else {
		output.WriteString(prefix + "├── ")
		nodePrefix += "│   "
	}

	output.WriteString(n.Name)
	if n.IsDir {
		output.WriteString("/")
	}
	output.WriteString("\n")

	// Process children using common rendering mechanism
	renderChildren(output, n.GetChildren(), nodePrefix)
}

func (n *TreeNode) GetChildren() []TreeRenderer {
	result := make([]TreeRenderer, len(n.Children))
	for i, child := range n.Children {
		result[i] = child
	}
	return result
}

func (n *TreeNode) GetName() string {
	return n.Name
}

func renderChildren(output *strings.Builder, children []TreeRenderer, prefix string) {
	for i, child := range children {
		isLast := i == len(children)-1
		child.RenderNode(output, prefix, isLast)
	}
}

func renderTree(output *strings.Builder, node TreeRenderer, prefix string, isLast bool) {
	node.RenderNode(output, prefix, isLast)
}

func buildProjectStructure(repoPath string) (*TreeNode, *CodeNode, error) {
	// first start by creating root nodes
	repoName := filepath.Base(repoPath)
	dirRoot := &TreeNode{
		Name:  repoName,
		IsDir: true,
	}

	codeRoot := &CodeNode{
		Name: repoName,
		Type: "repository",
	}

	// Initialize package map to avoid duplicates
	packages := make(map[string]*CodeNode)

	// Walk through the repository once
	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == repoPath {
			return nil
		}

		// Get relative path from the root
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}

		// Skip .git directories completely
		if d.IsDir() && (d.Name() == ".git" || strings.Contains(relPath, "/.git/")) {
			return filepath.SkipDir
		}

		// Skip vendor directories completely
		if d.IsDir() && (d.Name() == "vendor" || strings.Contains(relPath, "/vendor/")) {
			return filepath.SkipDir
		}

		// Add to directory structure
		addNodeToTree(dirRoot, relPath, d.IsDir())

		// Process Go files for code structure
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			processGoFile(path, relPath, codeRoot, packages)
		}

		return nil
	})

	// Sort directory tree
	sortTree(dirRoot)

	return dirRoot, codeRoot, err
}

func processGoFile(path, relPath string, codeRoot *CodeNode, packages map[string]*CodeNode) {
	// Parse Go file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return // Skip files with parsing errors
	}

	// Get package name and create package node if it doesn't exist
	packageName := file.Name.Name
	packagePath := filepath.Dir(relPath)
	packageKey := packagePath + ":" + packageName

	var packageNode *CodeNode
	if existingNode, exists := packages[packageKey]; exists {
		packageNode = existingNode
	} else {
		packageNode = &CodeNode{
			Name:     packageName,
			Type:     "package",
			FilePath: packagePath,
		}
		packages[packageKey] = packageNode
		codeRoot.Children = append(codeRoot.Children, packageNode)
	}

	// Process declarations in the file
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			functionNode := processFunction(d, relPath)
			if functionNode != nil {
				packageNode.Children = append(packageNode.Children, functionNode)

				// Add to global map for relationship building later
				nodeName := packageKey + ":" + functionNode.Name
				if functionNode.Receiver != "" {
					nodeName = packageKey + ":" + functionNode.Receiver + "." + functionNode.Name
				}
				allNodes[nodeName] = functionNode
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					typeNode := processType(typeSpec, relPath)
					if typeNode != nil {
						packageNode.Children = append(packageNode.Children, typeNode)

						// Add to global map
						nodeName := packageKey + ":" + typeNode.Name
						allNodes[nodeName] = typeNode
					}
				}
			}
		}
	}
}

// Update processFunction to not repeat filepath.Rel operations
func processFunction(funcDecl *ast.FuncDecl, relPath string) *CodeNode {
	// Create function node
	functionNode := &CodeNode{
		Name:     funcDecl.Name.Name,
		Type:     "function",
		FilePath: relPath,
	}

	// Check if it's a method
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		functionNode.Type = "method"

		// Get receiver type
		if expr, ok := funcDecl.Recv.List[0].Type.(*ast.StarExpr); ok {
			// Pointer receiver
			if ident, ok := expr.X.(*ast.Ident); ok {
				functionNode.Receiver = ident.Name
			}
		} else if ident, ok := funcDecl.Recv.List[0].Type.(*ast.Ident); ok {
			// Value receiver
			functionNode.Receiver = ident.Name
		}
	}

	return functionNode
}

// Update processType to not repeat filepath.Rel operations
func processType(typeSpec *ast.TypeSpec, relPath string) *CodeNode {
	// Determine type kind
	var typeKind string
	switch typeSpec.Type.(type) {
	case *ast.StructType:
		typeKind = "struct"
	case *ast.InterfaceType:
		typeKind = "interface"
	default:
		typeKind = "type"
	}

	// Create type node
	typeNode := &CodeNode{
		Name:     typeSpec.Name.Name,
		Type:     typeKind,
		FilePath: relPath,
	}

	return typeNode
}

// findModuleInfo attempts to find the go.mod file and extract module information
func findModuleInfo(repoPath string) (string, error) {
	goModPath := filepath.Join(repoPath, "go.mod")

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// findMainPackages finds all packages with main functions (entry points)
func findMainPackages(repoPath string) ([]string, error) {
	var mainPackages []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if !info.IsDir() && !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip directories that are not Go code directories
		if info.IsDir() && (info.Name() == "vendor" || info.Name() == ".git") {
			return filepath.SkipDir
		}

		// Check for main function in Go files
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return nil // Skip files with parsing errors
			}

			packageName := file.Name.Name
			if packageName == "main" {
				// Check if it contains a main function
				for _, decl := range file.Decls {
					if fn, ok := decl.(*ast.FuncDecl); ok {
						if fn.Name.Name == "main" && fn.Recv == nil {
							relPath, _ := filepath.Rel(repoPath, filepath.Dir(path))
							mainPackages = append(mainPackages, relPath)
							break
						}
					}
				}
			}
		}

		return nil
	})

	return mainPackages, err
}

// analyzeFunctionCalls performs static analysis to build a graph of function calls
// It uses Go's AST to accurately identify function and method calls across the codebase
// Returns a map of the most frequently called functions, sorted by call count
// Takes the repository path and returns an error if parsing fails
func analyzeFunctionCalls(repoPath string) map[string]int {
	// Track call counts for functions
	callCounts := make(map[string]int)

	// Walk through the repository
	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if !info.IsDir() && !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip vendor and .git directories
		if info.IsDir() && (info.Name() == "vendor" || info.Name() == ".git") {
			return filepath.SkipDir
		}

		// Process Go files to find function calls
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
			if err != nil {
				return nil // Skip files with parsing errors
			}

			// Extract package info
			packageName := file.Name.Name
			packagePath, _ := filepath.Rel(repoPath, filepath.Dir(path))
			packageKey := packagePath + ":" + packageName

			// Map to store imports for resolving function calls
			importMap := buildImportMap(file)

			// Track scope and current function
			var currentFunc *ast.FuncDecl
			var currentFuncKey string

			// Visit all nodes in the AST
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					// Track which function we're currently in
					currentFunc = node
					currentFuncKey = buildFunctionKey(packageKey, node)
					return true

				case *ast.CallExpr:
					// Skip if we're not in a function
					if currentFunc == nil {
						return true
					}

					// Resolve the called function
					calledFuncKey := resolveCallExpr(node, packageKey, importMap)
					if calledFuncKey != "" {
						// Update call count
						callCounts[calledFuncKey]++

						// Establish the relationship between functions
						if currentNode, exists := allNodes[currentFuncKey]; exists {
							if calledNode, exists := allNodes[calledFuncKey]; exists {
								// Check if this relationship already exists
								if !functionCallExists(currentNode, calledNode) {
									currentNode.Calls = append(currentNode.Calls, calledNode)
									calledNode.CalledBy = append(calledNode.CalledBy, currentNode)
								}
							}
						}
					}
				}
				return true
			})
		}
		return nil
	})

	// Find most called functions
	type FunctionCallCount struct {
		Name  string
		Count int
	}

	var mostCalled []FunctionCallCount
	for funcKey, count := range callCounts {
		mostCalled = append(mostCalled, FunctionCallCount{
			Name:  funcKey,
			Count: count,
		})
	}

	// Sort by call count in descending order
	sort.Slice(mostCalled, func(i, j int) bool {
		return mostCalled[i].Count > mostCalled[j].Count
	})

	return callCounts
}

// generateStructureTree creates the final output as a tree
func generateStructureDoc(codeRoot *CodeNode, dirRoot *TreeNode, moduleInfo string,
	mainPackages []string, callCounts map[string]int, stats map[string]int) string {
	var output strings.Builder

	// Add header with improved formatting
	output.WriteString("## Code Structure Analysis\n\n")
	output.WriteString("*Created at: " + time.Now().Format("Jan 02, 2006 15:04:05") + "*\n\n")

	// Add project stats section
	output.WriteString("### Project Statistics\n\n")
	output.WriteString("| Metric | Count |\n")
	output.WriteString("|--------|------:|\n")
	output.WriteString(fmt.Sprintf("| Go Files | %d |\n", stats["goFiles"]))
	output.WriteString(fmt.Sprintf("| Packages | %d |\n", stats["packages"]))
	output.WriteString(fmt.Sprintf("| Functions | %d |\n", stats["functions"]))
	output.WriteString(fmt.Sprintf("| Methods | %d |\n", stats["methods"]))
	output.WriteString(fmt.Sprintf("| Structs | %d |\n", stats["structs"]))
	output.WriteString(fmt.Sprintf("| Interfaces | %d |\n", stats["interfaces"]))
	output.WriteString(fmt.Sprintf("| Test Files | %d |\n", stats["testFiles"]))
	output.WriteString(fmt.Sprintf("| Directories | %d |\n", stats["directories"]))
	output.WriteString(fmt.Sprintf("| Total Lines of Code | %d |\n", stats["loc"]))
	output.WriteString(fmt.Sprintf("| Non-Go Files | %d |\n", stats["nonGoFiles"]))
	output.WriteString(fmt.Sprintf("| Total Files | %d |\n\n", stats["totalFiles"]))

	// Add module info with better formatting
	if moduleInfo != "" {
		output.WriteString("### Module Information\n\n")
		output.WriteString(fmt.Sprintf("```bash\nmodule %s\n```\n\n", moduleInfo))
	}

	// Add entry points with improved formatting
	if len(mainPackages) > 0 {
		output.WriteString("### Entry Points\n\n")
		for i, mainPkg := range mainPackages {
			output.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, mainPkg))
		}
		output.WriteString("\n")
	}

	// Add directory structure with collapsible section
	output.WriteString("Directory Structure\n\n")
	output.WriteString("```bash\n")
	renderTree(&output, dirRoot, "", true)
	output.WriteString("```\n</details>\n\n")

	// Add code structure with collapsible section
	output.WriteString("Code Structure\n\n")
	output.WriteString("```bash\n")
	renderTree(&output, codeRoot, "", true)
	output.WriteString("```\n</details>\n\n")

	// Add function call graph with improved formatting
	output.WriteString("## Function Call Graph\n\n")
	output.WriteString("View Function Call Graph\n\n")
	output.WriteString("```mermaid\ngraph TD\n")
	renderFunctionCallGraph(&output, allNodes)
	output.WriteString("```\n</details>\n\n")

	addMostCalledFunctionsToOutput(&output, callCounts)
	// Add footer
	output.WriteString("\n---\n*This document was automatically generated by the Go Code Structure Analyzer*\n")

	return output.String()
}

// renderFunctionCallGraph renders the function call graph in Mermaid format
func renderFunctionCallGraph(output *strings.Builder, nodes map[string]*CodeNode) {
	// Add a node for each function
	for key, node := range nodes {
		if node.Type == "function" || node.Type == "method" {
			// Clean key for Mermaid
			cleanKey := strings.ReplaceAll(key, ":", "_")
			cleanKey = strings.ReplaceAll(cleanKey, ".", "_")
			cleanKey = strings.ReplaceAll(cleanKey, "/", "_")

			// Node definition
			var label string
			if node.Type == "method" {
				label = fmt.Sprintf("%s.%s", node.Receiver, node.Name)
			} else {
				label = node.Name
			}
			output.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", cleanKey, label))

			// Edges for function calls
			for _, calledNode := range node.Calls {
				// Find the key for the called node
				var calledKey string
				for k, n := range nodes {
					if n == calledNode {
						calledKey = k
						break
					}
				}

				if calledKey != "" {
					cleanCalledKey := strings.ReplaceAll(calledKey, ":", "_")
					cleanCalledKey = strings.ReplaceAll(cleanCalledKey, ".", "_")
					cleanCalledKey = strings.ReplaceAll(cleanCalledKey, "/", "_")

					output.WriteString(fmt.Sprintf("    %s --> %s\n", cleanKey, cleanCalledKey))
				}
			}
		}
	}
}

// addNodeToTree adds a node to the tree based on its path
func addNodeToTree(root *TreeNode, path string, isDir bool) {
	parts := strings.Split(path, string(os.PathSeparator))
	current := root

	// Navifating through each path of the path
	for i, part := range parts {
		isLastPart := i == len(parts)-1
		found := false

		// checking if this point exists in the children
		for _, child := range current.Children {
			if child.Name == part {
				current = child
				found = true
				break
			}
		}

		// if not found, create a new node
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
		// if one is a directory and the other is not, directory comes first
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

// buildImportMap creates a map of import aliases to their full package paths
func buildImportMap(file *ast.File) map[string]string {
	importMap := make(map[string]string)

	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		var importName string

		// Get the import alias if specified, otherwise use the last part of the path
		if imp.Name != nil {
			importName = imp.Name.Name
		} else {
			pathParts := strings.Split(importPath, "/")
			importName = pathParts[len(pathParts)-1]
		}

		importMap[importName] = importPath
	}

	return importMap
}

// buildFunctionKey creates a unique key for a function
func buildFunctionKey(packageKey string, funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		receiverName := getReceiverTypeName(funcDecl.Recv.List[0].Type)
		if receiverName != "" {
			return packageKey + ":" + receiverName + "." + funcDecl.Name.Name
		}
	}
	return packageKey + ":" + funcDecl.Name.Name
}

// getReceiverTypeName gets the type name from a receiver
func getReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// resolveFunctionCall determines the actual function being called from a CallExpr
// Handles various call types: direct calls, method calls, package-qualified calls
// Returns function identifier and true if successfully resolved, empty string and false otherwise
// Takes the AST CallExpr node, current package info, and import aliases as inputs
func resolveCallExpr(callExpr *ast.CallExpr, packageKey string, importMap map[string]string) string {
	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		// Direct function call in the same package
		return packageKey + ":" + fun.Name

	case *ast.SelectorExpr:
		// Package.Function or Value.Method
		if x, ok := fun.X.(*ast.Ident); ok {
			// Check if this is a package reference
			if importPath, exists := importMap[x.Name]; exists {
				// This is a function from an imported package
				return importPath + ":" + fun.Sel.Name
			}

			// This could be a method call on a variable
			return packageKey + ":" + x.Name + "." + fun.Sel.Name
		}
	}

	return "" // Unknown call type
}

// functionCallExists checks if a function call relationship already exists
func functionCallExists(caller *CodeNode, callee *CodeNode) bool {
	for _, call := range caller.Calls {
		if call == callee {
			return true
		}
	}
	return false
}

// Add a new function to enrich the output with most called functions
func addMostCalledFunctionsToOutput(output *strings.Builder, callCounts map[string]int) {
	output.WriteString("\n## Most Called Functions\n\n")
	output.WriteString("| Function | Type | File | Call Count |\n")
	output.WriteString("|----------|------|------|------------|\n")

	type FunctionCallCount struct {
		Key   string
		Count int
	}

	var mostCalled []FunctionCallCount
	for funcKey, count := range callCounts {
		mostCalled = append(mostCalled, FunctionCallCount{
			Key:   funcKey,
			Count: count,
		})
	}

	// Sort by call count in descending order
	sort.Slice(mostCalled, func(i, j int) bool {
		return mostCalled[i].Count > mostCalled[j].Count
	})

	count := 0
	for _, fn := range mostCalled {
		if node, exists := allNodes[fn.Key]; exists {
			var displayName string
			if node.Type == "method" {
				displayName = fmt.Sprintf("(%s) %s", node.Receiver, node.Name)
			} else {
				displayName = node.Name
			}

			output.WriteString(fmt.Sprintf("| %s | %s | %s | %d |\n",
				displayName, node.Type, node.FilePath, fn.Count))
			count++
		}
	}
}
