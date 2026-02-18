package analysis

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
	"graphdb/internal/graph"
)

type JavaParser struct{}

func init() {
	RegisterParser(".java", &JavaParser{})
}

func (p *JavaParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	var nodes []*graph.Node
	var edges []*graph.Edge

	// Context state
	packageName := ""
	imports := make(map[string]string)               // Alias -> Full Name
	classFields := make(map[string]map[string]string) // ClassName -> FieldName -> TypeName

	// 1. Definition Query
	defQueryStr := `
		(package_declaration
			(scoped_identifier) @package.name
		)

		(import_declaration
			[(scoped_identifier) (identifier)] @import.name
		)

		(class_declaration 
			name: (identifier) @class.name
			superclass: (superclass (type_identifier) @class.extends)?
			interfaces: (super_interfaces (type_list (type_identifier) @class.implements))?
		)
		(interface_declaration 
			name: (identifier) @class.name
			(extends_interfaces (type_list (type_identifier) @class.extends))?
		)
		(enum_declaration name: (identifier) @class.name)
		(record_declaration name: (identifier) @class.name)
		
		(field_declaration
			type: (_) @field.type
			declarator: (variable_declarator name: (identifier) @field.name)
		)
		
		(method_declaration name: (identifier) @function.name)
		(constructor_declaration name: (identifier) @function.name)

        (formal_parameter 
            type: (_) @param.type
            name: (identifier) @param.name
        )
	`

	qDef, err := sitter.NewQuery([]byte(defQueryStr), java.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid definition query: %w", err)
	}
	defer qDef.Close()

	qcDef := sitter.NewQueryCursor()
	defer qcDef.Close()

	qcDef.Exec(qDef, tree.RootNode())

	// Helper to resolve type name using imports
	resolveType := func(typeName string) string {
		if fq, ok := imports[typeName]; ok {
			return fq
		}
		// Default to package prefix if not imported (heuristic)
        // Assume same package for simple names (skip basic types check for now, can be improved)
        if !strings.Contains(typeName, ".") && packageName != "" {
             // Quick check for common java.lang types to avoid bad prefixes
             switch typeName {
             case "String", "Integer", "Boolean", "Object", "System", "Exception", "List", "Map", "Set":
                 return typeName
             }
             return fmt.Sprintf("%s.%s", packageName, typeName)
        }
		return typeName
	}
    
    // Helper to extract types from generics
    extractTypes := func(typeStr string) []string {
        f := func(c rune) bool {
            return c == '<' || c == '>' || c == ',' || c == ' '
        }
        parts := strings.FieldsFunc(typeStr, f)
        var result []string
        for _, s := range parts {
            if s != "" {
                result = append(result, s)
            }
        }
        return result
    }

	for {
		m, ok := qcDef.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			captureName := qDef.CaptureNameForId(c.Index)
			nodeContent := c.Node.Content(content)

			switch captureName {
			case "package.name":
				packageName = nodeContent
			case "import.name":
				parts := strings.Split(nodeContent, ".")
				if len(parts) > 0 {
					alias := parts[len(parts)-1]
					imports[alias] = nodeContent
				}
			case "class.name":
				parentType := c.Node.Parent().Type()
				label := "Class"
				if parentType == "interface_declaration" {
					label = "Interface"
				} else if parentType == "enum_declaration" {
					label = "Enum"
				}

				id := nodeContent
				if packageName != "" {
					id = fmt.Sprintf("%s.%s", packageName, nodeContent)
				}
				
				nodes = append(nodes, &graph.Node{
					ID:    id,
					Label: label,
					Properties: map[string]interface{}{
						"name": nodeContent,
						"file": filePath,
						"line": int(c.Node.StartPoint().Row + 1),
						"end_line": int(c.Node.EndPoint().Row + 1),
					},
				})

			case "class.extends", "class.implements":
				sourceClass := findEnclosingClass(c.Node, content)
				if sourceClass != "" {
					sourceID := sourceClass
					if packageName != "" {
						sourceID = fmt.Sprintf("%s.%s", packageName, sourceClass)
					}
					
					targetType := resolveType(nodeContent)
					edgeType := "EXTENDS"
					if captureName == "class.implements" {
						edgeType = "IMPLEMENTS"
					}
					
					edges = append(edges, &graph.Edge{
						SourceID: sourceID,
						TargetID: targetType,
						Type:     edgeType,
					})
				}
			case "function.name":
				parentClass := findEnclosingClass(c.Node, content)
				if parentClass != "" {
					classID := parentClass
					if packageName != "" {
						classID = fmt.Sprintf("%s.%s", packageName, parentClass)
					}
					
					methodID := fmt.Sprintf("%s:%s", classID, nodeContent)
					label := "Function"
                    if c.Node.Parent().Type() == "constructor_declaration" {
                        label = "Constructor"
                    }
					
					nodes = append(nodes, &graph.Node{
						ID:    methodID,
						Label: label,
						Properties: map[string]interface{}{
							"name": nodeContent,
							"file": filePath,
							"line": int(c.Node.StartPoint().Row + 1),
							"end_line": int(c.Node.EndPoint().Row + 1),
						},
					})
					
					edges = append(edges, &graph.Edge{
						SourceID: classID,
						TargetID: methodID,
						Type:     "HAS_METHOD",
					})
				}
			}
		}

		// Handle Fields and Params
		var fieldName, fieldType string
		var fieldNode *sitter.Node
		var paramName, paramType string
        var paramNode *sitter.Node
		
		for _, c := range m.Captures {
			name := qDef.CaptureNameForId(c.Index)
			if name == "field.name" {
				fieldName = c.Node.Content(content)
				fieldNode = c.Node
			}
			if name == "field.type" {
				fieldType = c.Node.Content(content)
			}
			if name == "param.name" {
				paramName = c.Node.Content(content)
                paramNode = c.Node
			}
			if name == "param.type" {
				paramType = c.Node.Content(content)
			}
		}

		// Processing Field
		if fieldName != "" && fieldType != "" {
			parentClass := findEnclosingClass(m.Captures[0].Node, content)
			if parentClass != "" {
				if classFields[parentClass] == nil {
					classFields[parentClass] = make(map[string]string)
				}
				
                simpleBaseType := strings.Split(fieldType, "<")[0]
				resolvedType := resolveType(simpleBaseType)
				classFields[parentClass][fieldName] = resolvedType
				
				classID := parentClass
				if packageName != "" {
					classID = fmt.Sprintf("%s.%s", packageName, parentClass)
				}
				
				// Create Field Node
				fieldID := fmt.Sprintf("%s:%s", classID, fieldName)
				nodes = append(nodes, &graph.Node{
					ID: fieldID,
					Label: "Field",
					Properties: map[string]interface{}{
						"name": fieldName,
						"type": resolvedType,
						"file": filePath,
						"line": int(fieldNode.StartPoint().Row + 1),
						"end_line": int(fieldNode.EndPoint().Row + 1),
					},
				})
				
				edges = append(edges, &graph.Edge{
					SourceID: classID,
					TargetID: fieldID,
					Type:     "DEFINES",
				})

                // Create DEPENDS_ON edges
                types := extractTypes(fieldType)
                for _, tName := range types {
                    resolved := resolveType(tName)
                    edges = append(edges, &graph.Edge{
                        SourceID: classID,
                        TargetID: resolved,
                        Type:     "DEPENDS_ON",
                    })
                }
			}
		}
		
		// Processing Constructor Parameter
		if paramName != "" && paramType != "" {
            // Check if it belongs to a constructor
            // param -> formal_parameters -> constructor_declaration
            // parent of param is formal_parameter? No, paramNode is the identifier.
            // paramNode parent is formal_parameter.
            // formal_parameter parent is formal_parameters.
            // formal_parameters parent is constructor_declaration.
            
            // Wait, paramNode is 'identifier'.
            // c.Node for param.name is identifier.
            // identifier -> formal_parameter -> formal_parameters -> constructor_declaration
            
            p1 := paramNode.Parent() // formal_parameter
            if p1 != nil {
                p2 := p1.Parent() // formal_parameters (or method if single? no java always has parens)
                if p2 != nil {
                    p3 := p2.Parent() // constructor_declaration or method_declaration
                    if p3 != nil && p3.Type() == "constructor_declaration" {
                        parentClass := findEnclosingClass(p3, content)
                        if parentClass != "" {
                            classID := parentClass
                            if packageName != "" {
                                classID = fmt.Sprintf("%s.%s", packageName, parentClass)
                            }
                            
                            types := extractTypes(paramType)
                            for _, tName := range types {
                                resolved := resolveType(tName)
                                edges = append(edges, &graph.Edge{
                                    SourceID: classID,
                                    TargetID: resolved,
                                    Type:     "DEPENDS_ON",
                                })
                            }
                        }
                    }
                }
            }
        }
	}

	// 2. Reference/Call Query (Unchanged mostly)
	refQueryStr := `
		(method_invocation
			object: (identifier)? @call.scope
			name: (identifier) @call.target
		) @call.site

		(object_creation_expression
			type: (type_identifier) @call.target
		) @call.site
	`

	qRef, err := sitter.NewQuery([]byte(refQueryStr), java.GetLanguage())
	if err != nil {
		return nodes, nil, fmt.Errorf("invalid reference query: %w", err)
	}
	defer qRef.Close()

	qcRef := sitter.NewQueryCursor()
	defer qcRef.Close()

	qcRef.Exec(qRef, tree.RootNode())

	for {
		m, ok := qcRef.NextMatch()
		if !ok {
			break
		}

		var targetName string
		var scopeName string
		var callNode *sitter.Node

		for _, c := range m.Captures {
			name := qRef.CaptureNameForId(c.Index)
			if name == "call.target" {
				targetName = c.Node.Content(content)
			}
			if name == "call.scope" {
				scopeName = c.Node.Content(content)
			}
			if name == "call.site" {
				callNode = c.Node
			}
		}

		if callNode != nil {
			sourceFunc := findEnclosingJavaFunction(callNode, content)
			sourceClass := findEnclosingClass(callNode, content)
			
			if sourceFunc != "" && sourceClass != "" {
				classID := sourceClass
				if packageName != "" {
					classID = fmt.Sprintf("%s.%s", packageName, sourceClass)
				}
				sourceID := fmt.Sprintf("%s:%s", classID, sourceFunc)
				
				// 1. Constructor Calls
				if callNode.Type() == "object_creation_expression" && targetName != "" {
					resolvedType := resolveType(targetName)
					edges = append(edges, &graph.Edge{
						SourceID: sourceID,
						TargetID: resolvedType,
						Type:     "CALLS",
					})
				}

				// 2. Method Calls
				if callNode.Type() == "method_invocation" {
					if scopeName != "" {
						if typeName, ok := classFields[sourceClass][scopeName]; ok {
							targetID := fmt.Sprintf("%s:%s", typeName, targetName)
							edges = append(edges, &graph.Edge{
								SourceID: sourceID,
								TargetID: targetID,
								Type:     "CALLS",
							})
						} else {
							resolvedScope := resolveType(scopeName)
							targetID := fmt.Sprintf("%s:%s", resolvedScope, targetName)
							edges = append(edges, &graph.Edge{
								SourceID: sourceID,
								TargetID: targetID,
								Type:     "CALLS",
							})
						}
					} else {
						targetID := fmt.Sprintf("%s:%s", classID, targetName)
						edges = append(edges, &graph.Edge{
							SourceID: sourceID,
							TargetID: targetID,
							Type:     "CALLS",
						})
					}
				}
			}
		}
	}

	return nodes, edges, nil
}

func findEnclosingJavaFunction(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "method_declaration" || t == "constructor_declaration" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return ""
}

func findEnclosingClass(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "class_declaration" || t == "interface_declaration" || t == "enum_declaration" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return ""
}
