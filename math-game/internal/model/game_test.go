package model_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/Anthya1104/math-game-cli/internal/model"
	"github.com/stretchr/testify/assert"
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

func TestGamePlay_MultipleRoundFlow(t *testing.T) {

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
		MaxRounds: 3,
	}
	game.Start()

	if len(game.Questions) != 3 {
		t.Errorf("expected 3 question, got %d", len(game.Questions))
	}
}

func TestPlayRoundFirstAnswerCorrect(t *testing.T) {
	s1 := model.NewStudent("A", 1)
	s2 := model.NewStudent("B", 2)

	// set s1 as the quick one
	s1.WaitTime = 1 * time.Millisecond
	s2.WaitTime = 3 * time.Second

	game := &model.Game{
		Students: []*model.Student{s1, s2},
		Teacher:  model.NewTeacher("T"),
	}

	q := &model.Question{ID: 1, Answer: 123}

	// replace AskStudent to let s1 with correct answer
	oldAskStudent := model.AskStudent
	defer func() { model.AskStudent = oldAskStudent }()
	model.AskStudent = func(s *model.Student, q *model.Question, ch chan model.AnswerEvent) {
		time.Sleep(s.WaitTime)
		ch <- model.AnswerEvent{Student: s, Answer: q.Answer, QID: q.ID}
	}

	game.PlayRound(q)

	assert.Equal(t, "A", game.Results[0].Student.Name)
	assert.Equal(t, 123, game.Results[0].Answer)
}
