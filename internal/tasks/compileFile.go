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
	"github.com/golang/glog"
	"github.com/hibiken/asynq"
)

type CompileFile struct {
	Tag         string   `json:"tag"`
	Command     []string `json:"command"`
	Inputs      []File   `json:"inputs"`
	Outputs     []string `json:"outputs"`
	Environment []string `json:"environment"`
	Compiler    string   `json:"compiler"`
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
	compilerInstance := compiler.NewCompiler(compilerType)

	if compilerInstance == nil {
		return nil, fmt.Errorf("unknown compiler: %s", compilerType)
	}

	compilerInstance.Parse(args)

	inputs := compiler.GetInputs(compilerInstance)
	inputs = utils.Unique(inputs)

	filePaths := make([]string, 0)

	for _, input := range inputs {
		baseDir := filepath.Dir(input)
		filePaths = append(filePaths, walkFilesystem(baseDir)...)
	}
	filePaths = utils.Unique(filePaths)

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

	outputs := compiler.GetOutputs(compilerInstance)

	cf := CompileFile{
		Tag:         tag,
		Command:     args,
		Inputs:      inputFiles,
		Outputs:     outputs,
		Environment: make([]string, 0),
		Compiler:    compilerType,
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
	glog.Errorf("error: %v", err)
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
		return fmt.Errorf("%s: json.Unmarshal failed: %v: %w", t.ResultWriter().TaskID(), err, asynq.SkipRetry)
	}

	tool, ok := utils.Find(h.Tools, func(tool executor.Tool) bool { return tool.Tag == p.Tag })
	if !ok {
		return respondError(t, fmt.Errorf("%s could not find tool: %s", t.ResultWriter().TaskID(), p.Tag))
	}

	glog.Infof("%s: incomming request in queue '%s' for '%s' ['%s'] with %d packed files",
		t.ResultWriter().TaskID(),
		tool.Tag,
		tool.Executable,
		strings.Join(p.Command, "', '"),
		len(p.Inputs))

	glog.V(2).Infof("%s, files: ['%s']", t.ResultWriter().TaskID(), strings.Join(utils.Map(p.Inputs, func(file File) string { return file.Path }), "', '"))

	randomDirectory, err := os.MkdirTemp("", "all-build-*")
	glog.V(3).Infof("%s: created temporary directory: %s", t.ResultWriter().TaskID(), randomDirectory)
	if err != nil {
		return respondError(t, fmt.Errorf("%s: could not create temporary directory: %v", t.ResultWriter().TaskID(), err))
	}

	compilerInstance := compiler.NewCompiler(p.Compiler)
	if compilerInstance == nil {
		return respondError(t, fmt.Errorf("%s: unknown compiler: %s", t.ResultWriter().TaskID(), p.Compiler))
	}

	compilerInstance.Parse(p.Command)

	glog.V(3).Infof("%s: command before remapping: %v", t.ResultWriter().TaskID(), compiler.GetCommand(compilerInstance))
	compilerInstance.Chroot(randomDirectory)
	glog.V(3).Infof("%s: command after remapping: %v", t.ResultWriter().TaskID(), compiler.GetCommand(compilerInstance))

	for _, file := range p.Inputs {
		filePath := filepath.Join(randomDirectory, file.Path)

		glog.V(3).Infof("%s: creating directory: %s", t.ResultWriter().TaskID(), filepath.Dir(filePath))
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return respondError(t, fmt.Errorf("%s: could not create directory: %v", t.ResultWriter().TaskID(), err))
		}

		glog.V(3).Infof("%s: writing file: %s", t.ResultWriter().TaskID(), filePath)
		if err := os.WriteFile(filePath, file.Content, 0644); err != nil {
			return respondError(t, fmt.Errorf("%s: could not write file: %v", t.ResultWriter().TaskID(), err))
		}

		glog.V(3).Infof("%s: chmod file: %s", t.ResultWriter().TaskID(), filePath)
		if err := os.Chmod(filePath, os.FileMode(file.Chmod)); err != nil {
			return respondError(t, fmt.Errorf("%s: could not chmod file: %v", t.ResultWriter().TaskID(), err))
		}
	}

	// Create output directories
	for _, output := range p.Outputs {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(randomDirectory, output)), 0755); err != nil {
			return respondError(t, fmt.Errorf("%s: could not create directory: %v", t.ResultWriter().TaskID(), err))
		}
	}

	glog.V(2).Infof("%s: running command: %s ['%s']", t.ResultWriter().TaskID(), tool.Executable, strings.Join(compiler.GetCommand(compilerInstance), "', '"))
	glog.V(2).Infof("%s: requested outputs: %v", t.ResultWriter().TaskID(), p.Outputs)
	cmd := exec.Command(tool.Executable, compiler.GetCommand(compilerInstance)...)
	cmd.Dir = randomDirectory
	//command.Env = append(os.Environ(), p.Environment...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return respondError(t, fmt.Errorf("%s: could not get stderr pipe: %v", t.ResultWriter().TaskID(), err))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return respondError(t, fmt.Errorf("%s: could not get stdout pipe: %v", t.ResultWriter().TaskID(), err))
	}

	if err := cmd.Start(); err != nil {
		return respondError(t, fmt.Errorf("%s: could not start command: %v", t.ResultWriter().TaskID(), err))
	}

	errout, _ := io.ReadAll(stderr)
	out, _ := io.ReadAll(stdout)

	glog.V(1).Infof("%s: stderr: %s", t.ResultWriter().TaskID(), errout)
	glog.V(1).Infof("%s: stdout: %s", t.ResultWriter().TaskID(), out)

	cmd.Wait()

	fsContent := walkFilesystem(cmd.Dir)
	glog.V(3).Infof("%s: filesystem content: ['%s']", t.ResultWriter().TaskID(), strings.Join(fsContent, "', '"))

	outFiles := make([]File, 0)
	for _, output := range p.Outputs {
		content, err := os.ReadFile(filepath.Join(randomDirectory, output))
		if err != nil {
			glog.Warningf("%s: could not read output file: %v", t.ResultWriter().TaskID(), err)
			continue
		}

		info, err := os.Stat(filepath.Join(randomDirectory, output))
		if err != nil {
			glog.Warningf("%s: could not get file info: %v", t.ResultWriter().TaskID(), err)
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
