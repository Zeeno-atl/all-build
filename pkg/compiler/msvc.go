package compiler

import (
	"path/filepath"
	"strings"

	"github.com/Zeeno-atl/all-build/internal/utils"
)

type MSVC struct {
	Compiler

	inputs  []string
	outputs []string
}

func getIncludePaths(commands []string) []string {

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

		if cmd == "-I" || cmd == "/I" {
			state = StateInclude
			continue
		}
		if strings.HasPrefix(cmd, "-I") || strings.HasPrefix(cmd, "/I") {
			ret = append(ret, cmd[2:])
			continue
		}
		if strings.HasPrefix(cmd, "-external:I") || strings.HasPrefix(cmd, "/external:I") {
			ret = append(ret, cmd[11:])
			continue
		}
	}

	return ret
}

func getInputFiles(commands []string) []string {
	ret := make([]string, 0)
	for _, cmd := range commands {
		if !strings.HasPrefix(cmd, "-") && !strings.HasPrefix(cmd, "/") {
			ret = append(ret, cmd)
		}
	}
	return ret
}

func getOutputFiles(commands []string) []string {
	ret := make([]string, 0)
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, "-Fo") || strings.HasPrefix(cmd, "/Fo") {
			ret = append(ret, cmd[3:])
		}
	}

	if len(ret) == 0 {
		suffix := ".exe"
		if utils.Contains(commands, "-c") || utils.Contains(commands, "/c") {
			suffix = ".obj"
		}
		inputNames := getInputFiles(commands)
		ret = utils.Map(inputNames, func(name string) string {
			return strings.TrimSuffix(name, filepath.Ext(name)) + suffix
		})
	}

	return ret
}

func (c *MSVC) Parse(args []string) error {
	c.inputs = getIncludePaths(args)
	c.inputs = append(c.inputs, getInputFiles(args)...)

	c.outputs = getOutputFiles(args)
	return nil
}

func (c *MSVC) GetInputs() []string {
	return c.inputs
}

func (c *MSVC) GetOutputs() []string {
	return c.outputs
}
