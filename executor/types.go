package executor

import (
	"fmt"
	"sync"
	"time"
)

type State int

func (s State) String() string {
	return stateMap[s]
}

var stateMap = map[State]string{
	Queued:    "Queued",
	Running:   "Running",
	Failed:    "Failed",
	Completed: "Completed",
}

const (
	Queued State = iota
	Running
	Failed
	Completed
)

type Status struct {
	CreatedAt    time.Time
	Mutex        *sync.Mutex
	Pid          int
	StartedAt    time.Time
	State        State
	TerminatedAt time.Time
	err          error
}

func (s *Status) String() string {
	// Make better string representation for terminated time
	return fmt.Sprintf("PID: %d, Created: %s, Started: %s, State: %d, Terminated: %s",
		s.Pid, s.CreatedAt.Local().String(), s.StartedAt.Local().String(), s.State, s.TerminatedAt.Local().String())
}
