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
		
		(method_declaration
			(modifiers (annotation name: (identifier) @annotation.name))
			name: (identifier) @function.name
		) @function.with_annotations
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

	// Map to track annotations per method node
	methodAnnotations := make(map[uintptr][]string)

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
			case "annotation.name":
				attrName := nodeContent
				// Find the method_declaration node this attribute belongs to
				parent := c.Node.Parent()
				for parent != nil && parent.Type() != "method_declaration" {
					parent = parent.Parent()
				}
				if parent != nil {
					id := parent.ID()
					methodAnnotations[id] = append(methodAnnotations[id], attrName)
				}
						case "class.name":
							parentType := c.Node.Parent().Type()
							label := "Class"
							if parentType == "interface_declaration" {
								label = "Interface"
							} else if parentType == "enum_declaration" {
								label = "Enum"
							}
			
							fqn := nodeContent
							if packageName != "" {
								fqn = fmt.Sprintf("%s.%s", packageName, nodeContent)
							}
							id := GenerateNodeID(label, fqn, "")
			
							nodes = append(nodes, &graph.Node{
								ID:    id,
								Label: label,
								Properties: map[string]interface{}{
									"name":     nodeContent,
									"fqn":      fqn,
									"file":     filePath,
									"start_line":     int(c.Node.Parent().StartPoint().Row + 1),
									"end_line": int(c.Node.Parent().EndPoint().Row + 1),
								},
							})

									case "class.extends", "class.implements":
										sourceClass, sourceLabel := findEnclosingClassDetails(c.Node, content)
										if sourceClass != "" {
											sourceFQN := sourceClass
											if packageName != "" {
												sourceFQN = fmt.Sprintf("%s.%s", packageName, sourceClass)
											}
											
											if sourceLabel == "" {
												sourceLabel = "Class"
											}
											sourceID := GenerateNodeID(sourceLabel, sourceFQN, "")
						
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
															parentClass, parentLabel := findEnclosingClassDetails(c.Node, content)
															if parentClass != "" {
																classFQN := parentClass
																if packageName != "" {
																	classFQN = fmt.Sprintf("%s.%s", packageName, parentClass)
																}
																
																if parentLabel == "" {
																	parentLabel = "Class"
																}
																classID := GenerateNodeID(parentLabel, classFQN, "")
											
																methodFQN := fmt.Sprintf("%s.%s", classFQN, nodeContent)
																label := "Function"
																if c.Node.Parent().Type() == "constructor_declaration" {
																	label = "Constructor"
																}
											
																signature := getJavaMethodSignature(c.Node.Parent(), content)
																methodID := GenerateNodeID(label, methodFQN, signature)
											
																properties := map[string]interface{}{
																	"name":     nodeContent,
																	"fqn":      methodFQN,
																	"file":     filePath,
																	"start_line":     int(c.Node.Parent().StartPoint().Row + 1),
																	"end_line": int(c.Node.Parent().EndPoint().Row + 1),
																}

																// Structural Test Detection via Annotations
																if label == "Function" {
																	funcNode := c.Node.Parent()
																	if funcNode != nil {
																		attrs := methodAnnotations[funcNode.ID()]
																		for _, a := range attrs {
																			if a == "Test" {
																				properties["is_test"] = true
																				break
																			}
																		}
																	}
																}

																nodes = append(nodes, &graph.Node{
																	ID:         methodID,
																	Label:      label,
																	Properties: properties,
																})
											
																edges = append(edges, &graph.Edge{
																	SourceID: classID,
																	TargetID: methodID,
																	Type:     "HAS_METHOD",
																})
															}
														}		}

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
					parentClass, parentLabel := findEnclosingClassDetails(m.Captures[0].Node, content)
					if parentClass != "" {
						if classFields[parentClass] == nil {
							classFields[parentClass] = make(map[string]string)
						}
		
						simpleBaseType := strings.Split(fieldType, "<")[0]
						resolvedType := resolveType(simpleBaseType)
						classFields[parentClass][fieldName] = resolvedType
		
						classFQN := parentClass
						if packageName != "" {
							classFQN = fmt.Sprintf("%s.%s", packageName, parentClass)
						}
						
						if parentLabel == "" {
							parentLabel = "Class"
						}
						classID := GenerateNodeID(parentLabel, classFQN, "")
		
						fieldFQN := fmt.Sprintf("%s.%s", classFQN, fieldName)
		
						// Create Field Node
						fieldID := GenerateNodeID("Field", fieldFQN, "")
						nodes = append(nodes, &graph.Node{
							ID:    fieldID,
							Label: "Field",
							Properties: map[string]interface{}{
								"name":     fieldName,
								"fqn":      fieldFQN,
								"type":     resolvedType,
								"file":     filePath,
								"start_line":     int(fieldNode.StartPoint().Row + 1),
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
            p1 := paramNode.Parent() // formal_parameter
            if p1 != nil {
                p2 := p1.Parent() // formal_parameters
                if p2 != nil {
                    p3 := p2.Parent() // constructor_declaration or method_declaration
                    if p3 != nil && p3.Type() == "constructor_declaration" {
                        parentClass, parentLabel := findEnclosingClassDetails(p3, content)
                        if parentClass != "" {
                            classFQN := parentClass
                            if packageName != "" {
                                classFQN = fmt.Sprintf("%s.%s", packageName, parentClass)
                            }
							
							if parentLabel == "" {
								parentLabel = "Class"
							}
							classID := GenerateNodeID(parentLabel, classFQN, "")
                            
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
					sourceFuncNode := findEnclosingJavaFunctionNode(callNode)
					sourceClass, _ := findEnclosingClassDetails(callNode, content)
		
					if sourceFuncNode != nil && sourceClass != "" {
						nameNode := sourceFuncNode.ChildByFieldName("name")
						if nameNode != nil {
							sourceFunc := nameNode.Content(content)
							
							classFQN := sourceClass
							if packageName != "" {
								classFQN = fmt.Sprintf("%s.%s", packageName, sourceClass)
							}
							
							funcFQN := fmt.Sprintf("%s.%s", classFQN, sourceFunc)
							signature := getJavaMethodSignature(sourceFuncNode, content)
							
							label := "Function"
							if sourceFuncNode.Type() == "constructor_declaration" {
								label = "Constructor"
							}
							
							sourceID := GenerateNodeID(label, funcFQN, signature)
		
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
										targetID := fmt.Sprintf("%s.%s", typeName, targetName)
										edges = append(edges, &graph.Edge{
											SourceID: sourceID,
											TargetID: targetID,
											Type:     "CALLS",
										})
									} else {
										resolvedScope := resolveType(scopeName)
										targetID := fmt.Sprintf("%s.%s", resolvedScope, targetName)
										edges = append(edges, &graph.Edge{
											SourceID: sourceID,
											TargetID: targetID,
											Type:     "CALLS",
										})
									}
								} else {
									targetID := fmt.Sprintf("%s.%s", classFQN, targetName)
									edges = append(edges, &graph.Edge{
										SourceID: sourceID,
										TargetID: targetID,
										Type:     "CALLS",
									})
								}
							}
						}
					}
				}	}

	return nodes, edges, nil
}



func getJavaMethodSignature(methodNode *sitter.Node, content []byte) string {
	var params []string
	var paramsNode *sitter.Node

	for i := 0; i < int(methodNode.ChildCount()); i++ {
		child := methodNode.Child(i)
		if child.Type() == "formal_parameters" {
			paramsNode = child
			break
		}
	}

	if paramsNode != nil {
		for i := 0; i < int(paramsNode.ChildCount()); i++ {
			child := paramsNode.Child(i)
			if child.Type() == "formal_parameter" {
				typeNode := child.ChildByFieldName("type")
				if typeNode != nil {
					typ := typeNode.Content(content)
					typ = strings.ReplaceAll(typ, " ", "")
					typ = strings.ReplaceAll(typ, "\n", "")
					typ = strings.ReplaceAll(typ, "\t", "")
					params = append(params, typ)
				}
			} else if child.Type() == "spread_parameter" {
				if child.ChildCount() > 0 {
					typeNode := child.Child(0)
					typ := typeNode.Content(content)
					typ = strings.ReplaceAll(typ, " ", "")
					typ = strings.ReplaceAll(typ, "\n", "")
					typ = strings.ReplaceAll(typ, "\t", "")
					params = append(params, typ+"...")
				}
			}
		}
	}

	return "(" + strings.Join(params, ",") + ")"
}

func findEnclosingClassDetails(n *sitter.Node, content []byte) (string, string) {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "class_declaration" || t == "interface_declaration" || t == "enum_declaration" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				label := "Class"
				if t == "interface_declaration" {
					label = "Interface"
				} else if t == "enum_declaration" {
					label = "Enum"
				}
				return nameNode.Content(content), label
			}
		}
		curr = curr.Parent()
	}
	return "", ""
}

func findEnclosingJavaFunctionNode(n *sitter.Node) *sitter.Node {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "method_declaration" || t == "constructor_declaration" {
			return curr
		}
		curr = curr.Parent()
	}
	return nil
}
