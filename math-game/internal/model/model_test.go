package model_test

import (
	"testing"

	"github.com/Anthya1104/math-game-cli/internal/model"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func TestNewTeacher(t *testing.T) {
	name := "Mr. John"
	teacher := model.NewTeacher(name)
	if teacher.Name != name {
		t.Errorf("expected name %s, got %s", name, teacher.Name)
	}
	if teacher.Role != model.RoleTeacher {
		t.Errorf("expected role 'teacher', got %s", teacher.Role)
	}
}

func TestNewStudent(t *testing.T) {
	name := "Alice"
	id := 1
	student := model.NewStudent(name, id)
	if student.Name != name {
		t.Errorf("expected name %s, got %s", name, student.Name)
	}
	if student.Role != model.RoleStudent {
		t.Errorf("expected role 'student', got %s", student.Role)
	}
	if student.StudentID != id {
		t.Errorf("expected id %d, got %d", id, student.StudentID)
	}
	if student.AnswerCh == nil {
		t.Errorf("expected AnswerCh to be initialized")
	}
}

func TestNewQuestion(t *testing.T) {
	for i := 0; i < 10; i++ {
		q, err := model.NewQuestion(i)
		if err != nil {
			t.Errorf("NewQuestion returned error: %v", err)
		}
		if q.ID != i {
			t.Errorf("expected ID %d, got %d", i, q.ID)
		}
		ans, err := q.Operator.Apply(q.ArgumentA, q.ArgumentB)
		if err != nil {
			t.Errorf("Apply error: %v", err)
		}
		if ans != q.Answer {
			t.Errorf("expected answer %d, got %d", ans, q.Answer)
		}
	}
}
