package analysis

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/csharp"
	"graphdb/internal/graph"
)

type CSharpParser struct{}

func init() {
	RegisterParser(".cs", &CSharpParser{})
}

func (p *CSharpParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(csharp.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	// 1. Definition Query
	defQueryStr := `
		(class_declaration name: (identifier) @class.name) @class.def
		(interface_declaration name: (identifier) @class.name) @class.def
		(struct_declaration name: (identifier) @class.name) @class.def
		(record_declaration name: (identifier) @class.name) @class.def
		
		(method_declaration name: (identifier) @function.name) @function.def
		(constructor_declaration name: (identifier) @function.name) @function.def
		(local_function_statement name: (identifier) @function.name) @function.def

		(field_declaration) @field.declarator
		(property_declaration name: (identifier) @field.name)

		(using_directive (qualified_name) @using.namespace)
		(using_directive (identifier) @using.namespace)
	`

	qDef, err := sitter.NewQuery([]byte(defQueryStr), csharp.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid definition query: %w", err)
	}
	defer qDef.Close()

	qcDef := sitter.NewQueryCursor()
	defer qcDef.Close()

	qcDef.Exec(qDef, tree.RootNode())

	var nodes []*graph.Node
	var extraEdges []*graph.Edge // Store inheritance edges here
	var usings []string

	// Pre-scan for file-scoped namespace
	fileScopedNamespace := findFileScopedCSharpNamespace(tree.RootNode(), content)

	for {
		m, ok := qcDef.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			captureName := qDef.CaptureNameForId(c.Index)

			if captureName == "using.namespace" {
				usings = append(usings, c.Node.Content(content))
				continue
			}

			// Capture name extraction
			var nodeNames []string
			var nodeType string // "class", "function", "field"

			if strings.HasSuffix(captureName, ".name") {
				nodeNames = append(nodeNames, c.Node.Content(content))
				if strings.HasPrefix(captureName, "class") {
					nodeType = "class"
				} else if strings.HasPrefix(captureName, "function") {
					nodeType = "function"
				} else if strings.HasPrefix(captureName, "field") {
					nodeType = "field"
				}
			} else if captureName == "field.declarator" {
				nodeType = "field"
				// c.Node is field_declaration
				count := c.Node.ChildCount()
				for i := 0; i < int(count); i++ {
					child := c.Node.Child(i)
					if child.Type() == "variable_declarator" {
						name := extractCSharpNameFromDeclarator(child, content)
						if name != "" {
							nodeNames = append(nodeNames, name)
						}
					} else if child.Type() == "variable_declaration" {
						vCount := child.ChildCount()
						for k := 0; k < int(vCount); k++ {
							vChild := child.Child(k)
							if vChild.Type() == "variable_declarator" {
								name := extractCSharpNameFromDeclarator(vChild, content)
								if name != "" {
									nodeNames = append(nodeNames, name)
								}
							}
						}
					}
				}
			}

			if len(nodeNames) == 0 {
				continue
			}

			// Determine Context
			namespace := findEnclosingCSharpNamespace(c.Node, content)
			if namespace == "" {
				namespace = fileScopedNamespace
			}
			
			// If we are defining a class, we want the enclosing class of the definition, not itself.
			searchNode := c.Node
			if nodeType == "class" {
				if p := c.Node.Parent(); p != nil {
					searchNode = p
				}
			}
			enclosingClass := findEnclosingCSharpClass(searchNode, content)

			for _, nodeName := range nodeNames {
				var label string
				var fullID string
				
				// Build Qualified Name
				var parts []string
				if namespace != "" {
					parts = append(parts, namespace)
				}
				if enclosingClass != "" {
					parts = append(parts, enclosingClass)
				}
				parts = append(parts, nodeName)
				
				qualifiedName := strings.Join(parts, ".")
				fullID = fmt.Sprintf("%s:%s", filePath, qualifiedName)

				var properties = map[string]interface{}{
					"name": nodeName,
					"file": filePath,
					"line": c.Node.StartPoint().Row + 1,
				}
				if namespace != "" {
					properties["namespace"] = namespace
				}

				if nodeType == "class" {
					label = "Class"
					
					// Inheritance Logic
					parent := c.Node.Parent() // declaration node
					if parent != nil {
						var baseList *sitter.Node
						baseList = parent.ChildByFieldName("base_list")
						if baseList == nil {
							// fallback search
							count := parent.ChildCount()
							for i := 0; i < int(count); i++ {
								child := parent.Child(i)
								if child.Type() == "base_list" {
									baseList = child
									break
								}
							}
						}

						if baseList != nil {
							count := baseList.NamedChildCount()
							for i := 0; i < int(count); i++ {
								child := baseList.NamedChild(i)
								baseName := child.Content(content)
								// Naive target ID for inheritance
								// Ideally we resolve this too, but for now we guess
								// It might be in the same namespace or imported.
								// We leave it as simple name or qualified if provided.
								targetID := fmt.Sprintf("%s:%s", filePath, baseName) 
								extraEdges = append(extraEdges, &graph.Edge{
									SourceID: fullID,
									TargetID: targetID,
									Type:     "INHERITS",
								})
							}
						}
					}

				} else if nodeType == "function" {
					label = "Function"
				} else if nodeType == "field" {
					label = "Field"
				} else {
					continue
				}

				n := &graph.Node{
					ID:         fullID,
					Label:      label,
					Properties: properties,
				}
				nodes = append(nodes, n)
			}
		}
	}

	// 2. Reference/Call Query
	refQueryStr := `
		(invocation_expression
			function: (identifier) @call.target
		) @call.site

		(invocation_expression
			function: (member_access_expression name: (identifier) @call.target)
		) @call.site

		(object_creation_expression
			type: (identifier) @call.target
		) @call.site

		(object_creation_expression
			type: (generic_name (identifier) @call.target)
		) @call.site
	`

	qRef, err := sitter.NewQuery([]byte(refQueryStr), csharp.GetLanguage())
	if err != nil {
		return nodes, nil, fmt.Errorf("invalid reference query: %w", err)
	}
	defer qRef.Close()

	qcRef := sitter.NewQueryCursor()
	defer qcRef.Close()

	qcRef.Exec(qRef, tree.RootNode())

	var edges []*graph.Edge = extraEdges

	for {
		m, ok := qcRef.NextMatch()
		if !ok {
			break
		}

		var targetName string
		var callNode *sitter.Node

		for _, c := range m.Captures {
			name := qRef.CaptureNameForId(c.Index)
			if name == "call.target" {
				targetName = c.Node.Content(content)
			}
			if name == "call.site" {
				callNode = c.Node
			}
		}

		if targetName != "" && callNode != nil {
			sourceFuncNode := findEnclosingCSharpFunctionNode(callNode)
			if sourceFuncNode != nil {
				// Reconstruct Source ID
				ns := findEnclosingCSharpNamespace(sourceFuncNode, content)
				if ns == "" { ns = fileScopedNamespace }
				
				cls := findEnclosingCSharpClass(sourceFuncNode, content)
				
				funcNameNode := sourceFuncNode.ChildByFieldName("name")
				var funcName string
				if funcNameNode != nil {
					funcName = funcNameNode.Content(content)
				}
				
				if funcName != "" {
					var parts []string
					if ns != "" { parts = append(parts, ns) }
					if cls != "" { parts = append(parts, cls) }
					parts = append(parts, funcName)
					
					sourceID := fmt.Sprintf("%s:%s", filePath, strings.Join(parts, "."))

					candidates := resolveCSharpCandidates(targetName, usings, ns)
					for _, cand := range candidates {
						edges = append(edges, &graph.Edge{
							SourceID: sourceID,
							TargetID: cand, 
							Type:     "CALLS",
						})
					}
				}
			}
		}
	}

	return nodes, edges, nil
}

