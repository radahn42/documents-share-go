package main

import (
	"document-server/internal/app"
	"document-server/internal/config"
	"document-server/pkg/logger"
	"log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	if err := logger.InitLogger(cfg.Env); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	defer logger.Sync()

	if err := app.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
