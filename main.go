package main

import (
	"log"

	"github.com/pulzeai-oss/knn-router/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalf("Failed to execute command: %v", err)
	}
}
