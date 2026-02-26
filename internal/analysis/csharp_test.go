package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"graphdb/internal/analysis"
)

func TestParseCSharp(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/csharp/sample.cs")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content := []byte(`using System;
public class Greeter {
    public void Greet(string name) {
        Console.WriteLine("Hello " + name);
    }
}`)

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	foundGreet := false
	foundGreeter := false

	for _, n := range nodes {
		name, _ := n.Properties["name"].(string)
		if name == "Greet" && n.Label == "Function" {
			foundGreet = true
			if _, ok := n.Properties["end_line"]; !ok {
				t.Errorf("Function 'Greet' missing end_line")
			}
			if _, ok := n.Properties["content"]; ok {
				t.Errorf("Function 'Greet' should not have content property")
			}
		}
		if name == "Greeter" && n.Label == "Class" {
			foundGreeter = true
			if _, ok := n.Properties["end_line"]; !ok {
				t.Errorf("Class 'Greeter' missing end_line")
			}
			if _, ok := n.Properties["content"]; ok {
				t.Errorf("Class 'Greeter' should not have content property")
			}
		}
	}

	if !foundGreeter {
		t.Errorf("Expected Class 'Greeter' not found")
	}
	if !foundGreet {
		t.Errorf("Expected Function 'Greet' not found")
	}

	// Verify Call Edge
	foundCall := false
	for _, e := range edges {
		// Source: Function:...:Greet:(string)
		// Target: WriteLine OR System.WriteLine (Resolution candidates)
		if strings.Contains(e.SourceID, "Greet") && (strings.HasSuffix(e.TargetID, "WriteLine") || e.TargetID == "WriteLine") {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Errorf("Expected Call Edge from Greet to WriteLine not found")
	}
}

func TestParseCSharp_ClassAndConstructor(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	absPath, err := filepath.Abs("dummy_collision.cs")
	content := []byte(`
namespace MyApp.Core;

public class User {
    public User() { }
    public void Save() { }
}

public class Order {
    public Order() { }
    public void Save() { }
}
`)

	nodes, _, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ids := make(map[string]int)
	for _, n := range nodes {
		ids[n.ID]++
	}

	for id, count := range ids {
		if count > 1 {
			t.Errorf("Duplicate ID found: %s (Count: %d)", id, count)
		}
	}

	// Expected specific IDs (Qualified with namespace and class)
	expectedIDs := []string{
		"Class:MyApp.Core.User:",
		"Function:MyApp.Core.User.User:()", // Constructor
		"Function:MyApp.Core.User.Save:()",
		"Class:MyApp.Core.Order:",
		"Function:MyApp.Core.Order.Order:()", // Constructor
		"Function:MyApp.Core.Order.Save:()",
	}

	for _, expected := range expectedIDs {
		if _, exists := ids[expected]; !exists {
			t.Errorf("Expected ID not found: %s", expected)
		}
	}
}

func TestParseCSharp_DependencyInjection(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	relPath := "../../test/fixtures/csharp/di_sample.cs"
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	nodes, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify PaymentProcessor depends on IPaymentRepository
	foundRepo := false
	foundLogger := false

	for _, e := range edges {
		if e.Type == "DEPENDS_ON" {
			t.Logf("Found DEPENDS_ON Edge: %s -> %s", e.SourceID, e.TargetID)
			// Source: ...:Trucks.Processor.PaymentProcessor
			// Target: Trucks.Common.IPaymentRepository (or similar)
			if strings.Contains(e.SourceID, "PaymentProcessor") && strings.Contains(e.TargetID, "IPaymentRepository") {
				foundRepo = true
			}
			// Verify generic handling: ILogger<T> -> ILogger
			if strings.Contains(e.SourceID, "PaymentProcessor") && strings.Contains(e.TargetID, "ILogger") {
				foundLogger = true
			}
		}
	}

	if !foundRepo {
		t.Errorf("Expected DEPENDS_ON edge from PaymentProcessor to IPaymentRepository not found")
	}
	if !foundLogger {
		t.Errorf("Expected DEPENDS_ON edge from PaymentProcessor to ILogger not found")
	}
	
	// Check if we captured nodes correctly
	foundClass := false
	for _, n := range nodes {
		if strings.Contains(n.ID, "PaymentProcessor") && n.Label == "Class" {
			foundClass = true
		}
	}
	
	if !foundClass {
		t.Errorf("Expected Class PaymentProcessor not found")
	}
}

func TestParseCSharp_DependencyResolution(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	relPath := "../../test/fixtures/csharp/di_resolution.cs"
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	_, edges, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify PaymentController.Post -> IPaymentService.ProcessPayment
	// Expected behavior: Payment.Controllers.IPaymentService.ProcessPayment

	expectedSource := "Payment.Controllers.PaymentController.Post"
	expectedTarget := "Payment.Controllers.IPaymentService.ProcessPayment"

	foundCall := false
	for _, e := range edges {
		if e.Type == "CALLS" && strings.Contains(e.SourceID, "Post") {
			t.Logf("Found CALLS: %s -> %s", e.SourceID, e.TargetID)
			if strings.Contains(e.SourceID, expectedSource) && strings.Contains(e.TargetID, expectedTarget) {
				foundCall = true
				break
			}
		}
	}

	if !foundCall {
		t.Errorf("Expected CALLS edge from %s to %s not found", expectedSource, expectedTarget)
	}
}

func TestParseCSharp_IDCollision(t *testing.T) {
	parser, ok := analysis.GetParser(".cs")
	if !ok {
		t.Fatalf("CSharp parser not registered")
	}

	absPath, err := filepath.Abs("../../test/fixtures/csharp/IDCollisionTest.cs")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	nodes, _, err := parser.Parse(absPath, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ids := make(map[string]int)
	for _, n := range nodes {
		ids[n.ID]++
		// Verify fqn property is populated
		if n.Properties["fqn"] == nil || n.Properties["fqn"] == "" {
			t.Errorf("Node missing fqn property: %s", n.ID)
		}
	}

	for id, count := range ids {
		if count > 1 {
			t.Errorf("Duplicate ID found: %s (Count: %d)", id, count)
		}
	}
}

