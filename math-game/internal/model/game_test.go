package model_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/Anthya1104/math-game-cli/internal/model"
)

func TestGamePlay_SimpleFlow(t *testing.T) {

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

	if len(game.Questions) != 1 {
		t.Errorf("expected 1 question, got %d", len(game.Questions))
	}
}
