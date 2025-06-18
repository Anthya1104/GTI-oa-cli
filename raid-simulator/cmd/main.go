package main

import (
	"os"

	"github.com/Anthya1104/raid-simulator/internal/cobra"
	"github.com/Anthya1104/raid-simulator/internal/config"
	"github.com/Anthya1104/raid-simulator/internal/logger"
	"github.com/Anthya1104/raid-simulator/internal/raid"
	"github.com/sirupsen/logrus"
)

func main() {

	if err := logger.InitLogger(config.LogLevelInfo); err != nil {
		logrus.Fatalf(("Error initializing Logger : %v"), err)
	}

	if err := cobra.ExecuteCmd(); err != nil {
		logrus.Fatalf("Error executing command: %v", err)
		os.Exit(1)
	}

	raid := raid.NewRAID0Controller(3, 4)
	input := []byte("HelloRAIDSystem12345678")
	raid.Write(input)
	logrus.Info("Write done")

	// read and parse the string
	output, err := raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("Read failed: %v", err)
	} else {
		logrus.Infof("Recovered string: %v", string(output))
	}

	// clear one of the disk
	raid.ClearDisk(1)
	logrus.Infof("Disk 1 cleared")

	// try to read and parse string againg
	output, err = raid.Read(0, len(input))
	if err != nil {
		logrus.Errorf("Read failed: %v", err)
	} else {
		logrus.Infof("Recovered string: %v", string(output))
	}

}
