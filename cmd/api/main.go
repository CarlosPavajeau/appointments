package main

import (
	"context"
	"log/slog"
	"os"
	"wappiz/pkg/logger"
	"wappiz/svc/api"
)

func main() {
	logger.AddHandler(slog.NewJSONHandler(os.Stdout, nil))

	cfg := api.LoadConfiguration()
	err := api.Run(context.Background(), cfg)

	if err != nil {
		logger.Error("failed to run API",
			"err", err)
		os.Exit(1)
	}
}