func extractCSharpNameFromDeclarator(n *sitter.Node, content []byte) string {
	nameChild := n.ChildByFieldName("name")
	if nameChild != nil {
		return nameChild.Content(content)
	}
	count := n.ChildCount()
	for i := 0; i < int(count); i++ {
		if n.Child(i).Type() == "identifier" {
			return n.Child(i).Content(content)
		}
	}
	return ""
}

func resolveCSharpCandidates(name string, usings []string, currentNamespace string) []string {
	if strings.Contains(name, ".") {
		return []string{name}
	}

	var candidates []string
	// Local namespace
	if currentNamespace != "" {
		candidates = append(candidates, fmt.Sprintf("%s.%s", currentNamespace, name))
	} else {
		// Global namespace
		candidates = append(candidates, name)
	}

	for _, u := range usings {
		candidates = append(candidates, fmt.Sprintf("%s.%s", u, name))
	}

	return candidates
}

func findEnclosingCSharpNamespace(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		if curr.Type() == "namespace_declaration" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return ""
}

func findFileScopedCSharpNamespace(root *sitter.Node, content []byte) string {
	count := root.NamedChildCount()
	for i := 0; i < int(count); i++ {
		child := root.NamedChild(i)
		if child.Type() == "file_scoped_namespace_declaration" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
	}
	return ""
}

func findEnclosingCSharpClass(n *sitter.Node, content []byte) string {
	var parts []string
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "class_declaration" || t == "struct_declaration" || t == "interface_declaration" || t == "record_declaration" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				parts = append([]string{nameNode.Content(content)}, parts...) // Prepend
			}
		}
		curr = curr.Parent()
	}
	return strings.Join(parts, ".")
}

func findEnclosingCSharpFunctionNode(n *sitter.Node) *sitter.Node {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "method_declaration" || t == "constructor_declaration" || t == "local_function_statement" {
			return curr
		}
		curr = curr.Parent()
	}
	return nil
}
