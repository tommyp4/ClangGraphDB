package analysis

import (
	"testing"
)

func TestGenerateNodeID_ClassVsConstructor(t *testing.T) {
	// A Class and its Constructor might have the same FQN
	fqn := "MyApp.Models.User"
	classID := GenerateNodeID("Class", fqn, "")
	constructorID := GenerateNodeID("Function", fqn, "")

	if classID == constructorID {
		t.Errorf("Expected different IDs for Class and Constructor with same FQN, got: %s", classID)
	}

	expectedClassID := "Class:MyApp.Models.User:"
	if classID != expectedClassID {
		t.Errorf("Expected %s, got %s", expectedClassID, classID)
	}
}

func TestGenerateNodeID_FieldVsMethod(t *testing.T) {
	// A Field and a Method might have the same FQN
	fqn := "MyApp.Models.User.Count"
	fieldID := GenerateNodeID("Field", fqn, "")
	methodID := GenerateNodeID("Function", fqn, "()")

	if fieldID == methodID {
		t.Errorf("Expected different IDs for Field and Method with same FQN, got: %s", fieldID)
	}
}

func TestGenerateNodeID_Overloads(t *testing.T) {
	// Overloaded methods with same FQN but different signatures
	fqn := "MyApp.Services.Processor.Process"
	method1ID := GenerateNodeID("Function", fqn, "(int)")
	method2ID := GenerateNodeID("Function", fqn, "(string)")

	if method1ID == method2ID {
		t.Errorf("Expected different IDs for overloaded methods, got: %s", method1ID)
	}

	expected1 := "Function:MyApp.Services.Processor.Process:(int)"
	if method1ID != expected1 {
		t.Errorf("Expected %s, got %s", expected1, method1ID)
	}
}
