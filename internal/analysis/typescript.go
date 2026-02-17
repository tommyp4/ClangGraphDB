package analysis

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"graphdb/internal/graph"
)

type TypeScriptParser struct{}

func init() {
	RegisterParser(".ts", &TypeScriptParser{})
}

func (p *TypeScriptParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	var nodes []*graph.Node
	var edges []*graph.Edge
	
	imports := make(map[string]string)

	extractTypes := func(typeStr string) []string {
        typeStr = strings.TrimPrefix(strings.TrimSpace(typeStr), ":")
        f := func(c rune) bool {
            return c == '<' || c == '>' || c == ',' || c == ' ' || c == '|' || c == '&'
        }
        parts := strings.FieldsFunc(typeStr, f)
        var result []string
        for _, s := range parts {
            s = strings.TrimSpace(s)
            if s != "" && s != "Array" && s != "Promise" {
                result = append(result, s)
            }
        }
        return result
    }

	// 1. Import Query
	importQueryStr := `
		(import_statement
			(import_clause 
				(named_imports 
					(import_specifier) @import.specifier
				)
			)
			(string) @import.source
		)
		(import_statement
			(import_clause 
				(identifier) @import.default
			)
			(string) @import.source
		)
		(import_statement
			(import_clause 
				(namespace_import (identifier) @import.namespace)
			)
			(string) @import.source
		)
	`
	qImport, err := sitter.NewQuery([]byte(importQueryStr), typescript.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid import query: %w", err)
	}
	defer qImport.Close()
	
	qcImport := sitter.NewQueryCursor()
	defer qcImport.Close()
	qcImport.Exec(qImport, tree.RootNode())
	
	for {
		m, ok := qcImport.NextMatch()
		if !ok {
			break
		}
		
		var sourcePath string
		var defaultNode, specifierNode, namespaceNode *sitter.Node
		
		for _, c := range m.Captures {
			name := qImport.CaptureNameForId(c.Index)
			if name == "import.source" {
				sourcePath = c.Node.Content(content)
				sourcePath = strings.Trim(sourcePath, "\"'`")
			} else if name == "import.default" {
				defaultNode = c.Node
			} else if name == "import.specifier" {
				specifierNode = c.Node
			} else if name == "import.namespace" {
				namespaceNode = c.Node
			}
		}
		
		if sourcePath != "" {
			resolvedPath := resolveTSPath(filePath, sourcePath)
			
			if defaultNode != nil {
				localName := defaultNode.Content(content)
				imports[localName] = fmt.Sprintf("%s:default", resolvedPath)
			}
			
			if namespaceNode != nil {
				localName := namespaceNode.Content(content)
				imports[localName] = resolvedPath
			}
			
			if specifierNode != nil {
				var localName, remoteName string
				nameNode := specifierNode.ChildByFieldName("name")
				aliasNode := specifierNode.ChildByFieldName("alias")
				
				if nameNode != nil {
					remoteName = nameNode.Content(content)
				}
				if aliasNode != nil {
					localName = aliasNode.Content(content)
				} else {
					localName = remoteName
				}
				
				if localName != "" && remoteName != "" {
					imports[localName] = fmt.Sprintf("%s:%s", resolvedPath, remoteName)
				}
			}
		}
	}

    resolveTargetID := func(symbol string, currentFile string) string {
        if resolved, ok := imports[symbol]; ok {
            return resolved
        }
        return fmt.Sprintf("%s:%s", currentFile, symbol)
    }

	// 2. Definition & Field Query & DI
	defQueryStr := `
		(function_declaration name: (identifier) @function.name) @function.def
		(generator_function_declaration name: (identifier) @function.name) @function.def
		(method_definition name: (property_identifier) @method.name) @method.def
		(class_declaration name: (type_identifier) @class.name) @class.def
		(interface_declaration name: (type_identifier) @class.name) @class.def
		
        (public_field_definition 
            name: (property_identifier) @field.name
            type: (type_annotation)? @field.type
        ) @field.def

		(variable_declarator 
			name: (identifier) @function.name 
			value: [(arrow_function) (function_expression)]
		) @function.def

        (required_parameter
            pattern: (identifier) @param.name
            type: (type_annotation)? @param.type
        )
	`
	qDef, err := sitter.NewQuery([]byte(defQueryStr), typescript.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid definition query: %w", err)
	}
	defer qDef.Close()

	qcDef := sitter.NewQueryCursor()
	defer qcDef.Close()
	qcDef.Exec(qDef, tree.RootNode())
	
	for {
		m, ok := qcDef.NextMatch()
		if !ok {
			break
		}
		
        var fieldName, fieldType string
        var fieldNode *sitter.Node
        var paramName, paramType string
        var paramNode *sitter.Node

		for _, c := range m.Captures {
			captureName := qDef.CaptureNameForId(c.Index)
			nodeContent := c.Node.Content(content)

            switch captureName {
            case "field.name":
                fieldName = nodeContent
                fieldNode = c.Node
            case "field.type":
                fieldType = nodeContent
            case "param.name":
                paramName = nodeContent
                paramNode = c.Node
            case "param.type":
                paramType = nodeContent
            
            case "class.name", "function.name", "method.name":
                var label string
                var nodeType string
                
                if strings.HasPrefix(captureName, "class") {
                    label = "Class"
                    nodeType = "class"
                } else if strings.HasPrefix(captureName, "function") || strings.HasPrefix(captureName, "method") {
                    label = "Function"
                }
                
                searchNode := c.Node
                if nodeType == "class" {
                    if p := c.Node.Parent(); p != nil {
                        searchNode = p
                    }
                }
                enclosingClass := findEnclosingTSClass(searchNode, content)
                
                var fullID string
                if enclosingClass != "" {
                    fullID = fmt.Sprintf("%s:%s.%s", filePath, enclosingClass, nodeContent)
                } else {
                    fullID = fmt.Sprintf("%s:%s", filePath, nodeContent)
                }
                
                n := &graph.Node{
                    ID:    fullID,
                    Label: label,
                    Properties: map[string]interface{}{
                        "name": nodeContent,
                        "file": filePath,
                        "line": int(c.Node.StartPoint().Row + 1),
                        "end_line": int(c.Node.EndPoint().Row + 1),
                    },
                }
                nodes = append(nodes, n)
            }
		}

        if fieldName != "" && fieldType != "" {
            parentClass := findEnclosingTSClass(m.Captures[0].Node, content)
            if parentClass != "" {
                classID := fmt.Sprintf("%s:%s", filePath, parentClass)
                
                // Create Field Node
                fieldID := fmt.Sprintf("%s:%s.%s", filePath, parentClass, fieldName)
                nodes = append(nodes, &graph.Node{
                    ID:    fieldID,
                    Label: "Field",
                    Properties: map[string]interface{}{
                        "name": fieldName,
                        "type": fieldType,
                        "file": filePath,
                        "line": int(fieldNode.StartPoint().Row + 1),
                    },
                })
                
                // DEFINES Edge
                edges = append(edges, &graph.Edge{
                    SourceID: classID,
                    TargetID: fieldID,
                    Type:     "DEFINES",
                })
                
                types := extractTypes(fieldType)
                for _, tName := range types {
                    resolved := resolveTargetID(tName, filePath)
                    edges = append(edges, &graph.Edge{
                        SourceID: classID, // Dependency is from Class or Field? Usually Class depends on Type.
                        TargetID: resolved,
                        Type:     "DEPENDS_ON",
                    })
                }
            }
        }

        if paramName != "" && paramType != "" && paramNode != nil {
             p1 := paramNode.Parent() // required_parameter
             if p1 != nil {
                 p2 := p1.Parent() // formal_parameters
                 if p2 != nil {
                     p3 := p2.Parent() // method_definition
                     if p3 != nil && p3.Type() == "method_definition" {
                         nameNode := p3.ChildByFieldName("name")
                         if nameNode != nil && nameNode.Content(content) == "constructor" {
                             parentClass := findEnclosingTSClass(p3, content)
                             if parentClass != "" {
                                classID := fmt.Sprintf("%s:%s", filePath, parentClass)
                                
                                types := extractTypes(paramType)
                                for _, tName := range types {
                                    resolved := resolveTargetID(tName, filePath)
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
	}

	// 3. Inheritance Query (Unchanged)
	inheritanceQueryStr := `
		(class_declaration
			name: (type_identifier) @class.name
			(class_heritage
				(extends_clause (_) @extends.target)?
				(implements_clause (_) @implements.target)*
			)?
		)
	`
    
	qInh, err := sitter.NewQuery([]byte(inheritanceQueryStr), typescript.GetLanguage())
	if err != nil {
        return nil, nil, fmt.Errorf("invalid inheritance query: %w", err)
    }
    defer qInh.Close()
    
    qcInh := sitter.NewQueryCursor()
    defer qcInh.Close()
    qcInh.Exec(qInh, tree.RootNode())
    
    for {
        m, ok := qcInh.NextMatch()
        if !ok {
            break
        }
        
        var className string
        var extendsTarget string
        var implementsTargets []string
        var classNode *sitter.Node
        
        for _, c := range m.Captures {
            name := qInh.CaptureNameForId(c.Index)
            contentStr := c.Node.Content(content)
            
            if name == "class.name" {
                className = contentStr
                classNode = c.Node
            } else if name == "extends.target" {
                if contentStr != "extends" {
                    extendsTarget = contentStr
                }
            } else if name == "implements.target" {
                if contentStr != "implements" && contentStr != "," {
                     implementsTargets = append(implementsTargets, contentStr)
                }
            }
        }
        
        if className != "" {
             searchNode := classNode
            if p := classNode.Parent(); p != nil {
                searchNode = p
            }
            enclosingClass := findEnclosingTSClass(searchNode, content)
            
            var sourceID string
            if enclosingClass != "" {
                sourceID = fmt.Sprintf("%s:%s.%s", filePath, enclosingClass, className)
            } else {
                sourceID = fmt.Sprintf("%s:%s", filePath, className)
            }
            
            if extendsTarget != "" {
                if idx := strings.Index(extendsTarget, "<"); idx != -1 {
                    extendsTarget = extendsTarget[:idx]
                }
                extendsTarget = strings.TrimSpace(extendsTarget)

                targetID := resolveTargetID(extendsTarget, filePath)
                edges = append(edges, &graph.Edge{
                    SourceID: sourceID,
                    TargetID: targetID,
                    Type:     "EXTENDS",
                })
            }
            
            for _, imp := range implementsTargets {
                if idx := strings.Index(imp, "<"); idx != -1 {
                    imp = imp[:idx]
                }
                imp = strings.TrimSpace(imp)
                targetID := resolveTargetID(imp, filePath)
                edges = append(edges, &graph.Edge{
                    SourceID: sourceID,
                    TargetID: targetID,
                    Type:     "IMPLEMENTS",
                })
            }
        }
    }

	// 4. Reference/Call Query (Unchanged)
	refQueryStr := `
		(call_expression
		  function: [
			(identifier) @call.target
			(member_expression property: (property_identifier) @call.target)
		  ]
		) @call.site
		
		(new_expression
		  constructor: (identifier) @call.target
		) @call.site
	`
	qRef, err := sitter.NewQuery([]byte(refQueryStr), typescript.GetLanguage())
	if err != nil {
		return nodes, edges, fmt.Errorf("invalid reference query: %w", err)
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
			sourceFuncNode := findEnclosingTSFunctionNode(callNode)
			if sourceFuncNode != nil {
				funcName := extractTSFunctionName(sourceFuncNode, content)
				if funcName != "" {
					enclosingClass := findEnclosingTSClass(sourceFuncNode, content)
					var sourceID string
					if enclosingClass != "" {
						sourceID = fmt.Sprintf("%s:%s.%s", filePath, enclosingClass, funcName)
					} else {
						sourceID = fmt.Sprintf("%s:%s", filePath, funcName)
					}
					
					targetID := resolveTargetID(targetName, filePath)
					edges = append(edges, &graph.Edge{
						SourceID: sourceID,
						TargetID: targetID,
						Type:     "CALLS",
					})
				}
			}
		}
	}

	return nodes, edges, nil
}

func resolveTSPath(currentFile, importPath string) string {
    importPath = strings.Trim(importPath, "\"'`")
    if strings.HasPrefix(importPath, ".") {
        dir := filepath.Dir(currentFile)
        resolved := filepath.Join(dir, importPath)
        if filepath.Ext(resolved) == "" {
            resolved += ".ts"
        }
        return filepath.ToSlash(resolved)
    }
    return importPath
}

func findEnclosingTSClass(n *sitter.Node, content []byte) string {
    curr := n.Parent()
    for curr != nil {
        if curr.Type() == "class_declaration" || curr.Type() == "interface_declaration" {
            nameNode := curr.ChildByFieldName("name")
            if nameNode != nil {
                return nameNode.Content(content)
            }
        }
        curr = curr.Parent()
    }
    return ""
}

func findEnclosingTSFunctionNode(n *sitter.Node) *sitter.Node {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "function_declaration" || t == "generator_function_declaration" || t == "method_definition" {
			return curr
		}
		if t == "arrow_function" || t == "function_expression" {
			 if curr.Parent() != nil && curr.Parent().Type() == "variable_declarator" {
				 return curr.Parent()
			 }
		}
		curr = curr.Parent()
	}
	return nil
}

func extractTSFunctionName(n *sitter.Node, content []byte) string {
	t := n.Type()
	if t == "variable_declarator" {
		nameNode := n.ChildByFieldName("name")
		if nameNode != nil {
			return nameNode.Content(content)
		}
	} else {
		nameNode := n.ChildByFieldName("name")
		if nameNode != nil {
			return nameNode.Content(content)
		}
	}
	return ""
}
