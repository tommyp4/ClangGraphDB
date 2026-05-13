package clangast

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"clang-graphdb/internal/vcxproj"
)

type Extractor struct {
	ClangPath  string
	RepoRoot   string
	Verbose    bool
	Workers    int
	NodesFile  io.Writer
	EdgesFile  io.Writer

	nodesMu sync.Mutex
	edgesMu sync.Mutex

	nodeCount atomic.Int64
	edgeCount atomic.Int64

	// Dedup across files
	emittedNodes sync.Map
	emittedEdges sync.Map

	// Map clang AST node IDs to our graph node IDs within a single file
	idMap sync.Map
}

func NewExtractor(clangPath, repoRoot string, nodesFile, edgesFile io.Writer) *Extractor {
	return &Extractor{
		ClangPath: clangPath,
		RepoRoot:  filepath.Clean(repoRoot),
		Workers:   4,
		NodesFile: nodesFile,
		EdgesFile: edgesFile,
	}
}

func (e *Extractor) NodeCount() int64 { return e.nodeCount.Load() }
func (e *Extractor) EdgeCount() int64 { return e.edgeCount.Load() }

func (e *Extractor) Run(commands []vcxproj.CompileCommand) (succeeded, failed int) {
	total := len(commands)
	work := make(chan vcxproj.CompileCommand, total)
	for _, cmd := range commands {
		work <- cmd
	}
	close(work)

	var mu sync.Mutex
	var wg sync.WaitGroup
	processed := 0

	workers := e.Workers
	if workers < 1 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cmd := range work {
				nodesBefore := e.nodeCount.Load()
				edgesBefore := e.edgeCount.Load()
				err := e.processFile(cmd)
				mu.Lock()
				processed++
				if err != nil {
					failed++
					fmt.Fprintf(os.Stderr, "  [%d/%d] FAIL %s: %v\n", processed, total, filepath.Base(cmd.File), err)
				} else {
					succeeded++
					dn := e.nodeCount.Load() - nodesBefore
					de := e.edgeCount.Load() - edgesBefore
					fmt.Fprintf(os.Stderr, "  [%d/%d] OK   %s (+%d nodes, +%d edges)\n", processed, total, filepath.Base(cmd.File), dn, de)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return
}

func (e *Extractor) processFile(cmd vcxproj.CompileCommand) error {
	args := make([]string, 0, len(cmd.Arguments)+4)
	args = append(args, "-Xclang", "-ast-dump=json")
	if len(cmd.Arguments) > 1 {
		for _, a := range cmd.Arguments[1:] {
			if strings.HasPrefix(a, "/Fo") || strings.HasPrefix(a, "/Fd") ||
				strings.HasPrefix(a, "/Fa") || strings.HasPrefix(a, "-o") {
				continue
			}
			args = append(args, a)
		}
	}

	proc := exec.Command(e.ClangPath, args...)
	proc.Dir = cmd.Directory

	stdout, err := proc.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe: %w", err)
	}
	proc.Stderr = io.Discard

	if err := proc.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	// Stream-parse: only decode top-level "inner" children of TranslationUnitDecl.
	// Each child is decoded one at a time then discarded, keeping memory bounded.
	fileCtx := &fileContext{
		extractor: e,
		filePath:  cmd.File,
		idMap:     make(map[string]string),
	}
	err = fileCtx.streamParse(stdout)

	proc.Wait()
	return err
}

type fileContext struct {
	extractor    *Extractor
	filePath     string
	idMap        map[string]string // clang AST id -> graph node ID
	funcStack    []string          // stack of graph node IDs for enclosing functions
}

func (fc *fileContext) registerDecls(node *ASTNode, parentFQN string) {
	if node.IsImplicit {
		return
	}

	fqn := fc.buildFQN(node, parentFQN)

	switch node.Kind {
	case "FunctionDecl", "CXXMethodDecl", "CXXConstructorDecl", "CXXDestructorDecl":
		if !fc.isInRepo(node) || isSTLInternal(node.Name) {
			break
		}
		sig := fc.buildSignature(node)
		label := "Function"
		if node.Kind == "CXXConstructorDecl" {
			label = "Constructor"
		}
		nodeID := label + ":" + fqn + ":" + sig
		fc.idMap[node.ID] = nodeID

	case "CXXRecordDecl":
		if node.CompleteDefinition && fc.isInRepo(node) && !isSTLInternal(node.Name) {
			nodeID := "Class:" + fqn
			fc.idMap[node.ID] = nodeID
		}

	case "FieldDecl":
		if fc.isInRepo(node) && !isSTLInternal(node.Name) {
			nodeID := "Field:" + fqn
			fc.idMap[node.ID] = nodeID
		}

	case "VarDecl":
		if fc.isFileScope(node) && fc.isInRepo(node) && !isSTLInternal(node.Name) {
			nodeID := "Global:" + fqn
			fc.idMap[node.ID] = nodeID
		}
	}

	for i := range node.Inner {
		fc.registerDecls(&node.Inner[i], fqn)
	}
}

func (fc *fileContext) extractNodes(node *ASTNode, parentFQN string) {
	if node.IsImplicit {
		return
	}

	fqn := fc.buildFQN(node, parentFQN)

	switch node.Kind {
	case "FunctionDecl", "CXXMethodDecl", "CXXConstructorDecl", "CXXDestructorDecl":
		fc.handleFunction(node, fqn)
		return // handleFunction recurses into children itself

	case "CXXRecordDecl":
		fc.handleRecord(node, fqn)

	case "FieldDecl":
		fc.handleField(node, fqn, parentFQN)

	case "VarDecl":
		fc.handleVar(node, fqn)

	case "CallExpr", "CXXMemberCallExpr":
		fc.handleCall(node)

	case "DeclRefExpr":
		fc.handleDeclRef(node)
	}

	for i := range node.Inner {
		fc.extractNodes(&node.Inner[i], fqn)
	}
}

func (fc *fileContext) handleFunction(node *ASTNode, fqn string) {
	nodeID, ok := fc.idMap[node.ID]
	if !ok {
		return
	}

	file := fc.getFile(node)
	if file == "" {
		return
	}
	relPath := fc.makeRelative(file)

	label := "Function"
	if node.Kind == "CXXConstructorDecl" {
		label = "Constructor"
	}

	// Emit node
	fc.emitNode(nodeID, label, map[string]string{
		"name":       node.Name,
		"fqn":        fqn,
		"file":       relPath,
		"start_line": fmt.Sprintf("%d", node.Loc.Line),
		"end_line":   fmt.Sprintf("%d", fc.getEndLine(node)),
	})

	// DEFINED_IN
	fileID := "File:" + relPath
	fc.emitEdge(nodeID, fileID, "DEFINED_IN")

	// HAS_METHOD — if this is a method, look up parent class
	if node.Kind == "CXXMethodDecl" || node.Kind == "CXXConstructorDecl" || node.Kind == "CXXDestructorDecl" {
		// The parent FQN minus the function name gives us the class
		if idx := strings.LastIndex(fqn, "::"); idx >= 0 {
			classFQN := fqn[:idx]
			classID := "Class:" + classFQN
			fc.emitEdge(classID, nodeID, "HAS_METHOD")
		}
	}

	// Push function context and recurse
	fc.funcStack = append(fc.funcStack, nodeID)
	for i := range node.Inner {
		fc.extractNodes(&node.Inner[i], fqn)
	}
	fc.funcStack = fc.funcStack[:len(fc.funcStack)-1]
}

func (fc *fileContext) handleRecord(node *ASTNode, fqn string) {
	if !node.CompleteDefinition {
		return
	}

	nodeID, ok := fc.idMap[node.ID]
	if !ok {
		return
	}

	file := fc.getFile(node)
	if file == "" {
		return
	}
	relPath := fc.makeRelative(file)

	fc.emitNode(nodeID, "Class", map[string]string{
		"name":       node.Name,
		"fqn":        fqn,
		"file":       relPath,
		"start_line": fmt.Sprintf("%d", node.Loc.Line),
		"end_line":   fmt.Sprintf("%d", fc.getEndLine(node)),
	})

	fileID := "File:" + relPath
	fc.emitEdge(nodeID, fileID, "DEFINED_IN")

	// INHERITS edges from bases
	for _, base := range node.Bases {
		if base.Type.QualType == "" {
			continue
		}
		baseName := extractTypeName(base.Type.QualType)
		if baseName != "" {
			baseID := "Class:" + baseName
			fc.emitEdge(nodeID, baseID, "INHERITS")
		}
	}
}

func (fc *fileContext) handleField(node *ASTNode, fqn, parentFQN string) {
	nodeID, ok := fc.idMap[node.ID]
	if !ok {
		return
	}

	file := fc.getFile(node)
	if file == "" {
		return
	}
	relPath := fc.makeRelative(file)

	typeName := ""
	if node.Type != nil {
		typeName = node.Type.QualType
	}

	fc.emitNode(nodeID, "Field", map[string]string{
		"name":       node.Name,
		"fqn":        fqn,
		"file":       relPath,
		"start_line": fmt.Sprintf("%d", node.Loc.Line),
		"cpp_type":   typeName,
	})

	// DEFINES edge from parent class
	if parentFQN != "" {
		classID := "Class:" + parentFQN
		fc.emitEdge(classID, nodeID, "DEFINES")
	}

	// DEPENDS_ON for class-typed fields
	if typeName != "" {
		depName := extractTypeName(typeName)
		if depName != "" && parentFQN != "" {
			ownerID := "Class:" + parentFQN
			depID := "Class:" + depName
			fc.emitEdge(ownerID, depID, "DEPENDS_ON")
		}
	}
}

func (fc *fileContext) handleVar(node *ASTNode, fqn string) {
	if !fc.isFileScope(node) {
		return
	}

	nodeID, ok := fc.idMap[node.ID]
	if !ok {
		return
	}

	file := fc.getFile(node)
	if file == "" {
		return
	}
	relPath := fc.makeRelative(file)

	typeName := ""
	if node.Type != nil {
		typeName = node.Type.QualType
	}

	fc.emitNode(nodeID, "Global", map[string]string{
		"name":       node.Name,
		"fqn":        fqn,
		"file":       relPath,
		"start_line": fmt.Sprintf("%d", node.Loc.Line),
		"cpp_type":   typeName,
	})

	fileID := "File:" + relPath
	fc.emitEdge(nodeID, fileID, "DEFINED_IN")
}

func (fc *fileContext) handleCall(node *ASTNode) {
	if len(fc.funcStack) == 0 {
		return
	}
	callerID := fc.funcStack[len(fc.funcStack)-1]

	// For CXXMemberCallExpr, the callee is referenced via MemberExpr
	// For CallExpr, the callee is referenced via DeclRefExpr
	calleeID := fc.findCalleeID(node)
	if calleeID == "" {
		return
	}

	fc.emitEdge(callerID, calleeID, "CALLS")
}

func (fc *fileContext) handleDeclRef(node *ASTNode) {
	if len(fc.funcStack) == 0 {
		return
	}

	if node.ReferencedDecl == nil {
		return
	}

	ref := node.ReferencedDecl
	if ref.Kind != "VarDecl" {
		return
	}

	refID, ok := fc.idMap[ref.ID]
	if !ok {
		return
	}

	// Only emit USES_GLOBAL for file-scope vars
	if !strings.HasPrefix(refID, "Global:") {
		return
	}

	callerID := fc.funcStack[len(fc.funcStack)-1]
	fc.emitEdge(callerID, refID, "USES_GLOBAL")
}

func (fc *fileContext) findCalleeID(node *ASTNode) string {
	for i := range node.Inner {
		child := &node.Inner[i]
		switch child.Kind {
		case "MemberExpr":
			if child.ReferencedMemberDecl != "" {
				if id, ok := fc.idMap[child.ReferencedMemberDecl]; ok {
					return id
				}
			}
		case "DeclRefExpr":
			if child.ReferencedDecl != nil {
				if id, ok := fc.idMap[child.ReferencedDecl.ID]; ok {
					return id
				}
			}
		case "ImplicitCastExpr":
			if id := fc.findCalleeID(child); id != "" {
				return id
			}
		}
	}
	return ""
}

// Emit helpers with dedup

func (fc *fileContext) emitNode(id, nodeType string, props map[string]string) {
	if name := props["name"]; isSTLInternal(name) {
		return
	}
	if _, loaded := fc.extractor.emittedNodes.LoadOrStore(id, true); loaded {
		return
	}

	out := make(map[string]string, len(props)+2)
	out["id"] = id
	out["type"] = nodeType
	for k, v := range props {
		out[k] = v
	}

	data, _ := json.Marshal(out)
	data = append(data, '\n')

	fc.extractor.nodesMu.Lock()
	fc.extractor.NodesFile.Write(data)
	fc.extractor.nodesMu.Unlock()

	fc.extractor.nodeCount.Add(1)
}

func (fc *fileContext) emitEdge(source, target, edgeType string) {
	if isSTLInternalID(source) || isSTLInternalID(target) {
		return
	}
	key := source + "|" + edgeType + "|" + target
	if _, loaded := fc.extractor.emittedEdges.LoadOrStore(key, true); loaded {
		return
	}

	out := map[string]string{
		"source": source,
		"target": target,
		"type":   edgeType,
	}

	data, _ := json.Marshal(out)
	data = append(data, '\n')

	fc.extractor.edgesMu.Lock()
	fc.extractor.EdgesFile.Write(data)
	fc.extractor.edgesMu.Unlock()

	fc.extractor.edgeCount.Add(1)
}

// Helper methods

func (fc *fileContext) buildFQN(node *ASTNode, parentFQN string) string {
	if node.Name == "" {
		return parentFQN
	}
	switch node.Kind {
	case "FunctionDecl", "CXXMethodDecl", "CXXConstructorDecl", "CXXDestructorDecl",
		"CXXRecordDecl", "FieldDecl", "VarDecl", "NamespaceDecl":
		if parentFQN != "" {
			return parentFQN + "::" + node.Name
		}
		return node.Name
	}
	return parentFQN
}

func (fc *fileContext) buildSignature(node *ASTNode) string {
	var params []string
	for _, child := range node.Inner {
		if child.Kind == "ParmVarDecl" && child.Type != nil {
			params = append(params, child.Type.QualType)
		}
	}
	return "(" + strings.Join(params, ",") + ")"
}

func (fc *fileContext) getFile(node *ASTNode) string {
	if node.Loc.File != "" {
		return node.Loc.File
	}
	// Fallback to the file being processed
	return fc.filePath
}

func (fc *fileContext) getEndLine(node *ASTNode) int {
	if node.Range.End.Line > 0 {
		return node.Range.End.Line
	}
	return node.Loc.Line
}

func (fc *fileContext) isInRepo(node *ASTNode) bool {
	file := fc.getFile(node)
	if file == "" {
		return true // assume it's in the current file
	}
	return fc.isPathInRepo(file)
}

func (fc *fileContext) isPathInRepo(path string) bool {
	normalized := strings.ReplaceAll(filepath.Clean(path), "\\", "/")
	repoNorm := strings.ReplaceAll(fc.extractor.RepoRoot, "\\", "/")
	return strings.HasPrefix(strings.ToLower(normalized), strings.ToLower(repoNorm))
}

func (fc *fileContext) makeRelative(absPath string) string {
	rel, err := filepath.Rel(fc.extractor.RepoRoot, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}

func (fc *fileContext) isFileScope(node *ASTNode) bool {
	return node.StorageClass == "extern" || node.StorageClass == "static" || node.StorageClass == ""
}


// ExtractIncludes processes compile commands to find #include relationships
// by running clang with -H flag
func (e *Extractor) ExtractIncludes(commands []vcxproj.CompileCommand) {
	work := make(chan vcxproj.CompileCommand, len(commands))
	for _, cmd := range commands {
		work <- cmd
	}
	close(work)

	var wg sync.WaitGroup
	workers := e.Workers
	if workers < 1 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cmd := range work {
				e.extractFileIncludes(cmd)
			}
		}()
	}
	wg.Wait()
}

