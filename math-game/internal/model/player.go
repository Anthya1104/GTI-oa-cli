package model

import (
	"math/rand"
	"time"
)

// Role const for Player model
type Role int

const (
	RoleUnknown Role = iota
	RoleTeacher
	RoleStudent
)

func (r Role) String() string {
	switch r {
	case RoleTeacher:
		return "teacher"
	case RoleStudent:
		return "student"
	default:
		return "unknown"
	}
}

// basic model for game participants
type Player struct {
	Name     string
	Role     Role
	WaitTime time.Duration
}

type Teacher struct {
	Player
}

type Student struct {
	Player
	StudentID int
	AnswerCh  chan AnswerEvent // channel for answer race
}

func NewStudent(name string, id int) *Student {
	return &Student{
		Player: Player{
			Name:     name,
			Role:     RoleStudent,
			WaitTime: time.Duration(rand.Intn(3)+1) * time.Second,
		},
		StudentID: id,
		AnswerCh:  make(chan AnswerEvent, 1),
	}
}

func NewTeacher(name string) *Teacher {
	return &Teacher{
		Player: Player{
			Name: name,
			Role: RoleTeacher,
		},
	}
}
