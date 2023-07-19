package main

import (
	"flag"

	"github.com/Zeeno-atl/all-build/internal/tasks"
	"github.com/golang/glog"
	"github.com/hibiken/asynq"
)

const (
	Version = "0.0.1"
)

func main() {
	// GLOG: INFO, WARNING, ERROR, FATAL
	// V 0: INFO, WARNING, ERROR, FATAL
	// V 1: Logging incomming requests, showing Configuration
	// V 2: Logging content of requests
	// V 3: Tracing information

	flag.Parse()

	// rewrite glog parameter
	flag.Lookup("alsologtostderr").Value.Set("true")

	glog.V(1).Infof("buildall executor version %s", Version)

	config, err := LoadConfig()
	if err != nil {
		glog.Fatalln("Error loading configuration:", err)
	}
	glog.V(1).Infof("Configuration: %+v", config)

	queues := map[string]int{}
	for i, queue := range config.Tools {
		queues[queue.Tag] = len(config.Tools) - i
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: *config.TaskDatabase},
		asynq.Config{
			// Specify how many concurrent workers to use
			Concurrency: *config.Concurrency,
			// Optionally specify multiple queues with different priority.
			Queues: queues,
			// See the godoc for other configuration options
		},
	)

	// mux maps a type to a handler
	mux := asynq.NewServeMux()
	mux.Handle(tasks.TypeCompileFile, tasks.NewCompileFileHandler(config.Tools))

	if err := srv.Run(mux); err != nil {
		glog.Fatalf("could not run server: %v", err)
	}

	glog.Info("Exiting")
	glog.Flush()
}