func (e *Extractor) extractFileIncludes(cmd vcxproj.CompileCommand) {
	args := make([]string, 0, len(cmd.Arguments)+1)
	args = append(args, "-H", "-fsyntax-only")
	if len(cmd.Arguments) > 1 {
		args = append(args, cmd.Arguments[1:]...)
	}

	proc := exec.Command(e.ClangPath, args...)
	proc.Dir = cmd.Directory
	proc.Stdout = io.Discard

	stderr, err := proc.StderrPipe()
	if err != nil {
		return
	}

	if err := proc.Start(); err != nil {
		return
	}

	sourceRel := e.makeRelPath(cmd.File)
	sourceID := "File:" + sourceRel

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// -H output format: "." for depth 1, ".." for depth 2, etc.
		if len(line) == 0 || line[0] != '.' {
			continue
		}
		// Only depth-1 includes (direct includes from this file)
		if len(line) < 2 || line[1] != ' ' {
			continue
		}

		includedPath := strings.TrimSpace(line[2:])
		if !e.isPathInRepo(includedPath) {
			continue
		}

		targetRel := e.makeRelPath(includedPath)
		targetID := "File:" + targetRel

		e.emitEdgeDirect(sourceID, targetID, "INCLUDES")
	}

	proc.Wait()
}

