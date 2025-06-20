package model

import "context"

type StudentActioner interface {
	AskStudent(ctx context.Context, s *Student, q *Question, ch chan AnswerEvent)
}
