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
	// Note: These are simplified and might not cover all VB.NET syntax edge cases.
	classRegex := regexp.MustCompile(`(?i)(?:Class|Module)\s+(\w+)`)
	funcRegex := regexp.MustCompile(`(?i)(?:Sub|Function)\s+(\w+)`)
	endFuncRegex := regexp.MustCompile(`(?i)End\s+(?:Sub|Function)`)
	callRegex := regexp.MustCompile(`(\w+)\(`)

	lines := strings.Split(string(content), "\n")
	
	// Create File Node
	// Removed "content" property to reduce memory usage
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

	for i, line := range lines {
		lineNumber := i + 1
		trimmed := strings.TrimSpace(line)
		
		// 1. Check for Class/Module Definition
		if matches := classRegex.FindStringSubmatch(trimmed); matches != nil {
			className := matches[1]
			
			classID := fmt.Sprintf("%s:%s", filePath, className)
			classNode := &graph.Node{
				ID:    classID,
				Label: "Class",
				Properties: map[string]interface{}{
					"name": className,
					"file": filePath,
					"line": lineNumber,
				},
			}
			nodes = append(nodes, classNode)
		}

		// 2. Check for Function/Sub Definition
		if matches := funcRegex.FindStringSubmatch(trimmed); matches != nil {
			funcName := matches[1]
			
			currentFunc = &pendingFunc{
				Name:      funcName,
				StartLine: lineNumber,
				Signature: trimmed,
			}

			// Edge: DEFINED_IN File
			funcID := fmt.Sprintf("%s:%s", filePath, funcName)
			edges = append(edges, &graph.Edge{
				SourceID: funcID,
				TargetID: filePath,
				Type:     "DEFINED_IN",
			})
			continue // Skip checking for calls on the definition line itself
		}

		// 3. Check for End of Function/Sub
		if endFuncRegex.MatchString(trimmed) {
			if currentFunc != nil {
				// Create the Function Node now that we have the end line
				funcID := fmt.Sprintf("%s:%s", filePath, currentFunc.Name)
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
				
				// Avoid self-references or keywords if possible (basic filtering)
				if strings.EqualFold(calledFunc, "If") || strings.EqualFold(calledFunc, "While") || strings.EqualFold(calledFunc, "For") {
					continue
				}

				// Construct IDs
				sourceID := fmt.Sprintf("%s:%s", filePath, currentFunc.Name)
				// Target ID is tricky without semantic analysis. 
				targetID := fmt.Sprintf("%s:%s", filePath, calledFunc)

				edges = append(edges, &graph.Edge{
					SourceID: sourceID,
					TargetID: targetID,
					Type:     "CALLS",
				})
			}
		}
	}

	// Handle case where function is not closed at EOF
	if currentFunc != nil {
		funcID := fmt.Sprintf("%s:%s", filePath, currentFunc.Name)
		funcNode := &graph.Node{
			ID:    funcID,
			Label: "Function",
			Properties: map[string]interface{}{
				"name":      currentFunc.Name,
				"signature": currentFunc.Signature,
				"file":      filePath,
				"line":      currentFunc.StartLine,
				"end_line":  len(lines),
			},
		}
		nodes = append(nodes, funcNode)
	}

	return nodes, edges, nil
}
