package compiler

import (
	"path/filepath"

	"github.com/Zeeno-atl/all-build/internal/utils"
)

const (
	ArgumentTypeInput  = "input"
	ArgumentTypeOutput = "output"
)

type IArgument interface {
	IsInput() bool
	IsOutput() bool
	Type() string
	Command() string
	Parameter() string

	Chroot(path string)
	Stringify() string
}

type ICompiler interface {
	Parse(args []string) error
	Arguments() []IArgument
	Chroot(path string) // Does mapping of folders
}

func GetInputs(c ICompiler) []string {
	inputs := utils.Map(utils.Filter(c.Arguments(), func(arg IArgument) bool {
		return arg.IsInput() || arg.Command() == ""
	}), func(arg IArgument) string {
		return arg.Parameter()
	})
	for i, input := range inputs {
		if filepath.IsAbs(input) {
			inputs[i] = input //TODO: remap filepath.Join(c.basePath, input)
		}
	}
	return inputs
}

func GetOutputs(c ICompiler) []string {
	outputs := utils.Map(utils.Filter(c.Arguments(), func(arg IArgument) bool {
		return arg.IsOutput()
	}), func(arg IArgument) string {
		return arg.Parameter()
	})
	for i, output := range outputs {
		if filepath.IsAbs(output) {
			outputs[i] = output //TODO: remap filepath.Join(c.basePath, output)
		}
	}
	return outputs
}

func GetCommand(c ICompiler) []string {
	command := make([]string, 0)
	for _, arg := range c.Arguments() {
		command = append(command, arg.Stringify())
	}
	return command
}

const (
	MSVCCompiler = "msvc"
	GCCCompiler  = "gcc"
)

func NewCompiler(compiler string) ICompiler {
	switch compiler {
	case MSVCCompiler:
		return &MSVC{}
	case GCCCompiler:
		return &GCC{}
	default:
		return nil
	}
}
