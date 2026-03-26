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

	localDefs := make(map[string]string)
	includes := []string{}

	// Helper to extract types
    extractTypes := func(typeStr string) []string {
        f := func(c rune) bool {
            return c == '<' || c == '>' || c == ',' || c == ' '
        }
        parts := strings.FieldsFunc(typeStr, f)
        var result []string
        for _, s := range parts {
            s = strings.TrimSpace(s)
            if s != "" && s != "std" && s != "vector" && s != "shared_ptr" && s != "unique_ptr" {
                result = append(result, s)
            }
        }
        return result
    }

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

		(field_declaration) @field.decl
		
		(parameter_declaration) @param.decl

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

            // Handle Field Declarations
            if name == "field.decl" {
                // Extract Type
                typeNode := c.Node.ChildByFieldName("type")
                if typeNode != nil {
                    typeStr := typeNode.Content(content)
                    // Extract Name (Declarator)
                    declNames := extractCppDeclarators(c.Node, content)
                    
                    if len(declNames) > 0 {
                        // For fields, we assume they belong to the enclosing class
                        searchNode := c.Node
                        if p := c.Node.Parent(); p != nil {
                            searchNode = p
                        }
                        enclosingClass := findEnclosingCppClass(searchNode, content)
                        
                        if enclosingClass != "" {
                            // Create Field Nodes and Dependencies
                            for _, fieldName := range declNames {
                                // Create Field Node
                                // FQN: Class::Field
                                fieldFqn := fmt.Sprintf("%s:%s::%s", filePath, enclosingClass, fieldName)
                                fieldID := GenerateNodeID("Field", fieldFqn, "")
                                nodes = append(nodes, &graph.Node{
                                    ID:    fieldID,
                                    Label: "Field",
                                    Properties: map[string]interface{}{
                                        "name": fieldName,
                                        "fqn":  fieldFqn,
                                        "type": typeStr,
                                        "file": filePath,
                                        "start_line": int(c.Node.StartPoint().Row + 1),
                                    },
                                })
                                
                                sourceFqn := fmt.Sprintf("%s:%s", filePath, enclosingClass)
                                sourceID := GenerateNodeID("Class", sourceFqn, "")
                                edges = append(edges, &graph.Edge{
                                    SourceID: sourceID,
                                    TargetID: fieldID,
                                    Type:     "DEFINES",
                                })
                                
                                // Create DEPENDS_ON edges
                                types := extractTypes(typeStr)
                                for _, tName := range types {
                                    // Resolve Type
                                    targetID := localDefs[tName]
                                    if targetID == "" {
                                        targetID = resolveCppInclude(tName, includes, filePath)
                                    }
                                    
                                    edges = append(edges, &graph.Edge{
                                        SourceID: sourceID,
                                        TargetID: targetID,
                                        Type:     "DEPENDS_ON",
                                    })
                                }
                            }
                        }
                    }
                }
                continue
            }
            
            // Handle Parameter Declarations
            if name == "param.decl" {
                typeNode := c.Node.ChildByFieldName("type")
                if typeNode != nil {
                    typeStr := typeNode.Content(content)
                    
                    // Check if inside Constructor
                    funcDef := findEnclosingCppFunctionNode(c.Node)
                    if funcDef != nil {
                        funcName := extractCppFunctionName(funcDef, content)
                        cls := findEnclosingCppClass(funcDef, content)
                        
                        // Constructor check: funcName == className (last part)
                        clsName := cls
                        if idx := strings.LastIndex(cls, "::"); idx != -1 {
                            clsName = cls[idx+2:]
                        }
                        
                        // Also handle qualified func name "Class::Class"
                        simpleFuncName := funcName
                        if idx := strings.LastIndex(funcName, "::"); idx != -1 {
                            simpleFuncName = funcName[idx+2:]
                        }
                        
                        if clsName != "" && simpleFuncName == clsName {
                            // Constructor found
                             sourceFqn := fmt.Sprintf("%s:%s", filePath, cls)
                             sourceID := GenerateNodeID("Class", sourceFqn, "") // Link to Class
                             
                             types := extractTypes(typeStr)
                             for _, tName := range types {
                                targetID := localDefs[tName]
                                if targetID == "" {
                                    targetID = resolveCppInclude(tName, includes, filePath)
                                }
                                edges = append(edges, &graph.Edge{
                                    SourceID: sourceID,
                                    TargetID: targetID,
                                    Type:     "DEPENDS_ON",
                                })
                             }
                        }
                    }
                }
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
                        fqn := fmt.Sprintf("%s:%s", filePath, qualifiedName)
                        signature := ""
                        if label == "Function" {
                                signature = extractCppSignature(c.Node, content)
                        }
                        nodeID := GenerateNodeID(label, fqn, signature)
                        // Use declaration node for line span (function.name is nested 2 levels deep)
                        declNode := c.Node
                        if label == "Function" {
                                if p := c.Node.Parent(); p != nil {
                                        if pp := p.Parent(); pp != nil {
                                                declNode = pp
                                        }
                                }
                        } else if parent := c.Node.Parent(); parent != nil {
                                declNode = parent
                        }
                        props := map[string]interface{}{
                                "name": nodeContent,
                                "fqn":  fqn,
                                "file": filePath,
                                "start_line": declNode.StartPoint().Row + 1,
                                "end_line": declNode.EndPoint().Row + 1,
                        }
                        if namespace != "" {
                                props["namespace"] = namespace
                        }
                        nodes = append(nodes, &graph.Node{
                                ID:    nodeID,
                                Label: label,
                                Properties: props,
                        })
			
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
			ns := findEnclosingCppNamespace(srcNode, content)
			searchNode := srcNode
			if p := srcNode.Parent(); p != nil {
				searchNode = p
			}
			cls := findEnclosingCppClass(searchNode, content)
			
			var parts []string
			if ns != "" { parts = append(parts, ns) }
			if cls != "" { parts = append(parts, cls) }
			parts = append(parts, src)
			
                        sourceFqn := fmt.Sprintf("%s:%s", filePath, strings.Join(parts, "::"))
                        sourceID := GenerateNodeID("Class", sourceFqn, "")

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
		
		if targetName != "" && siteNode != nil {
			sourceFuncNode := findEnclosingCppFunctionNode(siteNode)
			if sourceFuncNode != nil {
				ns := findEnclosingCppNamespace(sourceFuncNode, content)
				cls := findEnclosingCppClass(sourceFuncNode, content)
				funcName := extractCppFunctionName(sourceFuncNode, content)
				
				if funcName != "" {
					var parts []string
					if ns != "" { parts = append(parts, ns) }
					if cls != "" { parts = append(parts, cls) }
					parts = append(parts, funcName)
					
					sourceFqn := fmt.Sprintf("%s:%s", filePath, strings.Join(parts, "::"))
					// FIX: Use signature and "Function" label instead of "Class"
					signature := extractCppSignature(sourceFuncNode, content)
					sourceID := GenerateNodeID("Function", sourceFqn, signature)

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

func extractCppDeclarators(n *sitter.Node, content []byte) []string {
    var names []string
    count := n.NamedChildCount()
    for i := 0; i < int(count); i++ {
        child := n.NamedChild(i)
        // Check if child is a declarator (or pointer_declarator, etc)
        if n.FieldNameForChild(i) == "declarator" {
            // Traverse down to find identifier
            names = append(names, extractCppIdentifier(child, content))
        }
    }
    return names
}

func extractCppIdentifier(n *sitter.Node, content []byte) string {
    // Recursive decent to find identifier
    if n.Type() == "identifier" || n.Type() == "field_identifier" {
        return n.Content(content)
    }
    count := n.NamedChildCount()
    for i := 0; i < int(count); i++ {
        res := extractCppIdentifier(n.NamedChild(i), content)
        if res != "" {
            return res
        }
    }
    return ""
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
            // Found matching include file.
            // Return symbol name without path, assuming uniqueness.
            // (e.g. "MyClass")
			return symbol
		}
	}
	// Fallback to the symbol itself to allow name-based resolution in the graph.
	return symbol
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
        return extractCppIdentifier(decl, content)
	}
	return ""
}


func extractCppSignature(n *sitter.Node, content []byte) string {
	curr := n
	for curr != nil && curr.Type() != "function_definition" && curr.Type() != "declaration" {
		curr = curr.Parent()
	}
	if curr == nil {
		curr = n
	}

	var paramList *sitter.Node
	var findParamList func(node *sitter.Node) *sitter.Node
	findParamList = func(node *sitter.Node) *sitter.Node {
		if node == nil {
			return nil
		}
		if node.Type() == "parameter_list" {
			return node
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if res := findParamList(node.NamedChild(i)); res != nil {
				return res
			}
		}
		return nil
	}

	paramList = findParamList(curr)
	if paramList == nil {
		return "()"
	}

	sig := paramList.Content(content)
	sig = strings.ReplaceAll(sig, " ", "")
	sig = strings.ReplaceAll(sig, "\n", "")
	sig = strings.ReplaceAll(sig, "\t", "")
	return sig
}
