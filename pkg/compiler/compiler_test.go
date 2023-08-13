package compiler

import (
	"strings"
	"testing"
)

func TestCompilerGccGetIncludePaths(t *testing.T) {
	gcc := NewCompiler(GCCCompiler)
	if gcc == nil {
		t.Fatalf("Expected compiler to be not nil")
	}

	commands := []string{
		"-I", "include",
		"-isystem", "system",
		"-Iinclude2", "-isystemsystem2",
	}

	gcc.Parse(commands)

	includes := GetInputs(gcc)

	if len(includes) != 4 {
		t.Errorf("Expected 4 includes, got %d ['%s']", len(includes), strings.Join(includes, "', '"))
	}

	if includes[0] != "include" {
		t.Errorf("Expected include, got %s", includes[0])
	}

	if includes[1] != "system" {
		t.Errorf("Expected system, got %s", includes[1])
	}

	if includes[2] != "include2" {
		t.Errorf("Expected include2, got %s", includes[2])
	}

	if includes[3] != "system2" {
		t.Errorf("Expected system2, got %s", includes[3])
	}
}

func TestCompilerGCCAddOFlag(t *testing.T) {
	gcc := NewCompiler(GCCCompiler)
	if gcc == nil {
		t.Fatalf("Expected compiler to be not nil")
	}

	commands := []string{
		"CMakeCXXCompilerId.cpp",
	}

	gcc.Parse(commands)

	outputs := GetOutputs(gcc)

	if len(outputs) != 1 {
		t.Errorf("Expected 1 output, got %d ['%s']", len(outputs), strings.Join(outputs, "', '"))
	}

	if outputs[0] != "a.out" {
		t.Errorf("Expected a.out, got %s", outputs[0])
	}
}
