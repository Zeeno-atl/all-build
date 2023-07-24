package compiler

import (
	"path/filepath"
	"strings"

	"github.com/Zeeno-atl/all-build/internal/utils"
)

type GCC struct {
	Compiler

	inputs  []string
	outputs []string
}

func gccGetIncludePaths(commands []string) []string {

	const (
		StateParsing = iota
		StateInclude
	)

	state := StateParsing

	ret := make([]string, 0)

	for _, cmd := range commands {

		if state == StateInclude {
			state = StateParsing
			ret = append(ret, cmd)
			continue
		}

		if cmd == "-I" || cmd == "-isystem" {
			state = StateInclude
			continue
		}
		if strings.HasPrefix(cmd, "-I") {
			ret = append(ret, cmd[2:])
			continue
		}
		if strings.HasPrefix(cmd, "-isystem") {
			ret = append(ret, cmd[8:])
			continue
		}
	}

	return ret
}

func gccGetInputFiles(commands []string) []string {
	ret := make([]string, 0)
	for _, cmd := range commands {
		if !strings.HasPrefix(cmd, "-") {
			ret = append(ret, cmd)
		}
	}
	// also base folders, because you can have relative includes
	for _, cmd := range ret {
		ret = append(ret, filepath.Base(cmd))
	}

	return ret
}

func gccGetOutputFiles(commands []string) []string {
	const (
		StateParsing = iota
		StateOutput
	)

	state := StateParsing

	ret := make([]string, 0)
	for _, cmd := range commands {
		if state == StateOutput {
			state = StateParsing
			ret = append(ret, cmd)
			continue
		}

		if cmd == "-o" {
			state = StateOutput
			continue
		}
		if strings.HasPrefix(cmd, "-o") {
			ret = append(ret, cmd[2:])
		}
	}

	if len(ret) == 0 {
		out := "a.out"
		inputNames := getInputFiles(commands)
		if utils.Contains(commands, "-c") && len(inputNames) > 0 {
			out = inputNames[0] + ".o"
		}
		return []string{out}
	}

	return ret
}

func (c *GCC) Parse(args []string) error {
	c.inputs = gccGetIncludePaths(args)
	c.inputs = append(c.inputs, gccGetInputFiles(args)...)
	c.outputs = gccGetOutputFiles(args)
	return nil
}

func (c *GCC) GetInputs() []string {
	return c.inputs
}

func (c *GCC) GetOutputs() []string {
	return c.outputs
}
