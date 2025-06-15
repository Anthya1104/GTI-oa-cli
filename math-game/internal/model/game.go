package model

import (
	"time"

	"github.com/sirupsen/logrus"
)

type Game struct {
	Students  []*Student
	Teacher   *Teacher
	Questions []*Question
	MaxRounds int
}

func (g *Game) Start() {
	logrus.Infof("Teacher: Guys, are you ready?")
	time.Sleep(g.Teacher.WaitTime)

	for i := 1; i <= g.MaxRounds; i++ {
		q, err := NewQuestion(i)
		if err != nil {
			logrus.Errorf("failed to generate question %d: %v", i, err)
			continue
		}
		g.Questions = append(g.Questions, q)
		g.playRound(q)
	}
}

func (g *Game) playRound(q *Question) {
	logrus.Infof("Teacher: %s", q)

	answerCh := make(chan AnswerEvent, len(g.Students))

	for _, s := range g.Students {
		go askStudent(s, q, answerCh)
	}

	// wait for the first student to answer
	first := <-answerCh

	logrus.Infof("%s answered %d", first.Student.Name, first.Answer)

	if first.Answer == q.Answer {
		logrus.Infof("Teacher: %s, you're right!", first.Student.Name)
	} else {
		logrus.Infof("Teacher: %s, wrong answer!", first.Student.Name)
	}

	// interaction from other students
	for _, s := range g.Students {
		if s.Name != first.Student.Name {
			logrus.Infof("%s: %s, you win", s.Name, first.Student.Name)
		}
	}
}

func askStudent(s *Student, q *Question, ch chan AnswerEvent) {
	time.Sleep(s.WaitTime)

	ch <- AnswerEvent{
		Student: s,
		Answer:  q.Answer, // would always as right answer in this state
		QID:     q.ID,
		Time:    time.Now(),
	}
}
