package client

import (
	"context"

	"github.com/andyantrim/grpc-example/tasks"
	"github.com/teamwork/log"
	"google.golang.org/grpc"
)

func RunClient() {
	// Connect GRPC dial
	conn, err := grpc.Dial(":9000", grpc.WithInsecure())
	if err != nil {
		log.Error(err, "Failed to connect to local server")
		return
	}
	defer conn.Close()

	client := tasks.NewTasksClient(conn)
	taskToCreate := tasks.TaskRequest{
		Title:       "Eho",
		Description: "This is a lengthy description",
	}
	resp, err := client.Create(context.Background(), &taskToCreate)
	if err != nil {
		log.Error(err, "failed to create task :(")
		return
	}

	log.Infof("Created task with id %d", resp.Id)
}
