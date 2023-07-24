package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Zeeno-atl/all-build/internal/executor"
	"github.com/Zeeno-atl/all-build/internal/utils"
	"github.com/Zeeno-atl/all-build/pkg/compiler"
	"github.com/hibiken/asynq"
)

type CompileFile struct {
	Tag         string   `json:"tag"`
	Command     []string `json:"command"`
	Inputs      []File   `json:"inputs"`
	Outputs     []string `json:"outputs"`
	Environment []string `json:"environment"`
}

func walkFilesystem(path string) []string {
	files := make([]string, 0)
	filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func NewCompileFile(args []string, tag string, compilerType string) (*asynq.Task, error) {
	compiler := compiler.NewCompiler(compilerType)

	if compiler == nil {
		return nil, fmt.Errorf("unknown compiler: %s", compilerType)
	}

	compiler.Parse(args)

	inputs := compiler.GetInputs()
	inputs = utils.Unique(inputs)

	filePaths := make([]string, 0)

	for _, input := range inputs {
		filePaths = append(filePaths, walkFilesystem(input)...)
	}

	inputFiles := utils.Map(filePaths, func(path string) File {
		content, err := os.ReadFile(path)

		if err != nil {
			log.Fatalf("could not read file: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			log.Fatalf("could not get file info: %v", err)
		}

		return File{
			Path:    path,
			Content: content,
			Chmod:   int(info.Mode().Perm()),
		}
	})

	outputs := compiler.GetOutputs()

	cf := CompileFile{
		Tag:         tag,
		Command:     args,
		Inputs:      inputFiles,
		Outputs:     outputs,
		Environment: make([]string, 0),
	}

	payload, err := json.Marshal(cf)
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeCompileFile, payload), nil
}

type CompileFileHandler struct {
	Tools []executor.Tool
}

func NewCompileFileHandler(tools []executor.Tool) *CompileFileHandler {
	return &CompileFileHandler{Tools: tools}
}

func respondError(t *asynq.Task, err error) error {
	log.Printf("error: %v", err)
	responsePayload, err := json.Marshal(Response{
		ReturnCode: -1,
		Stdout:     "",
		Stderr:     fmt.Sprintf("%v", err),
		Files:      make([]File, 0),
	})
	if err != nil {
		panic(err)
	}

	t.ResultWriter().Write(responsePayload)
	return nil
}

func (h CompileFileHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p CompileFile
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	tool, ok := utils.Find(h.Tools, func(tool executor.Tool) bool { return tool.Tag == p.Tag })
	if !ok {
		return respondError(t, fmt.Errorf("could not find tool: %s", p.Tag))
	}

	log.Printf("found tool: %s %s", tool.Tag, tool.Executable)
	log.Printf("files: ['%s']", strings.Join(utils.Map(p.Inputs, func(file File) string { return file.Path }), "', '"))

	randomDirectory, err := os.MkdirTemp("", "all-build-*")
	log.Printf("created temporary directory: %s", randomDirectory)
	if err != nil {
		return respondError(t, fmt.Errorf("could not create temporary directory: %v", err))
	}

	for _, file := range p.Inputs {
		filePath := filepath.Join(randomDirectory, file.Path)

		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return respondError(t, fmt.Errorf("could not create directory: %v", err))
		}

		if err := os.WriteFile(filePath, file.Content, 0644); err != nil {
			return respondError(t, fmt.Errorf("could not write file: %v", err))
		}

		if err := os.Chmod(filePath, os.FileMode(file.Chmod)); err != nil {
			return respondError(t, fmt.Errorf("could not chmod file: %v", err))
		}
	}

	// Create output directories
	for _, output := range p.Outputs {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(randomDirectory, output)), 0755); err != nil {
			return respondError(t, fmt.Errorf("could not create directory: %v", err))
		}
	}

	// TODO: remap all files to the new path

	log.Printf("running command: %s %s", tool.Executable, p.Command)
	log.Printf("requested outputs: %v", p.Outputs)
	cmd := exec.Command(tool.Executable, p.Command...)
	cmd.Dir = randomDirectory
	//command.Env = append(os.Environ(), p.Environment...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return respondError(t, fmt.Errorf("could not get stderr pipe: %v", err))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return respondError(t, fmt.Errorf("could not get stdout pipe: %v", err))
	}

	if err := cmd.Start(); err != nil {
		return respondError(t, fmt.Errorf("could not start command: %v", err))
	}

	errout, _ := io.ReadAll(stderr)
	out, _ := io.ReadAll(stdout)

	log.Printf("stderr: %s", errout)
	log.Printf("stdout: %s", out)

	cmd.Wait()

	outFiles := make([]File, 0)
	for _, output := range p.Outputs {
		content, err := os.ReadFile(filepath.Join(randomDirectory, output))
		if err != nil {
			log.Printf("could not read output file: %v", err)
			continue
		}

		info, err := os.Stat(filepath.Join(randomDirectory, output))
		if err != nil {
			log.Printf("could not get file info: %v", err)
			continue
		}

		outFiles = append(outFiles, File{
			Path:    output,
			Content: content,
			Chmod:   int(info.Mode().Perm()),
		})
	}

	reponse := Response{
		ReturnCode: cmd.ProcessState.ExitCode(),
		Stdout:     string(out),
		Stderr:     string(errout),
		Files:      outFiles,
	}
	payload, err := json.Marshal(reponse)
	if err != nil {
		return err
	}

	t.ResultWriter().Write(payload)

	return nil
}
