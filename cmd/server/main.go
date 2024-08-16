package main

import (
	"log"

	"github.com/suifei/ocr-server/internal/config"
	"github.com/suifei/ocr-server/internal/server"
	"github.com/suifei/ocr-server/internal/utils"
)

func main() {
	cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

	utils.SetupLogger(cfg)

	log.Println("Starting OCR server...")

	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Initialize(); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	srv.Start()
}
