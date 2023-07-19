package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Zeeno-atl/all-build/internal/tasks"
	"github.com/hibiken/asynq"
)

const (
	Version = "0.0.1"
)

func main() {
	log.Printf("buildall compiler version %s", Version)
	fmt.Println(len(os.Args), os.Args)

	config, err := LoadConfig()
	if err != nil {
		log.Fatalln("Error loading configuration:", err)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: *config.TaskDatabase})
	defer client.Close()

	task, err := tasks.NewCompileFile(os.Args[1:], config.Tag)
	if err != nil {
		log.Fatalf("could not create task: %v", err)
	}

	info, err := client.Enqueue(task, asynq.Queue(config.Tag), asynq.Retention(time.Minute*2))
	if err != nil {
		log.Fatalf("could not enqueue task: %v", err)
	}

	// loop until the task is finished
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: *config.TaskDatabase})

	for {
		status, err := inspector.GetTaskInfo(info.Queue, info.ID)
		if err != nil {
			log.Fatalf("could not get task status: %v", err)
			break
		}
		if status.State == asynq.TaskStateCompleted {
			log.Printf("task (%s) succeeded with output: %q", info.ID, status.Result)
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

}
