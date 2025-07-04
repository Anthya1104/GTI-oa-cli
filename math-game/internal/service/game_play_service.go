package service

import (
	"context"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Anthya1104/math-game-cli/internal/model"
	"github.com/sirupsen/logrus"
)

func StartGamePlay(maxRounds int) {

	if maxRounds <= 0 {
		logrus.Warnf("Invalid MaxRounds value '%d' from CLI. Defaulting to 5 rounds.", maxRounds)
		maxRounds = 1
	}

	// graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logrus.Infof("Received signal: %s. Initiating graceful shutdown...", sig)
		cancel()
	}()

	gameDone := make(chan struct{})
	go func() {
		RunGame(ctx, maxRounds)
		close(gameDone)
	}()

	select {
	case <-ctx.Done():
		logrus.Infof("Game play was interrupted, exiting the game.")
	case <-gameDone:
		logrus.Infof("All game rounds finished, exiting the game.")
	}

}

func RunGame(ctx context.Context, maxRounds int) {

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
		Students:        students,
		Teacher:         teacher,
		MaxRounds:       maxRounds,
		StudentActioner: &model.DefaultStudentActioner{},
	}

	gameDone := game.Start(ctx)

	select {
	case <-ctx.Done():
		logrus.Infof("The game play has been interrupted, exiting the game.")
	case <-gameDone:
		logrus.Infof("All game rounds finished, exiting the game.")
	}

}
