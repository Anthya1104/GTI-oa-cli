package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/Anthya1104/math-game-cli/internal/cobra"
	"github.com/Anthya1104/math-game-cli/internal/config"
	"github.com/Anthya1104/math-game-cli/internal/logger"
	"github.com/Anthya1104/math-game-cli/internal/model"
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

	students := []*model.Student{
		model.NewStudent("A", 1),
		model.NewStudent("B", 2),
		model.NewStudent("C", 3),
		model.NewStudent("D", 4),
		model.NewStudent("E", 5),
	}

	teacher := model.NewTeacher("Teacher")

	// init thinking time for students and teacher
	teacher.WaitTime = time.Second * 3
	for _, s := range students {
		s.WaitTime = time.Duration(rand.Intn(3)+1) * time.Second
	}

	game := model.Game{
		Students:  students,
		Teacher:   teacher,
		MaxRounds: 1,
	}
	game.Start()

}
