package analysis

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"
	"graphdb/internal/graph"
)

type CppParser struct{}

func init() {
	p := &CppParser{}
	RegisterParser(".c", p)
	RegisterParser(".h", p)
	RegisterParser(".cpp", p)
	RegisterParser(".hpp", p)
	RegisterParser(".cc", p)
}

func (p *CppParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(cpp.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	var nodes []*graph.Node
	var edges []*graph.Edge

	// Local definitions map: Name -> ID
	// Note: With scoped IDs, this map needs to handle scope?
	// For now, we might just map strict names or last-segment names?
	// If we have strict IDs, we might need a better resolution strategy for edges.
	localDefs := make(map[string]string)
	includes := []string{}

	// 1. Structure Query
	structureQueryStr := `
		(function_definition
			declarator: (function_declarator
				declarator: (identifier) @function.name
			)
		)
		(function_definition
			declarator: (function_declarator
				declarator: (field_identifier) @function.name
			)
		)
		(function_definition
			declarator: (function_declarator
				declarator: (qualified_identifier) @function.name
			)
		)

		(translation_unit
			(declaration
				declarator: (init_declarator
					declarator: (identifier) @global.name
				)
			)
		)
		(translation_unit
			(declaration
				declarator: (identifier) @global.name
			)
		)

		(field_declaration
			declarator: (field_identifier) @field.name
		)

		(class_specifier
			name: (type_identifier) @class.name
		)
		(struct_specifier
			name: (type_identifier) @class.name
		)

		(preproc_include
			path: (string_literal) @include.path
		)
		(preproc_include
			path: (system_lib_string) @include.system
		)
	`
	qStruct, err := sitter.NewQuery([]byte(structureQueryStr), cpp.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid structure query: %w", err)
	}
	defer qStruct.Close()

	qcStruct := sitter.NewQueryCursor()
	defer qcStruct.Close()
	qcStruct.Exec(qStruct, tree.RootNode())

	for {
		m, ok := qcStruct.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			name := qStruct.CaptureNameForId(c.Index)
			nodeContent := c.Node.Content(content)
			
			if name == "include.path" || name == "include.system" {
				incPath := strings.Trim(nodeContent, "\"<>")
				includes = append(includes, incPath)
				continue
			}

			var label string
			var nodeType string
			if name == "function.name" {
				label = "Function"
				nodeType = "function"
			} else if name == "global.name" {
				label = "Global"
				nodeType = "global"
			} else if name == "field.name" {
				label = "Field"
				nodeType = "field"
			} else if name == "class.name" {
				label = "Class"
				nodeType = "class"
			} else {
				continue
			}

			// Determine Context
			namespace := findEnclosingCppNamespace(c.Node, content)
			
			searchNode := c.Node
			if nodeType == "class" {
				if p := c.Node.Parent(); p != nil {
					searchNode = p
				}
			}
			enclosingClass := findEnclosingCppClass(searchNode, content)

			// Build Qualified Name
			var parts []string
			if namespace != "" {
				parts = append(parts, namespace)
			}
			if enclosingClass != "" {
				parts = append(parts, enclosingClass)
			}
			parts = append(parts, nodeContent)
			
			qualifiedName := strings.Join(parts, "::")
			nodeID := fmt.Sprintf("%s:%s", filePath, qualifiedName)

			nodes = append(nodes, &graph.Node{
				ID:    nodeID,
				Label: label,
				Properties: map[string]interface{}{
					"name": nodeContent,
					"file": filePath,
					"line": c.Node.StartPoint().Row + 1,
					"end_line": c.Node.EndPoint().Row + 1,
					"namespace": namespace,
				},
			})
			
			// Store mapping for resolution (nodeContent -> ID)
			// Warning: Collisions in key (nodeContent) might happen if multiple items have same name in different scopes.
			// This `localDefs` is used for simplistic resolution.
			// We might want to store more context or rely on qualified names in usage.
			localDefs[nodeContent] = nodeID
		}
	}

	// 2. Inheritance Query
	inheritQueryStr := `
		(class_specifier
			name: (type_identifier) @src
			(base_class_clause
				(type_identifier) @dst
			)
		)
		(class_specifier
			name: (type_identifier) @src
			(base_class_clause
				(_ (type_identifier) @dst)
			)
		)
		(struct_specifier
			name: (type_identifier) @src
			(base_class_clause
				(type_identifier) @dst
			)
		)
	`
	qInherit, err := sitter.NewQuery([]byte(inheritQueryStr), cpp.GetLanguage())
	if err != nil {
		return nodes, edges, fmt.Errorf("invalid inheritance query: %w", err)
	}
	defer qInherit.Close()

	qcInherit := sitter.NewQueryCursor()
	defer qcInherit.Close()
	qcInherit.Exec(qInherit, tree.RootNode())

	for {
		m, ok := qcInherit.NextMatch()
		if !ok {
			break
		}
		var src, dst string
		var srcNode *sitter.Node
		
		for _, c := range m.Captures {
			name := qInherit.CaptureNameForId(c.Index)
			if name == "src" {
				src = c.Node.Content(content)
				srcNode = c.Node
			} else if name == "dst" {
				dst = c.Node.Content(content)
			}
		}
		if src != "" && dst != "" {
			// Construct Source ID properly
			ns := findEnclosingCppNamespace(srcNode, content)
			// Class is srcNode, so enclosing class of DEFINITION is parent of parent...
			// Wait, srcNode is 'name' identifier. Parent is class_specifier.
			// findEnclosingCppClass(class_specifier) -> returns outer class.
			// Logic matches above.
			
			searchNode := srcNode
			if p := srcNode.Parent(); p != nil {
				searchNode = p
			}
			cls := findEnclosingCppClass(searchNode, content)
			
			var parts []string
			if ns != "" { parts = append(parts, ns) }
			if cls != "" { parts = append(parts, cls) }
			parts = append(parts, src)
			
			sourceID := fmt.Sprintf("%s:%s", filePath, strings.Join(parts, "::"))

			targetID := localDefs[dst]
			if targetID == "" {
				targetID = resolveCppInclude(dst, includes, filePath)
			}

			edges = append(edges, &graph.Edge{
				SourceID: sourceID,
				TargetID: targetID,
				Type:     "INHERITS",
			})
		}
	}

	// 3. Usage/Reference Query
	usageQueryStr := `
		(call_expression
			function: (identifier) @call.target
		) @call.site

		(call_expression
			function: (field_expression field: (field_identifier) @call.target)
		) @call.site
		
		(call_expression
			function: (qualified_identifier) @call.target
		) @call.site

		(assignment_expression
			left: (identifier) @usage.write
		) @usage.site

		(assignment_expression
			right: (identifier) @usage.read
		) @usage.site
		
		(update_expression
			argument: (identifier) @usage.write
		) @usage.site
	`
	qUsage, err := sitter.NewQuery([]byte(usageQueryStr), cpp.GetLanguage())
	if err != nil {
		return nodes, edges, fmt.Errorf("invalid usage query: %w", err)
	}
	defer qUsage.Close()

	qcUsage := sitter.NewQueryCursor()
	defer qcUsage.Close()
	qcUsage.Exec(qUsage, tree.RootNode())

	for {
		m, ok := qcUsage.NextMatch()
		if !ok {
			break
		}
		
		var targetName string
		var siteNode *sitter.Node
		var edgeType string = "USES"

		for _, c := range m.Captures {
			name := qUsage.CaptureNameForId(c.Index)
			if name == "call.target" {
				targetName = c.Node.Content(content)
				edgeType = "CALLS"
			} else if name == "usage.read" || name == "usage.write" {
				targetName = c.Node.Content(content)
				edgeType = "USES"
			} else if name == "call.site" || name == "usage.site" {
				siteNode = c.Node
			}
		}
		
		// Debug logging
		// fmt.Printf("Usage Capture: %s, Target: %s\n", edgeType, targetName)

		if targetName != "" && siteNode != nil {
			sourceFuncNode := findEnclosingCppFunctionNode(siteNode)
			if sourceFuncNode != nil {
				// Reconstruct Source ID
				ns := findEnclosingCppNamespace(sourceFuncNode, content)
				cls := findEnclosingCppClass(sourceFuncNode, content)
				
				// Extract function name
				// This is tricky because findEnclosingCppFunctionNode returns the definition node
				// We need to extract the name from it.
				funcName := extractCppFunctionName(sourceFuncNode, content)
				
				if funcName != "" {
					var parts []string
					if ns != "" { parts = append(parts, ns) }
					if cls != "" { parts = append(parts, cls) }
					parts = append(parts, funcName)
					
					sourceID := fmt.Sprintf("%s:%s", filePath, strings.Join(parts, "::"))

					targetID := localDefs[targetName]
					if targetID == "" {
						targetID = resolveCppInclude(targetName, includes, filePath)
					}

					edges = append(edges, &graph.Edge{
						SourceID: sourceID,
						TargetID: targetID,
						Type:     edgeType,
					})
				}
			}
		}
	}

	return nodes, edges, nil
}

