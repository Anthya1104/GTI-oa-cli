package model

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type RoundResult struct {
	Student *Student
	Answer  int
}

type Game struct {
	Students  []*Student
	Teacher   *Teacher
	Questions []*Question // TODO: refactor as chan to expand for bonus ii
	MaxRounds int

	// recourd the winner and answer
	Results []RoundResult
}

func (g *Game) Start(ctx context.Context) {
	logrus.Infof("Teacher: Guys, are you ready?")

	countdownSeconds := int(g.Teacher.WaitTime.Seconds())
	if countdownSeconds <= 0 {
		countdownSeconds = 3
	}

	for i := countdownSeconds; i > 0; i-- {
		select {
		case <-ctx.Done():
			logrus.Warnf("Game interrupted during countdown.")
			return
		case <-time.After(1 * time.Second):
			logrus.Debugf("# Count %d", i)
		}
	}

	for i := 1; i <= g.MaxRounds; i++ {
		select {
		case <-ctx.Done():
			logrus.Warnf("Game interrupted before round %d.", i)
			return
		default:
		}

		q, err := NewQuestion(i)
		if err != nil {
			logrus.Errorf("failed to generate question %d: %v", i, err)
			continue
		}
		g.Questions = append(g.Questions, q)
		g.PlayRound(ctx, q)
	}
}

func (g *Game) PlayRound(ctx context.Context, q *Question) {
	logrus.Infof("Teacher: %s", q)

	roundCtx, cancelRound := context.WithCancel(ctx)
	defer cancelRound() // cancel the context when the round ended

	answerCh := make(chan AnswerEvent, len(g.Students))
	var wg sync.WaitGroup // wait answers from all students

	for _, s := range g.Students {
		wg.Add(1) //add count for each students
		go func(student *Student) {
			defer wg.Done()
			AskStudent(roundCtx, student, q, answerCh)
		}(s)
	}

	correctAnswerFound := false
	answersReceived := 0
	totalStudents := len(g.Students)

	for answersReceived < totalStudents {
		select {
		case <-roundCtx.Done():
			logrus.Infof("Round %d interrupted: %v", q.ID, roundCtx.Err())
			goto EndRound
		case answerEvent := <-answerCh:
			answersReceived++

			logrus.Infof("%s: %d %s %d = %d!", answerEvent.Student.Name, q.ArgumentA, q.Operator, q.ArgumentB, answerEvent.Answer)

			if answerEvent.Answer == q.Answer {
				g.Results = append(g.Results, RoundResult{Student: answerEvent.Student, Answer: answerEvent.Answer})
				logrus.Infof("Teacher: %s, you're right!", answerEvent.Student.Name)
				correctAnswerFound = true
				cancelRound()
				goto EndRound
			} else {
				logrus.Infof("Teacher: %s, wrong answer! (%s)", answerEvent.Student.Name, q.String())
				if answersReceived == totalStudents {
					logrus.Infof("Teacher: Everyone was wrong this round for Q%d.", q.ID)
					goto EndRound
				}
			}
		case <-time.After(10 * time.Second): // time out for  round
			logrus.Warnf("Round %d timed out! No one answered correctly within the time limit.", q.ID)
			cancelRound()
			goto EndRound
		}
	}

EndRound:
	wg.Wait()
	logrus.Debugf("All student goroutines for round %d finished.", q.ID)
	if correctAnswerFound {
		for _, s := range g.Students {
			if s.Name != g.Results[len(g.Results)-1].Student.Name { // students who not be the winner
				logrus.Infof("%s: %s, you win!", s.Name, g.Results[len(g.Results)-1].Student.Name)
			}
		}
	}
	logrus.Infof("--- Round %d Ends ---", q.ID)
}

// TODO: could do interface extracting if multiple student types are required
var AskStudent = func(ctx context.Context, s *Student, q *Question, ch chan AnswerEvent) {
	// students' thinking time
	select {
	case <-ctx.Done():
		logrus.Debugf("%s's goroutine for Q%d cancelled.", s.Name, q.ID)
		return
	case <-time.After(s.WaitTime):
	}

	// random answer with 70% correct rate
	answer := q.Answer
	if rand.Float32() < 0.3 {
		wrongAnswer := q.Answer + rand.Intn(10) - 5
		if wrongAnswer == q.Answer {
			wrongAnswer++
		}
		answer = wrongAnswer
	}

	select {
	case <-ctx.Done():
		logrus.Debugf("%s's goroutine for Q%d cancelled before sending answer.", s.Name, q.ID)
		return
	case ch <- AnswerEvent{
		Student: s,
		Answer:  answer,
		QID:     q.ID,
		Time:    time.Now(),
	}:
		logrus.Debugf("%s sent answer %d for Q%d.", s.Name, answer, q.ID)
	}
}
