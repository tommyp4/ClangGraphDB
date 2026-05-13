package analysis_test

import (
	"testing"

	"clang-graphdb/internal/analysis"
)

func TestParseVBNet_Namespace(t *testing.T) {
	parser, ok := analysis.GetParser(".vb")
	if !ok {
		t.Fatalf("VB.NET parser not registered")
	}

	content := []byte(`
Namespace MyOrg.Utils
    Public Class MathHelper
        Public Function Add(a As Integer, b As Integer) As Integer
            Return a + b
        End Function
    End Class
End Namespace
`)

	nodes, _, err := parser.Parse("dummy.vb", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ids := make(map[string]bool)
	for _, n := range nodes {
		ids[n.ID] = true
	}

	expectedClassID := "MyOrg.Utils.MathHelper"
	expectedFuncID := "MyOrg.Utils.MathHelper.Add"

	if !ids[expectedClassID] {
		t.Errorf("Expected Class ID %s not found. Found: %+v", expectedClassID, ids)
	}

	if !ids[expectedFuncID] {
		t.Errorf("Expected Function ID %s not found. Found: %+v", expectedFuncID, ids)
	}
}
