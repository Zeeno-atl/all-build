package compiler

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/Zeeno-atl/all-build/internal/utils"
)

type GCCArgument struct {
	IArgument

	argType   string
	command   string
	parameter string
	basePath  string
}

func (a *GCCArgument) IsInput() bool {
	return a.Type() == ArgumentTypeInput
}

func (a *GCCArgument) IsOutput() bool {
	return a.Type() == ArgumentTypeOutput
}

func (a *GCCArgument) Type() string {
	return a.argType
}

func (a *GCCArgument) Command() string {
	return a.command
}

func (a *GCCArgument) Parameter() string {
	if a.IsInput() || a.IsOutput() || a.command == "" {
		return filepath.Join(a.basePath, a.parameter)
	}
	return a.parameter
}

func (a *GCCArgument) Stringify() string {
	command := a.command
	if a.parameter != "" && a.command != "" {
		command += a.Parameter()
	} else if a.parameter != "" {
		command = a.Parameter()
	}
	return command
}

func (a *GCCArgument) Chroot(path string) {
	a.basePath = path
}

func gccInputs() []string {
	inputs := []string{"-isystem", "-I"}
	// sort by length, so that we can match the longest first (consider prefixes "-isystem" and "-i")
	sort.Slice(inputs, func(i int, j int) bool {
		return len(inputs[i]) > len(inputs[j])
	})
	return inputs
}

func gccOutputs() []string {
	outputs := []string{"-o"}
	// see why we sort by length in gccInputs
	sort.Slice(outputs, func(i int, j int) bool {
		return len(outputs[i]) > len(outputs[j])
	})
	return outputs
}

func parseArguments(args []string) []GCCArgument {
	arguments := make([]GCCArgument, 0)

	inputTypes := gccInputs()
	outputTypes := gccOutputs()

	const (
		StateParsing = iota
		StateGettingParameter
	)

	state := StateParsing
	argument := GCCArgument{}

	for _, arg := range args {
		if state == StateGettingParameter {
			state = StateParsing
			argument.parameter = arg
			arguments = append(arguments, argument)
			continue
		}

		// First, exact matches are looked for, so we the next arg is the path

		if utils.Contains(inputTypes, arg) {
			state = StateGettingParameter
			argument = GCCArgument{command: arg, argType: ArgumentTypeInput}
			continue
		}

		if utils.Contains(outputTypes, arg) {
			state = StateGettingParameter
			argument = GCCArgument{command: arg, argType: ArgumentTypeOutput}
			continue
		}

		// It is possible to not have a space between the argument and the parameter

		if prefix, ok := utils.Prefix(arg, inputTypes); ok {
			state = StateParsing
			argument = GCCArgument{command: arg[:len(prefix)], argType: ArgumentTypeInput, parameter: arg[len(prefix):]}
			arguments = append(arguments, argument)
			continue
		}

		if prefix, ok := utils.Prefix(arg, outputTypes); ok {
			state = StateParsing
			argument = GCCArgument{command: arg[:len(prefix)], argType: ArgumentTypeOutput, parameter: arg[len(prefix):]}
			arguments = append(arguments, argument)
			continue
		}

		state = StateParsing
		if strings.HasPrefix(arg, "-") {
			arguments = append(arguments, GCCArgument{command: arg})
		} else {
			arguments = append(arguments, GCCArgument{parameter: arg})
		}
	}

	return arguments
}

type GCC struct {
	ICompiler

	arguments []GCCArgument
}

func (c *GCC) Arguments() []IArgument {
	return utils.Map[GCCArgument, IArgument](c.arguments, func(arg GCCArgument) IArgument {
		return &arg
	})
}

func (c *GCC) Parse(args []string) error {
	c.arguments = parseArguments(args)

	hasFlag := func(flag string) bool {
		return utils.ContainsIf(c.arguments, func(arg GCCArgument) bool {
			return arg.command == flag
		})
	}

	hasInput := utils.ContainsIf(c.arguments, func(arg GCCArgument) bool {
		extension := filepath.Ext(arg.parameter)
		return arg.command == "" && (extension == ".c" || extension == ".cpp")
	})

	shouldAddDefaultOutput := false

	// if has flag -c and not -o
	if hasFlag("-c") && !hasFlag("-o") {
		shouldAddDefaultOutput = true
	}

	// if has no -c and no -o and has input files
	if !hasFlag("-c") && !hasFlag("-o") && hasInput {
		shouldAddDefaultOutput = true
	}

	if shouldAddDefaultOutput {
		c.arguments = append(c.arguments, GCCArgument{command: "-o", argType: ArgumentTypeOutput, parameter: "a.out"})
	}
	return nil
}

func (c *GCC) Chroot(path string) {
	for i, arg := range c.arguments {
		arg.Chroot(path)
		c.arguments[i] = arg
	}
}

func (c *GCC) GetCommand() []string {
	command := make([]string, 0)

	for _, arg := range c.arguments {
		command = append(command, arg.Stringify())
	}

	return command
}