func (e *Extractor) isPathInRepo(path string) bool {
	normalized := strings.ReplaceAll(filepath.Clean(path), "\\", "/")
	repoNorm := strings.ReplaceAll(e.RepoRoot, "\\", "/")
	return strings.HasPrefix(strings.ToLower(normalized), strings.ToLower(repoNorm))
}

func (e *Extractor) makeRelPath(absPath string) string {
	rel, err := filepath.Rel(e.RepoRoot, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}

func isSTLInternal(name string) bool {
	return len(name) > 0 && name[0] == '_'
}

func isSTLInternalID(id string) bool {
	if idx := strings.LastIndex(id, "::"); idx >= 0 {
		name := id[idx+2:]
		if paren := strings.IndexByte(name, '('); paren >= 0 {
			name = name[:paren]
		}
		return isSTLInternal(name)
	}
	if colon := strings.IndexByte(id, ':'); colon >= 0 {
		name := id[colon+1:]
		if paren := strings.IndexByte(name, '('); paren >= 0 {
			name = name[:paren]
		}
		return isSTLInternal(name)
	}
	return isSTLInternal(id)
}

func (e *Extractor) emitEdgeDirect(source, target, edgeType string) {
	key := source + "|" + edgeType + "|" + target
	if _, loaded := e.emittedEdges.LoadOrStore(key, true); loaded {
		return
	}

	out := map[string]string{
		"source": source,
		"target": target,
		"type":   edgeType,
	}

	data, _ := json.Marshal(out)
	data = append(data, '\n')

	e.edgesMu.Lock()
	e.EdgesFile.Write(data)
	e.edgesMu.Unlock()

	e.edgeCount.Add(1)
}
