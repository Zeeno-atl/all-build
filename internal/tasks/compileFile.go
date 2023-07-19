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
	"regexp"

	"github.com/Zeeno-atl/all-build/internal/compiler"
	"github.com/Zeeno-atl/all-build/internal/utils"
	"github.com/hibiken/asynq"
)

type CompileFile struct {
	Tag         string   `json:"tag"`
	Command     []string `json:"command"`
	Files       []File   `json:"files"`
	Environment []string `json:"environment"`
}

func getIncludePaths(commands []string) []string {
	includes := utils.Filter(commands, func(cmd string) bool { return regexp.MustCompile(`^-I`).MatchString(cmd) })
	return utils.Map(includes, func(cmd string) string { return cmd[2:] })
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
	// TODO: add link folders

	for _, folder := range folders {
		filePaths = append(filePaths, walkDirectory(folder)...)
	}

	files := utils.Map(filePaths, func(path string) File {
		content, err := os.ReadFile(path)

		if err != nil {
			log.Fatalf("could not read file: %v", err)
		}

		//This is a debug line
		content = []byte{}

		return File{
			Path:    path,
			Content: content,
		}
	})

	cf := CompileFile{
		Tag:         tag,
		Command:     args,
		Files:       files,
		Environment: make([]string, 0),
	}

	payload, err := json.Marshal(cf)
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(TypeCompileFile, payload), nil
}

type CompileFileHandler struct {
	Tools []compiler.Tool
}

func NewCompileFileHandler(tools []compiler.Tool) *CompileFileHandler {
	return &CompileFileHandler{Tools: tools}
}

func (h CompileFileHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p CompileFile
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	tool, ok := utils.Find(h.Tools, func(tool compiler.Tool) bool { return tool.Tag == p.Tag })
	if !ok {
		return fmt.Errorf("could not find tool: %s", p.Tag)
	}

	// TODO: write files to disk
	// TODO: remap all files to the new path

	cmd := exec.Command(tool.Executable, p.Command...)
	//command.Env = append(os.Environ(), p.Environment...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not get stderr pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start command: %v", err)
	}

	errout, _ := io.ReadAll(stderr)
	out, _ := io.ReadAll(stdout)

	reponse := Response{
		ReturnCode: cmd.ProcessState.ExitCode(),
		Stdout:     string(out),
		Stderr:     string(errout),
		Files:      make([]File, 0),
	}
	payload, err := json.Marshal(reponse)
	if err != nil {
		return err
	}

	t.ResultWriter().Write(payload)

	cmd.Wait()
	return nil
}
