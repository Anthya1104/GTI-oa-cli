package model_test

import (
	"testing"
	"time"

	"github.com/Anthya1104/math-game-cli/internal/model"
	"github.com/stretchr/testify/assert"
)

// to prepare test question data
func NewQuestionWith(op model.Operator, a, b, id int) (*model.Question, error) {
	ans, err := op.Apply(a, b)
	if err != nil {
		return nil, err
	}
	return &model.Question{
		ID:        id,
		ArgumentA: a,
		ArgumentB: b,
		Operator:  op,
		Answer:    ans,
		Created:   time.Now(),
	}, nil
}

func TestNewQuestionWithAdd(t *testing.T) {
	q, err := NewQuestionWith("+", 5, 3, 1)
	assert.NoError(t, err)
	assert.Equal(t, 8, q.Answer)
	assert.Equal(t, "+", string(q.Operator))
}

func TestNewQuestionWithSubtract(t *testing.T) {
	q, err := NewQuestionWith("-", 9, 4, 2)
	assert.NoError(t, err)
	assert.Equal(t, 5, q.Answer)
	assert.Equal(t, "-", string(q.Operator))
}

func TestNewQuestionWithMultiply(t *testing.T) {
	q, err := NewQuestionWith("*", 7, 6, 3)
	assert.NoError(t, err)
	assert.Equal(t, 42, q.Answer)
	assert.Equal(t, "*", string(q.Operator))
}

func TestNewQuestionWithDivideValid(t *testing.T) {
	q, err := NewQuestionWith("/", 20, 5, 4)
	assert.NoError(t, err)
	assert.Equal(t, 4, q.Answer)
	assert.Equal(t, "/", string(q.Operator))
}

func TestNewQuestionWithDivideByZero(t *testing.T) {
	q, err := NewQuestionWith("/", 10, 0, 5)
	assert.Error(t, err)
	assert.Nil(t, q)
}

func TestNewQuestionWithInvalidOperator(t *testing.T) {
	q, err := NewQuestionWith("%", 10, 2, 6)
	assert.Error(t, err)
	assert.Nil(t, q)
}
