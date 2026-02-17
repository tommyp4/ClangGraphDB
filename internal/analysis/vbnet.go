package analysis

import (
	"fmt"
	"regexp"
	"strings"

	"graphdb/internal/graph"
)

type VBNetParser struct{}

func init() {
	RegisterParser(".vb", &VBNetParser{})
}

func (p *VBNetParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	nodes := []*graph.Node{}
	edges := []*graph.Edge{}

	// Regex patterns
	namespaceRegex := regexp.MustCompile(`(?i)^\s*Namespace\s+([\w\.]+)`)
	endNamespaceRegex := regexp.MustCompile(`(?i)^\s*End\s+Namespace`)
	
	classRegex := regexp.MustCompile(`(?i)^\s*(?:Public|Private|Friend|Protected|Partial)?\s*(?:Class|Module|Structure|Interface)\s+(\w+)`)
	endClassRegex := regexp.MustCompile(`(?i)^\s*End\s+(?:Class|Module|Structure|Interface)`)
	
	funcRegex := regexp.MustCompile(`(?i)^\s*(?:Public|Private|Friend|Protected|Overrides|Shared)?\s*(?:Sub|Function)\s+(\w+)`)
	endFuncRegex := regexp.MustCompile(`(?i)^\s*End\s+(?:Sub|Function)`)
	
	callRegex := regexp.MustCompile(`(\w+)\(`)

	lines := strings.Split(string(content), "\n")
	
	// Create File Node
	fileNode := &graph.Node{
		ID:    filePath,
		Label: "File",
		Properties: map[string]interface{}{
			"name":    filePath, 
			"lang":    "vbnet",
		},
	}
	nodes = append(nodes, fileNode)

	type pendingFunc struct {
		Name      string
		StartLine int
		Signature string
	}
	var currentFunc *pendingFunc
	
	var namespaceStack []string
	var classStack []string

	for i, line := range lines {
		lineNumber := i + 1
		trimmed := strings.TrimSpace(line)
		
		// 0. Namespace Handling
		if matches := namespaceRegex.FindStringSubmatch(trimmed); matches != nil {
			namespaceStack = append(namespaceStack, matches[1])
			continue
		}
		if endNamespaceRegex.MatchString(trimmed) {
			if len(namespaceStack) > 0 {
				namespaceStack = namespaceStack[:len(namespaceStack)-1]
			}
			continue
		}

		// 1. Class/Module Definition
		if matches := classRegex.FindStringSubmatch(trimmed); matches != nil {
			className := matches[1]
			classStack = append(classStack, className)
			
			// Construct FQN
			var parts []string
			parts = append(parts, namespaceStack...)
			parts = append(parts, classStack...)
			fqn := strings.Join(parts, ".")
			
			classNode := &graph.Node{
				ID:    fqn,
				Label: "Class",
				Properties: map[string]interface{}{
					"name": className,
					"file": filePath,
					"line": lineNumber,
				},
			}
			nodes = append(nodes, classNode)
			continue
		}
		
		// End Class
		if endClassRegex.MatchString(trimmed) {
			if len(classStack) > 0 {
				classStack = classStack[:len(classStack)-1]
			}
			continue
		}

		// 2. Check for Function/Sub Definition
		if matches := funcRegex.FindStringSubmatch(trimmed); matches != nil {
			funcName := matches[1]
			
			currentFunc = &pendingFunc{
				Name:      funcName,
				StartLine: lineNumber,
				Signature: trimmed,
			}

			// Construct FQN for Function (Container + Method)
			var parts []string
			parts = append(parts, namespaceStack...)
			parts = append(parts, classStack...)
			// If we are not inside a class (top level module or script), just append funcName
			
			fqnPrefix := strings.Join(parts, ".")
			var funcID string
			if fqnPrefix != "" {
				funcID = fmt.Sprintf("%s.%s", fqnPrefix, funcName)
			} else {
				funcID = funcName
			}

			// Edge: DEFINED_IN File (Implicitly handled by worker, but we can add edges if needed)
			// Actually worker adds DEFINED_IN based on nodes list.
			
			// If inside class, add HAS_METHOD edge
			if len(classStack) > 0 {
				// Parent Class ID
				classID := strings.Join(parts, ".")
				edges = append(edges, &graph.Edge{
					SourceID: classID,
					TargetID: funcID,
					Type:     "HAS_METHOD",
				})
			}

			continue 
		}

		// 3. Check for End of Function/Sub
		if endFuncRegex.MatchString(trimmed) {
			if currentFunc != nil {
				// Construct ID again
				var parts []string
				parts = append(parts, namespaceStack...)
				parts = append(parts, classStack...)
				
				fqnPrefix := strings.Join(parts, ".")
				var funcID string
				if fqnPrefix != "" {
					funcID = fmt.Sprintf("%s.%s", fqnPrefix, currentFunc.Name)
				} else {
					funcID = currentFunc.Name
				}
				
				funcNode := &graph.Node{
					ID:    funcID,
					Label: "Function",
					Properties: map[string]interface{}{
						"name":      currentFunc.Name,
						"signature": currentFunc.Signature,
						"file":      filePath,
						"line":      currentFunc.StartLine,
						"end_line":  lineNumber,
					},
				}
				nodes = append(nodes, funcNode)
				currentFunc = nil
			}
		}

		// 4. Check for Calls (only if inside a function)
		if currentFunc != nil {
			// Find all calls in the line
			callMatches := callRegex.FindAllStringSubmatch(trimmed, -1)
			for _, match := range callMatches {
				calledFunc := match[1]
				
				if strings.EqualFold(calledFunc, "If") || strings.EqualFold(calledFunc, "While") || strings.EqualFold(calledFunc, "For") || strings.EqualFold(calledFunc, "Catch") {
					continue
				}

				// Construct Source ID
				var parts []string
				parts = append(parts, namespaceStack...)
				parts = append(parts, classStack...)
				fqnPrefix := strings.Join(parts, ".")
				sourceID := fmt.Sprintf("%s.%s", fqnPrefix, currentFunc.Name)

				// Target ID is tricky. Use simple name if unknown.
				// Or assume it's in the same class/namespace?
				// For now, use simple name to avoid assuming wrong namespace.
				// OR better: Just use the called function name.
				// If we want FQN target, we need symbol resolution.
				// User wants "Node ID refactoring".
				// For Calls, we just emit the edge.
				targetID := calledFunc 

				edges = append(edges, &graph.Edge{
					SourceID: sourceID,
					TargetID: targetID,
					Type:     "CALLS",
				})
			}
		}
	}

	return nodes, edges, nil
}
