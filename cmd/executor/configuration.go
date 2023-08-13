package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/Zeeno-atl/all-build/internal/executor"
	"github.com/golang/glog"
	"gopkg.in/yaml.v3"
)

type Config struct {
	TaskDatabase *string         `yaml:"task-database"`
	Concurrency  *int            `yaml:"concurrency"`
	Tools        []executor.Tool `yaml:"tools"`
}

func toType[T any](value string) T {
	var result T
	fmt.Sscanf(value, "%v", &result)
	return result
}

// TODO: There should be a library for getting a configuration and you do not care
// where it comes from. Found several projects being able to have multiple sources,
// but they could not be combined and prioritized.
// I will do it later, but for now, this is enough.
func loadValue[T any](config **T, name string, description string, value T) {
	//priority: command line > environment variable > config file > default value

	cmd := flag.Lookup(name) //TODO: make it commandliney name
	if cmd != nil {
		cmd.Usage = fmt.Sprintf("%s (default: %v)", description, value)

		if cmd.Value.String() != "" {
			*config = new(T)
			**config = toType[T](cmd.Value.String())
			return
		}
	}

	// replace dashes with underscores and make it uppercase
	name = strings.Replace(name, "-", "_", -1)
	name = strings.ToUpper(name)

	env := os.Getenv("ALLBUILD_" + name)
	if env != "" {
		*config = new(T)
		**config = toType[T](env)
		return
	}

	if *config == nil {
		*config = new(T)
		**config = value
	}
}

func LoadConfig() (Config, error) {
	var config Config

	filenames := []string{"executor.yaml", "configs/executor.yaml", "/etc/all-build/executor.yaml"}

	for _, filename := range filenames {
		// Read the YAML configuration file
		data, err := os.ReadFile(filename)
		if err == nil {
			glog.V(1).Infof("Loading configuration from %s", filename)
			err = yaml.Unmarshal(data, &config)
			if err != nil {
				return config, fmt.Errorf("failed to parse YAML data: %v", err)
			}
			break
		}
	}

	loadValue(&config.TaskDatabase, "task-database", "Task database", "127.0.0.1:6379")
	loadValue(&config.Concurrency, "concurrency", "Concurrency", runtime.NumCPU())

	return config, nil
}
