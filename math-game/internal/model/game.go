package model

import (
	"context" // Import fmt for new log messages
	"math/rand"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type DefaultStudentActioner struct{}

type RoundResult struct {
	Student    *Student
	Answer     int
	QuestionID int
	IsCorrect  bool
}

type Game struct {
	Students  []*Student
	Teacher   *Teacher
	MaxRounds int

	Results   []RoundResult
	resultsMu sync.Mutex // Mutex to protect access to the Results slice

	// Injected student action interface
	StudentActioner StudentActioner

	roundResultCh chan RoundResult
	roundsWg      sync.WaitGroup

	gameDone chan struct{} // signal to exit the whole game plays
}

func (g *Game) Start(ctx context.Context) <-chan struct{} {
	logrus.Infof("Teacher: Guys, are you ready?")

	g.gameDone = make(chan struct{})

	// WaitGroup to track the two main goroutines launched by Start:
	// 1. The question generation/orchestration goroutine
	// 2. The result collection goroutine
	var gameMainWg sync.WaitGroup
	gameMainWg.Add(2)

	countdownSeconds := int(g.Teacher.WaitTime.Seconds())
	if countdownSeconds <= 0 {
		countdownSeconds = 3
	}

	for i := countdownSeconds; i > 0; i-- {
		select {
		case <-ctx.Done():
			logrus.Warnf("Game interrupted during countdown.")
			close(g.gameDone) // Close gameDone if interrupted early during countdown
			return g.gameDone // Return immediately if context is cancelled
		case <-time.After(1 * time.Second):
			logrus.Infof("# Count %d", i)
		}
	}

	g.roundResultCh = make(chan RoundResult, g.MaxRounds)

	go func() {
		defer gameMainWg.Done() // Signal completion of this goroutine to gameMainWg
		g.collectResults(ctx)   // This will block until roundResultCh is closed
	}()

	// Goroutine to generate and launch questions concurrently
	go func() {
		defer gameMainWg.Done()      // Signal completion of this goroutine to gameMainWg
		defer close(g.roundResultCh) // Close the results channel when all questions are launched/processed

		for i := 1; i <= g.MaxRounds; i++ {
			select {
			case <-ctx.Done():
				logrus.Warnf("Game interrupted: Stopping question generation.")
				g.roundsWg.Add(-1 * (g.MaxRounds - (i - 1)))
				return
			case <-time.After(1 * time.Second): // Teacher writes a question per second
				// Generate question
				q, err := NewQuestion(i)
				if err != nil {
					logrus.Errorf("Failed to generate question %d: %v", i, err)
					g.roundsWg.Done()
					continue
				}

				g.roundsWg.Add(1)
				go g.PlayQuestion(ctx, q)
			}
		}

		g.roundsWg.Wait()
		logrus.Infof("All question rounds processed.")

	}()

	// Goroutine to wait for all main game goroutines to finish and then signal game completion
	go func() {
		gameMainWg.Wait()
		close(g.gameDone)
		logrus.Debug("Game finished signal sent.")
	}()

	return g.gameDone // Return the gameDone channel
}

func (g *Game) PlayQuestion(ctx context.Context, q *Question) {
	defer g.roundsWg.Done()

	logrus.Infof("Teacher: Q%d: %d %s %d = ?", q.ID, q.ArgumentA, q.Operator, q.ArgumentB)

	roundCtx, cancelRound := context.WithCancel(ctx)
	defer cancelRound()

	answerCh := make(chan AnswerEvent, len(g.Students))
	var studentWg sync.WaitGroup

	for _, s := range g.Students {
		studentWg.Add(1)
		go func(student *Student) {
			defer studentWg.Done()
			g.StudentActioner.AskStudent(roundCtx, student, q, answerCh)
		}(s)
	}

	correctAnswerFound := false
	answersReceived := 0

	go func() {
		studentWg.Wait()
		close(answerCh)
	}()

	winnerStudent := (*Student)(nil)

	for answerEvent := range answerCh {
		answersReceived++

		logrus.Infof("%s: Q%d: %d %s %d = %d!", answerEvent.Student.Name, answerEvent.QID, q.ArgumentA, q.Operator, q.ArgumentB, answerEvent.Answer)

		if answerEvent.Answer == q.Answer {
			// Correct answer found for this question
			g.roundResultCh <- RoundResult{Student: answerEvent.Student, Answer: answerEvent.Answer, QuestionID: q.ID, IsCorrect: true}
			logrus.Infof("Teacher: %s, Q%d you are right!", answerEvent.Student.Name, q.ID)
			correctAnswerFound = true
			winnerStudent = answerEvent.Student
			cancelRound()
			break // Break from this for-range loop, no more answers needed for this question
		} else {
			logrus.Infof("Teacher: %s, Q%d wrong answer! (%s)", answerEvent.Student.Name, q.ID, q.String())
		}
	}

	if !correctAnswerFound {
		// If no correct answer was found after all students (who could) answered or channel closed
		logrus.Infof("Teacher: Boooo~ Q%d Answer is %d.", q.ID, q.Answer)
		// send a RoundResult indicating no winner
		g.roundResultCh <- RoundResult{QuestionID: q.ID, IsCorrect: false, Answer: q.Answer}
	} else {
		// Announce winner to other students for this specific question
		for _, s := range g.Students {
			if s.Name != winnerStudent.Name { // Students who are not the winner of THIS question
				logrus.Infof("%s: %s, Q%d you win!", s.Name, winnerStudent.Name, q.ID)
			}
		}
	}
	logrus.Infof("--- Round Q%d Ends ---", q.ID)
}

func (g *Game) collectResults(ctx context.Context) {
	// This goroutine runs until the g.roundResultCh is closed AND all buffered results are processed.
	for {
		select {
		case <-ctx.Done():
			// If main context is cancelled, try to drain channel quickly then exit
			logrus.Warnf("Result collector interrupted by main context cancellation.")
			// drain remaining buffered results before exiting
			for len(g.roundResultCh) > 0 {
				res := <-g.roundResultCh
				g.resultsMu.Lock()
				g.Results = append(g.Results, res)
				g.resultsMu.Unlock()
				logrus.Debugf("Collected buffered result for Q%d during shutdown.", res.QuestionID)
			}
			return
		case res, ok := <-g.roundResultCh:
			if !ok {
				// Channel closed and drained
				logrus.Infof("Result channel closed. All results collected.")
				return
			}
			g.resultsMu.Lock()
			g.Results = append(g.Results, res)
			g.resultsMu.Unlock()
			logrus.Debugf("Collected result for Q%d (Correct: %t)", res.QuestionID, res.IsCorrect)
		}
	}
}
func (d *DefaultStudentActioner) AskStudent(ctx context.Context, s *Student, q *Question, ch chan AnswerEvent) {
	// Simulate student's thinking time
	select {
	case <-ctx.Done():
		logrus.Debugf("%s's goroutine for Q%d cancelled during thinking.", s.Name, q.ID)
		return
	case <-time.After(s.WaitTime):
		// Student finished thinking
	}

	// Simulate student giving a correct or wrong answer (30% chance of wrong answer)
	answer := q.Answer
	if rand.Float32() < 0.3 {
		// Generate a wrong answer that is guaranteed not to be the correct one
		wrongAnswer := q.Answer + rand.Intn(10) - 5
		if wrongAnswer == q.Answer {
			wrongAnswer++
		}
		answer = wrongAnswer
	}

	// Send the answer to the channel
	select {
	case <-ctx.Done(): // Check context again before sending, in case it was cancelled during delay
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
