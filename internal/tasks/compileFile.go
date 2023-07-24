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
	"github.com/hibiken/asynq"
)

type CompileFile struct {
	Tag         string   `json:"tag"`
	Command     []string `json:"command"`
	Inputs      []File   `json:"inputs"`
	Outputs     []string `json:"outputs"`
	Environment []string `json:"environment"`
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
		if strings.HasPrefix(cmd, "-imsvc") || strings.HasPrefix(cmd, "/imsvc") {
			ret = append(ret, cmd[6:])
			continue
		}
	}

	return ret
}

func getInputFiles(commands []string) []string {
	// This works primarily for MSVC as no argument is passed without flag character (- or /) except input files
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

func walkDirectory(path string) []string {
	files := make([]string, 0)
	filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func NewCompileFile(args []string, tag string) (*asynq.Task, error) {
	filePaths := []string{}
	folders := getIncludePaths(args)
	folders = utils.Unique(folders)

	for _, folder := range folders {
		filePaths = append(filePaths, walkDirectory(folder)...)
	}

	filePaths = append(filePaths, getInputFiles(args)...)
	filePaths = utils.Unique(filePaths)

	files := utils.Map(filePaths, func(path string) File {
		content, err := os.ReadFile(path)

		if err != nil {
			log.Fatalf("could not read file: %v", err)
		}

		return File{
			Path:    path,
			Content: content,
		}
	})

	cf := CompileFile{
		Tag:         tag,
		Command:     args,
		Inputs:      files,
		Outputs:     getOutputFiles(args),
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
	log.Printf("files: %v", p.Inputs)

	randomDirectory, err := os.MkdirTemp("", "all-build-*")
	log.Printf("created temporary directory: %s", randomDirectory)
	if err != nil {
		return respondError(t, fmt.Errorf("could not create temporary directory: %v", err))
	}

	for _, file := range p.Inputs {
		if err := os.WriteFile(filepath.Join(randomDirectory, file.Path), file.Content, 0644); err != nil {
			return respondError(t, fmt.Errorf("could not write file: %v", err))
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

		outFiles = append(outFiles, File{
			Path:    output,
			Content: content,
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
