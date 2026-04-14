package analysis

import (
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestPythonParser_Registration(t *testing.T) {
	parser, ok := GetParser(".py")
	assert.True(t, ok, "Python parser should be registered for .py extension")
	assert.NotNil(t, parser, "Python parser should not be nil")
}

func TestPythonParser_ParseDefinitions(t *testing.T) {
	parser, ok := GetParser(".py")
	assert.True(t, ok)

	content := []byte(`
class MyClass:
    def my_method(self):
        pass

def my_function():
    pass
`)
	nodes, _, err := parser.Parse("test.py", content)
	assert.NoError(t, err)

	foundClass := false
	foundMethod := false
	foundFunction := false

	for _, n := range nodes {
		name := n.Properties["name"].(string)
		if n.Label == "Class" && name == "MyClass" {
			foundClass = true
			assert.Equal(t, "test.py:MyClass", n.Properties["fqn"])
		}
		if n.Label == "Function" && name == "my_method" {
			foundMethod = true
			assert.Equal(t, "test.py:MyClass.my_method", n.Properties["fqn"])
		}
		if n.Label == "Function" && name == "my_function" {
			foundFunction = true
			assert.Equal(t, "test.py:my_function", n.Properties["fqn"])
		}
	}

	assert.True(t, foundClass, "Class MyClass not found")
	assert.True(t, foundMethod, "Method my_method not found")
	assert.True(t, foundFunction, "Function my_function not found")
}

func TestPythonParser_ParseRelationships(t *testing.T) {
	parser, ok := GetParser(".py")
	assert.True(t, ok)

	content := []byte(`
class Parent:
    def base_method(self):
        pass

class Child(Parent):
    def child_method(self):
        self.base_method()
        global_function()

def global_function():
    pass
`)
	_, edges, err := parser.Parse("test.py", content)
	assert.NoError(t, err)

	foundExtends := false
	foundCallBase := false
	foundCallGlobal := false

	for _, e := range edges {
		if e.Type == "EXTENDS" &&
			strings.Contains(e.SourceID, "test.py:Child") &&
			strings.Contains(e.TargetID, "test.py:Parent") {
			foundExtends = true
		}
		if e.Type == "CALLS" &&
			strings.Contains(e.SourceID, "test.py:Child.child_method") &&
			strings.Contains(e.TargetID, "test.py:Child.base_method") {
			foundCallBase = true
		}
		if e.Type == "CALLS" &&
			strings.Contains(e.SourceID, "test.py:Child.child_method") &&
			strings.Contains(e.TargetID, "test.py:global_function") {
			foundCallGlobal = true
		}
	}

	assert.True(t, foundExtends, "EXTENDS edge Child -> Parent not found")
	assert.True(t, foundCallBase, "CALLS edge child_method -> base_method not found")
	assert.True(t, foundCallGlobal, "CALLS edge child_method -> global_function not found")
}

func TestPythonParser_Imports(t *testing.T) {
	parser, ok := GetParser(".py")
	assert.True(t, ok)

	content := []byte(`
import os
import numpy as np
from typing import List

def my_function():
    os.path.join("a", "b")
    np.array([1, 2, 3])
    List()
`)
	_, edges, err := parser.Parse("test.py", content)
	assert.NoError(t, err)

	foundOS := false
	foundNP := false
	foundList := false

	for _, e := range edges {
		if e.Type == "CALLS" {
			if strings.Contains(e.TargetID, "os.path.join") {
				foundOS = true
			}
			if strings.Contains(e.TargetID, "numpy.array") {
				foundNP = true
			}
			if strings.Contains(e.TargetID, "typing.List") {
				foundList = true
			}
		}
	}

	assert.True(t, foundOS, "CALLS edge to os.path.join not found")
	assert.True(t, foundNP, "CALLS edge to numpy.array not found")
	assert.True(t, foundList, "CALLS edge to typing.List not found")
}
