package model

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MockStudentActioner struct {
	AskStudentFunc func(ctx context.Context, s *Student, q *Question, ch chan AnswerEvent) // Removed model. prefix
}

func (m *MockStudentActioner) AskStudent(ctx context.Context, s *Student, q *Question, ch chan AnswerEvent) { // Removed model. prefix
	if m.AskStudentFunc != nil {
		m.AskStudentFunc(ctx, s, q, ch)
	} else {
		// Default mock behavior if AskStudentFunc is not set: always correct answer after wait.
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.WaitTime):
			ch <- AnswerEvent{Student: s, Answer: q.Answer, QID: q.ID}
		}
	}
}

func TestGamePlay_MultipleRoundFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	students := []*Student{NewStudent("A", 1), NewStudent("B", 2)}
	teacher := NewTeacher("Teacher")
	teacher.WaitTime = 1 * time.Millisecond

	game := Game{
		Students:        students,
		Teacher:         teacher,
		MaxRounds:       3,
		StudentActioner: &DefaultStudentActioner{},
	}
	game.Start(ctx)

	// Wait for all 3 results to be collected.
	assert.Eventually(t, func() bool {
		game.resultsMu.Lock()
		defer game.resultsMu.Unlock()
		return len(game.Results) == 3
	}, 6*time.Second, 10*time.Millisecond, "Expected 3 results to be collected")
}

func TestPlayQuestion_FirstAnswerCorrect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s1 := NewStudent("QuickStudent", 1)
	s2 := NewStudent("SlowStudent", 2)
	s1.WaitTime = 1 * time.Millisecond
	s2.WaitTime = 100 * time.Millisecond

	game := &Game{
		Students:        []*Student{s1, s2},
		Teacher:         NewTeacher("T"),
		MaxRounds:       1,
		StudentActioner: &MockStudentActioner{},
	}

	mockActioner := game.StudentActioner.(*MockStudentActioner)
	mockActioner.AskStudentFunc = func(mockCtx context.Context, s *Student, q *Question, ch chan AnswerEvent) {
		select {
		case <-mockCtx.Done():
			return
		case <-time.After(s.WaitTime):
		}
		ch <- AnswerEvent{Student: s, Answer: q.Answer, QID: q.ID}
	}

	game.Start(ctx)

	// Wait for exactly 1 result to be collected (the winner of Q1)
	assert.Eventually(t, func() bool {
		game.resultsMu.Lock() // Now accessing unexported resultsMu
		defer game.resultsMu.Unlock()
		return len(game.Results) == 1
	}, 2*time.Second, 10*time.Millisecond, "Expected exactly one result to be collected for the winning round")

	game.resultsMu.Lock()
	collectedResult := game.Results[0]
	game.resultsMu.Unlock()

	assert.Equal(t, "QuickStudent", collectedResult.Student.Name, "Winner should be QuickStudent")
	assert.True(t, collectedResult.IsCorrect, "Result should indicate a correct answer")
}

func TestPlayQuestion_FirstWrongThenCorrect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s1 := NewStudent("QuickButWrong", 1)
	s2 := NewStudent("SlowButCorrect", 2)
	s1.WaitTime = 1 * time.Millisecond
	s2.WaitTime = 5 * time.Millisecond

	game := &Game{
		Students:        []*Student{s1, s2},
		Teacher:         NewTeacher("T"),
		MaxRounds:       1,
		StudentActioner: &MockStudentActioner{},
	}

	mockActioner := game.StudentActioner.(*MockStudentActioner)
	mockActioner.AskStudentFunc = func(mockCtx context.Context, s *Student, q *Question, ch chan AnswerEvent) {
		select {
		case <-mockCtx.Done():
			return
		case <-time.After(s.WaitTime):
		}
		answer := -1
		if s.Name == "SlowButCorrect" {
			answer = q.Answer // The correct student gives the right answer
		}
		ch <- AnswerEvent{Student: s, Answer: answer, QID: q.ID}
	}

	game.Start(ctx) // Start the game

	// Wait for the single result to be collected (the winner of Q1)
	assert.Eventually(t, func() bool {
		game.resultsMu.Lock() // Now accessing unexported resultsMu
		defer game.resultsMu.Unlock()
		return len(game.Results) == 1
	}, 2*time.Second, 10*time.Millisecond, "Expected exactly one result to be collected for the winning round")

	game.resultsMu.Lock()
	collectedResult := game.Results[0]
	game.resultsMu.Unlock()

	assert.Equal(t, "SlowButCorrect", collectedResult.Student.Name, "Winner should be SlowButCorrect")
	assert.True(t, collectedResult.IsCorrect, "Result should indicate a correct answer")
}

func TestPlayQuestion_AllWrong(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	students := []*Student{
		NewStudent("A", 1), NewStudent("B", 2), NewStudent("C", 3),
	}
	for _, s := range students {
		s.WaitTime = time.Duration(rand.Intn(5)+1) * time.Millisecond
	}

	game := &Game{
		Students:        students,
		Teacher:         NewTeacher("T"),
		MaxRounds:       1,
		StudentActioner: &MockStudentActioner{},
	}

	mockActioner := game.StudentActioner.(*MockStudentActioner)
	mockActioner.AskStudentFunc = func(mockCtx context.Context, s *Student, q *Question, ch chan AnswerEvent) {
		select {
		case <-mockCtx.Done():
			return
		case <-time.After(s.WaitTime):
		}
		ch <- AnswerEvent{Student: s, Answer: q.Answer + 1, QID: q.ID}
	}

	game.Start(ctx)

	assert.Eventually(t, func() bool {
		game.resultsMu.Lock()
		defer game.resultsMu.Unlock()
		return len(game.Results) == 1
	}, 2*time.Second, 10*time.Millisecond, "Expected one result to be collected (indicating round outcome)")

	game.resultsMu.Lock()
	collectedResult := game.Results[0]
	game.resultsMu.Unlock()

	assert.False(t, collectedResult.IsCorrect, "Result should indicate an incorrect answer (no winner)")
	assert.Nil(t, collectedResult.Student, "No student should be recorded as winner if all are wrong")
}
