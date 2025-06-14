package model

type Game struct {
	Students  []*Student
	Teacher   *Teacher
	Questions []*Question
	MaxRounds int
}
