package model

import (
	"fmt"
	"math/rand"
	"time"
)

// math operator
type Operator string

var Operators = []Operator{"+", "-", "*", "/"}

func (o Operator) Apply(a, b int) (int, error) {
	switch o {
	case "+":
		return a + b, nil
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	case "/":
		if b == 0 {
			return 0, fmt.Errorf("divide by zero")
		}
		return a / b, nil
	default:
		return 0, fmt.Errorf("unknown operator: %s", o)
	}
}

// Question structure
type Question struct {
	ID        int
	ArgumentA int
	ArgumentB int
	Operator  Operator
	Answer    int
	Created   time.Time
}

func (q Question) String() string {
	return fmt.Sprintf("Q%d: %d %s %d = ?", q.ID, q.ArgumentA, q.Operator, q.ArgumentB)
}

func NewQuestion(id int) (*Question, error) {
	a := rand.Intn(101)
	b := rand.Intn(101)
	op := Operators[rand.Intn(len(Operators))]

	ans, err := op.Apply(a, b)
	if err != nil {
		return nil, err
	}

	return &Question{
		ID:        id,
		ArgumentA: a,
		ArgumentB: b,
		Operator:  op,
		Answer:    ans,
		Created:   time.Now(),
	}, nil
}

type AnswerEvent struct {
	Student *Student
	Answer  int
	QID     int // question ids
	Time    time.Time
}
