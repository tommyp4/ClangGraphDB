package analysis

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/sql"
	"clang-graphdb/internal/graph"
)

type SqlParser struct{}

func init() {
	p := &SqlParser{}
	RegisterParser(".sql", p)
}

func (p *SqlParser) Parse(filePath string, content []byte) ([]*graph.Node, []*graph.Edge, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(sql.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()

	// 1. Definition Query
	// Capture the entire function definition to manually inspect name
	defQueryStr := `
		(create_function) @function.def
	`

	qDef, err := sitter.NewQuery([]byte(defQueryStr), sql.GetLanguage())
	if err != nil {
		return nil, nil, fmt.Errorf("invalid definition query: %w", err)
	}
	defer qDef.Close()

	qcDef := sitter.NewQueryCursor()
	defer qcDef.Close()

	qcDef.Exec(qDef, tree.RootNode())

	var nodes []*graph.Node

	for {
		m, ok := qcDef.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			// Iterate children to find name (object_reference or identifier)
			var nodeName string
			count := int(c.Node.ChildCount())
			for i := 0; i < count; i++ {
				child := c.Node.Child(i)
				if child.Type() == "object_reference" || child.Type() == "identifier" {
					nodeName = child.Content(content)
					break
				}
			}

			if nodeName == "" {
				continue
			}
			
			// Build fqn = FilePath:Schema.Name
			fqn := fmt.Sprintf("%s:%s", filePath, nodeName)

			id := GenerateNodeID("Function", fqn, "")

			n := &graph.Node{
				ID:    id,
				Label: "Function",
				Properties: map[string]interface{}{
					"name": nodeName,
					"fqn":  fqn,
					"file": filePath,
					"start_line": c.Node.Parent().StartPoint().Row + 1,
					"end_line": c.Node.Parent().EndPoint().Row + 1,
				},
			}
			nodes = append(nodes, n)
		}
	}

	// 2. Reference/Call Query
	// Manual inspection for calls too? 
	// No, calls are usually `invocation -> object_reference`.
    // Try to capture `invocation` and inspect?
	refQueryStr := `
		(invocation) @call.site
	`

	qRef, err := sitter.NewQuery([]byte(refQueryStr), sql.GetLanguage())
	if err != nil {
		return nodes, nil, fmt.Errorf("invalid reference query: %w", err)
	}
	defer qRef.Close()

	qcRef := sitter.NewQueryCursor()
	defer qcRef.Close()

	qcRef.Exec(qRef, tree.RootNode())

	var edges []*graph.Edge

	for {
		m, ok := qcRef.NextMatch()
		if !ok {
			break
		}

		var targetName string
		var callNode *sitter.Node

		for _, c := range m.Captures {
            // Find target inside invocation
            count := int(c.Node.ChildCount())
            for i := 0; i < count; i++ {
                child := c.Node.Child(i)
                if child.Type() == "object_reference" || child.Type() == "identifier" {
                    targetName = child.Content(content)
                    break
                }
            }
            callNode = c.Node
		}

		if targetName != "" && callNode != nil {
			sourceFunc := findEnclosingSqlFunction(callNode, content)
			if sourceFunc != "" {
				sourceFQN := fmt.Sprintf("%s:%s", filePath, sourceFunc)
				sourceID := GenerateNodeID("Function", sourceFQN, "")
				targetFQN := fmt.Sprintf("%s:%s", filePath, targetName)

				edges = append(edges, &graph.Edge{
					SourceID: sourceID,
					TargetID: targetFQN,
					Type:     "CALLS",
				})
			}
		}
	}

	return nodes, edges, nil
}

func findEnclosingSqlFunction(n *sitter.Node, content []byte) string {
	curr := n.Parent()
	for curr != nil {
		t := curr.Type()
		if t == "create_function" {
            // Find child that is object_reference OR identifier
            count := int(curr.ChildCount())
            for i := 0; i < count; i++ {
                child := curr.Child(i)
                if child.Type() == "object_reference" || child.Type() == "identifier" {
                     return child.Content(content)
                }
            }
		}
		curr = curr.Parent()
	}
	return ""
}
