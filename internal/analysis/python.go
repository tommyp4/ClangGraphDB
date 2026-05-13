package analysis

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	"clang-graphdb/internal/graph"
)

type PythonParser struct{}

func init() {
	RegisterParser(".py", &PythonParser{})
}

func (p *PythonParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	var nodes []*graph.Node
	var edges []*graph.Edge

	imports := make(map[string]string)

	importQueryStr := `
		(import_statement name: (dotted_name) @import.name)
		(import_statement (aliased_import name: (dotted_name) @import.name alias: (identifier) @import.alias))
		(import_from_statement module_name: (dotted_name) @import.module name: (dotted_name) @import.name)
	`
	qImport, err := sitter.NewQuery([]byte(importQueryStr), python.GetLanguage())
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

		var importName string
		var importAlias string
		var importModule string

		for _, c := range m.Captures {
			captureName := qImport.CaptureNameForId(c.Index)
			if captureName == "import.name" {
				importName = c.Node.Content(content)
			}
			if captureName == "import.alias" {
				importAlias = c.Node.Content(content)
			}
			if captureName == "import.module" {
				importModule = c.Node.Content(content)
			}
		}

		if importModule != "" {
			// from module import name
			imports[importName] = fmt.Sprintf("%s.%s", importModule, importName)
		} else if importAlias != "" {
			// import name as alias
			imports[importAlias] = importName
		} else {
			// import name
			imports[importName] = importName
		}
	}

	defQueryStr := `
		(class_definition name: (identifier) @class.name) @class.def
		(function_definition name: (identifier) @function.name) @function.def
	`
	qDef, err := sitter.NewQuery([]byte(defQueryStr), python.GetLanguage())
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

		var classNameNode *sitter.Node
		var classDefNode *sitter.Node
		var funcNameNode *sitter.Node
		var funcDefNode *sitter.Node

		for _, c := range m.Captures {
			captureName := qDef.CaptureNameForId(c.Index)
			if captureName == "class.name" {
				classNameNode = c.Node
			} else if captureName == "class.def" {
				classDefNode = c.Node
			} else if captureName == "function.name" {
				funcNameNode = c.Node
			} else if captureName == "function.def" {
				funcDefNode = c.Node
			}
		}

		if classNameNode != nil && classDefNode != nil {
			nodeContent := classNameNode.Content(content)
			fqn := fmt.Sprintf("%s:%s", filePath, nodeContent)
			nodeID := GenerateNodeID("Class", fqn, "")
			nodes = append(nodes, &graph.Node{
				ID:    nodeID,
				Label: "Class",
				Properties: map[string]interface{}{
					"name":       nodeContent,
					"fqn":        fqn,
					"file":       filePath,
					"start_line": int(classDefNode.StartPoint().Row + 1),
					"end_line":   int(classDefNode.EndPoint().Row + 1),
				},
			})
		}

		if funcNameNode != nil && funcDefNode != nil {
			nodeContent := funcNameNode.Content(content)
			enclosingClass := findEnclosingPythonClass(funcNameNode, content)
			var fqn string
			if enclosingClass != "" {
				fqn = fmt.Sprintf("%s:%s.%s", filePath, enclosingClass, nodeContent)
			} else {
				fqn = fmt.Sprintf("%s:%s", filePath, nodeContent)
			}

			nodeID := GenerateNodeID("Function", fqn, "")
			nodes = append(nodes, &graph.Node{
				ID:    nodeID,
				Label: "Function",
				Properties: map[string]interface{}{
					"name":       nodeContent,
					"fqn":        fqn,
					"file":       filePath,
					"start_line": int(funcDefNode.StartPoint().Row + 1),
					"end_line":   int(funcDefNode.EndPoint().Row + 1),
				},
			})
		}
	}

	// Inheritance
	inheritanceQueryStr := `
		(class_definition 
			name: (identifier) @class.name 
			superclasses: (argument_list (_) @extends.target)
		)
	`
	qInh, err := sitter.NewQuery([]byte(inheritanceQueryStr), python.GetLanguage())
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
		for _, c := range m.Captures {
			captureName := qInh.CaptureNameForId(c.Index)
			if captureName == "class.name" {
				className = c.Node.Content(content)
			}
			if captureName == "extends.target" {
				targetName := c.Node.Content(content)
				sourceFqn := fmt.Sprintf("%s:%s", filePath, className)
				sourceID := GenerateNodeID("Class", sourceFqn, "")

				// Simple heuristic: assume target is in the same file or already fully qualified
				targetFqn := fmt.Sprintf("%s:%s", filePath, targetName)
				targetID := GenerateNodeID("Class", targetFqn, "")

				edges = append(edges, &graph.Edge{
					SourceID: sourceID,
					TargetID: targetID,
					Type:     "EXTENDS",
				})
			}
		}
	}

	// Calls
	refQueryStr := `
		(call function: (identifier) @call.target) @call.site
		(call function: (attribute object: (_) @call.object attribute: (identifier) @call.target)) @call.site
	`
	qRef, err := sitter.NewQuery([]byte(refQueryStr), python.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid reference query: %w", err)
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
		var objectName string
		var callSiteNode *sitter.Node

		for _, c := range m.Captures {
			captureName := qRef.CaptureNameForId(c.Index)
			if captureName == "call.target" {
				targetName = c.Node.Content(content)
			}
			if captureName == "call.object" {
				objectName = c.Node.Content(content)
			}
			if captureName == "call.site" {
				callSiteNode = c.Node
			}
		}

		if callSiteNode != nil {
			enclosingFunc, enclosingClass := findEnclosingPythonFunctionAndClass(callSiteNode, content)
			if enclosingFunc != "" {
				var sourceFqn string
				if enclosingClass != "" {
					sourceFqn = fmt.Sprintf("%s:%s.%s", filePath, enclosingClass, enclosingFunc)
				} else {
					sourceFqn = fmt.Sprintf("%s:%s", filePath, enclosingFunc)
				}
				sourceID := GenerateNodeID("Function", sourceFqn, "")

				var targetFqn string
				if objectName == "self" && enclosingClass != "" {
					targetFqn = fmt.Sprintf("%s:%s.%s", filePath, enclosingClass, targetName)
				} else if objectName != "" {
					// Check if objectName is an import alias
					if resolved, ok := imports[objectName]; ok {
						targetFqn = fmt.Sprintf("%s:%s.%s", filePath, resolved, targetName)
					} else {
						targetFqn = fmt.Sprintf("%s:%s.%s", filePath, objectName, targetName)
					}
				} else {
					// Check if targetName itself is an import
					if resolved, ok := imports[targetName]; ok {
						targetFqn = fmt.Sprintf("%s:%s", filePath, resolved)
					} else {
						targetFqn = fmt.Sprintf("%s:%s", filePath, targetName)
					}
				}
				targetID := GenerateNodeID("Function", targetFqn, "")

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

func findEnclosingPythonClass(node *sitter.Node, content []byte) string {
	curr := node.Parent()
	for curr != nil {
		if curr.Type() == "class_definition" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return ""
}

func findEnclosingPythonFunctionAndClass(node *sitter.Node, content []byte) (string, string) {
	var functionName string
	var className string

	curr := node.Parent()
	for curr != nil {
		if functionName == "" && curr.Type() == "function_definition" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				functionName = nameNode.Content(content)
			}
		}
		if className == "" && curr.Type() == "class_definition" {
			nameNode := curr.ChildByFieldName("name")
			if nameNode != nil {
				className = nameNode.Content(content)
			}
		}
		curr = curr.Parent()
	}
	return functionName, className
}
