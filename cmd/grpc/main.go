package main

import (
	"flag"

	"github.com/andyantrim/grpc-example/client"
	"github.com/andyantrim/grpc-example/server"
	"github.com/teamwork/log"
)

var serverMode = "server"
var clientMode = "client"

func main() {
	// Parse the flag to get running mode
	mode := serverMode
	flag.StringVar(&mode, "mode", serverMode, "[server|client]")

	switch mode {
	case clientMode:
		log.Info("Running client connection")
		client.RunClient()
	case serverMode:
		log.Info("Running server")
		server.Start()
	default:
		log.Infof("No mode %s, running server as default", mode)
		log.Info("Running server")
		server.Start()
	}
}
