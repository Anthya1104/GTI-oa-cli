package model

import (
	"math/rand"
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
		g.PlayRound(q)
	}
}

func (g *Game) PlayRound(q *Question) {
	logrus.Infof("Teacher: %s", q)

	answerCh := make(chan AnswerEvent, len(g.Students))
	for _, s := range g.Students {
		go AskStudent(s, q, answerCh)
	}

	attemptedStudents := make(map[string]bool)
	var winner *Student

	// Loop to process answers until a winner is found or everyone has tried
	for len(attemptedStudents) < len(g.Students) {
		answer := <-answerCh

		// Skip if this student has already attempted in this round
		if _, ok := attemptedStudents[answer.Student.Name]; ok {
			continue
		}

		attemptedStudents[answer.Student.Name] = true
		logrus.Infof("%s: %d %s %d = %d!", answer.Student.Name, q.ArgumentA, q.Operator, q.ArgumentB, answer.Answer)

		// Check if the answer is correct
		if answer.Answer == q.Answer {
			logrus.Infof("Teacher: %s, you're right!", answer.Student.Name)
			winner = answer.Student
			g.Results = append(g.Results, RoundResult{Student: winner, Answer: answer.Answer})
			break
		} else {
			logrus.Infof("Teacher: %s, you are wrong.", answer.Student.Name)
		}
	}

	// After the loop, check if we found a winner
	if winner != nil {
		for _, s := range g.Students {
			if s.Name != winner.Name {
				logrus.Infof("%s: %s, you win.", s.Name, winner.Name)
			}
		}
	} else {
		logrus.Infof("Teacher: Boooo~ Answer is %d.", q.Answer)
	}
}

// TODO: could do interface extracting if multiple student types are required
var AskStudent = func(s *Student, q *Question, ch chan AnswerEvent) {
	time.Sleep(s.WaitTime)

	studentAnswer := q.Answer
	//50% chance of getting the wrong answer
	if rand.Intn(2) == 0 { // 0 represents a wrong answer
		// Generate a random wrong answer that is not the correct one
		for {
			wrongAnswer := q.Answer + (rand.Intn(10) - 5) // a random number near the answer
			if wrongAnswer != q.Answer {
				studentAnswer = wrongAnswer
				break
			}
		}
	}

	ch <- AnswerEvent{
		Student: s,
		Answer:  studentAnswer,
		QID:     q.ID,
		Time:    time.Now(),
	}
}
