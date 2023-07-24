package compiler

type Compiler interface {
	Parse(args []string) error

	GetInputs() []string
	GetOutputs() []string
}

const (
	MSVCCompiler = "msvc"
	GCCCompiler  = "gcc"
)

func NewCompiler(compiler string) Compiler {
	switch compiler {
	case MSVCCompiler:
		return &MSVC{}
	case GCCCompiler:
		return &GCC{}
	default:
		return nil
	}
}