func resolveCppInclude(symbol string, includes []string, currentFile string) string {
	symbolBase := symbol
	if idx := strings.Index(symbol, "::"); idx != -1 {
		symbolBase = symbol[:idx]
	}

	for _, inc := range includes {
		base := filepath.Base(inc)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		
		if strings.EqualFold(name, symbolBase) {
			dir := filepath.Dir(currentFile)
			resolvedPath := filepath.Join(dir, inc) 
			return fmt.Sprintf("%s:%s", resolvedPath, symbol)
		}
	}
	return fmt.Sprintf("UNKNOWN:%s", symbol)
}

func findEnclosingCppNamespace(n *sitter.Node, content []byte) string {
	var parts []string
	curr := n.Parent()
	for curr != nil {
		if curr.Type() == "namespace_definition" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				parts = append([]string{nameNode.Content(content)}, parts...)
			}
		}
		curr = curr.Parent()
	}
	return strings.Join(parts, "::")
}

func findEnclosingCppClass(n *sitter.Node, content []byte) string {
	var parts []string
	curr := n.Parent()
	for curr != nil {
		if curr.Type() == "class_specifier" || curr.Type() == "struct_specifier" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				parts = append([]string{nameNode.Content(content)}, parts...)
			}
		}
		curr = curr.Parent()
	}
	return strings.Join(parts, "::")
}

func findEnclosingCppFunctionNode(n *sitter.Node) *sitter.Node {
	curr := n.Parent()
	for curr != nil {
		if curr.Type() == "function_definition" {
			return curr
		}
		curr = curr.Parent()
	}
	return nil
}

func extractCppFunctionName(n *sitter.Node, content []byte) string {
	decl := n.ChildByFieldName("declarator")
	if decl != nil {
		innerDecl := decl.ChildByFieldName("declarator")
		if innerDecl != nil {
			if innerDecl.Type() == "identifier" || innerDecl.Type() == "field_identifier" {
				return innerDecl.Content(content)
			}
			if innerDecl.Type() == "qualified_identifier" {
				return innerDecl.Content(content)
			}
		}
	}
	return ""
}
