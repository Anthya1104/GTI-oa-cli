package model_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/Anthya1104/math-game-cli/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestGamePlay_SimpleFlow(t *testing.T) {
	ctx := context.Background()

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
	game.Start(ctx)

	if len(game.Questions) != 1 {
		t.Errorf("expected 1 question, got %d", len(game.Questions))
	}
}

// This is more of an integration test. It's good for checking the overall flow.
func TestGamePlay_MultipleRoundFlow(t *testing.T) {
	ctx := context.Background()

	students := []*model.Student{
		model.NewStudent("A", 1),
		model.NewStudent("B", 2),
	}
	game := model.Game{
		Students:  students,
		Teacher:   model.NewTeacher("Teacher"),
		MaxRounds: 3,
	}
	game.Start(ctx)

	// We can assert the number of rounds played
	assert.Equal(t, 3, len(game.Questions), "Should have generated 3 questions for 3 rounds")
}

// This unit test focuses on the PlayRound logic with a simple correct answer.
func TestPlayRoundFirstAnswerCorrect(t *testing.T) {
	ctx := context.Background()

	s1 := model.NewStudent("QuickStudent", 1)
	s2 := model.NewStudent("SlowStudent", 2)
	s1.WaitTime = 1 * time.Millisecond // s1 is faster
	s2.WaitTime = 1 * time.Second

	game := &model.Game{
		Students: []*model.Student{s1, s2},
		Teacher:  model.NewTeacher("T"),
	}
	q := &model.Question{ID: 1, Answer: 123}

	oldAskStudent := model.AskStudent
	defer func() { model.AskStudent = oldAskStudent }()
	model.AskStudent = func(mockCtx context.Context, s *model.Student, q *model.Question, ch chan model.AnswerEvent) {
		select {
		case <-mockCtx.Done():
			return
		case <-time.After(s.WaitTime):
		}
		ch <- model.AnswerEvent{Student: s, Answer: q.Answer, QID: q.ID}
	}

	game.PlayRound(ctx, q)

	// Assert that the winner is the first (and faster) student
	assert.Len(t, game.Results, 1, "There should be exactly one winner")
	assert.Equal(t, "QuickStudent", game.Results[0].Student.Name)
	assert.Equal(t, 123, game.Results[0].Answer)
}

func TestPlayRound_FirstWrongThenCorrect(t *testing.T) {
	ctx := context.Background()

	s1 := model.NewStudent("QuickButWrong", 1)
	s2 := model.NewStudent("SlowButCorrect", 2)
	s1.WaitTime = 1 * time.Millisecond // s1 answers first
	s2.WaitTime = 5 * time.Millisecond // s2 answers second

	game := &model.Game{
		Students: []*model.Student{s1, s2},
		Teacher:  model.NewTeacher("T"),
	}
	q := &model.Question{ID: 1, Answer: 100}

	oldAskStudent := model.AskStudent
	defer func() { model.AskStudent = oldAskStudent }()
	model.AskStudent = func(mockCtx context.Context, s *model.Student, q *model.Question, ch chan model.AnswerEvent) {
		select {
		case <-mockCtx.Done():
			return
		case <-time.After(s.WaitTime):
		}
		answer := -1 // Default wrong answer
		if s.Name == "SlowButCorrect" {
			answer = q.Answer // The correct student gives the right answer
		}
		ch <- model.AnswerEvent{Student: s, Answer: answer, QID: q.ID}
	}
	game.PlayRound(ctx, q)

	// Assert that the winner is the second student who answered correctly
	assert.Len(t, game.Results, 1, "There should be exactly one winner")
	assert.Equal(t, "SlowButCorrect", game.Results[0].Student.Name)
	assert.Equal(t, 100, game.Results[0].Answer)
}

func TestPlayRound_AllWrong(t *testing.T) {
	ctx := context.Background()

	students := []*model.Student{
		model.NewStudent("A", 1),
		model.NewStudent("B", 2),
		model.NewStudent("C", 3),
	}
	// Give them random wait times
	for _, s := range students {
		s.WaitTime = time.Duration(rand.Intn(5)+1) * time.Millisecond
	}

	game := &model.Game{
		Students: students,
		Teacher:  model.NewTeacher("T"),
	}
	q := &model.Question{ID: 1, Answer: 100}

	oldAskStudent := model.AskStudent
	defer func() { model.AskStudent = oldAskStudent }()
	model.AskStudent = func(mockCtx context.Context, s *model.Student, q *model.Question, ch chan model.AnswerEvent) {
		select {
		case <-mockCtx.Done():
			return
		case <-time.After(s.WaitTime):
		}
		// Send a wrong answer that is guaranteed not to be the correct one
		ch <- model.AnswerEvent{Student: s, Answer: q.Answer + 1, QID: q.ID}
	}

	game.PlayRound(ctx, q)

	// Assert that there are no winners recorded in the results
	assert.Len(t, game.Results, 0, "There should be no winners if everyone is wrong")
}
