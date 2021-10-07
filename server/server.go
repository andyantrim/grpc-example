package server

import (
	"net"

	"github.com/andyantrim/grpc-example/tasks"
	"github.com/teamwork/log"
	"google.golang.org/grpc"
)

func Start() {
	lis, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Error(err, "Failed to start")
	}
	grpcServer := grpc.NewServer()
	tasks.RegisterTasksServer(grpcServer, tasks.NewTaskService())

	log.Infof("Starting server on %v", lis.Addr())

	if err := grpcServer.Serve(lis); err != nil {
		log.Error(err, "Failed to start ")
	}
}
