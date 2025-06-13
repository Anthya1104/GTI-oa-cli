package main

import (
	"os"

	"github.com/Anthya1104/raid-simulator/internal/cobra"
	"github.com/Anthya1104/raid-simulator/internal/config"
	"github.com/Anthya1104/raid-simulator/internal/logger"
	"github.com/sirupsen/logrus"
)

func main() {

	if err := logger.InitLogger(config.LogLevelInfo); err != nil {
		logrus.Fatalf(("Error initializing Logger : %v"), err)
	}

	if err := cobra.InitCmd(); err != nil {
		logrus.Fatalf("Error executing command: %v", err)
		os.Exit(1)
	}

}
