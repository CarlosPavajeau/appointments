package main

import (
	"os"
	"wappiz/pkg/logger"
	"wappiz/svc/api"
)

func main() {
	cfg := api.LoadConfiguration()
	err := api.Run(cfg)

	if err != nil {
		logger.Error("failed to run API: %v", err)
		os.Exit(1)
	}
}
