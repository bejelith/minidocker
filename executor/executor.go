package executor

import (
	"fmt"
	"io"
	"minidocker/internal/mount"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ProcInfo represents a Process status to the calling client
type ProcInfo struct {
	// Id is a monotonic Process identifier as we don't expect to execute at a big scale
	// Clients of the library will see this ID as Process ID and not real Linux Pid.
	ID uint64
	// CreatedAt reports the time the process was created
	CreatedAt time.Time
	// Error reports if the process has terminated with non 0 exit status
	Error error
	// OsPid represent the Linux process ID of the child
	OsPid int
	// StartedAt represents the process was started
	StartedAt time.Time
	// State represents the process state (WAITING, RUNNING, ABORTED, COMPLETED)
	State string
	// TerminatedAt is the time he process was terminated
	TerminatedAt time.Time
}

// Executor is a simple Process executor for Linux that guarantees isolation between
// children with the use of linux namespaces [https://man7.org/linux/man-pages/man7/namespaces.7.html]
// NOTE: This component is unbounded as it's not a requirement in the feature request doc, normally
// process should be inserted into a bounded queue and only 'MaxProcs' would run concurrently.
type Executor struct {
	deviceMaj uint
	deviceMin uint
	done      chan struct{}
	id        uuid.UUID
	jobs      map[uint64]*process
	mutex     sync.RWMutex
	nextID    int64
	wg        *sync.WaitGroup
}

func New() (*Executor, error) {
	maj, min, err := mount.GetRootDeviceMajorMinor()
	if err != nil {
		return nil, fmt.Errorf("error reading device info: %w", err)
	}
	s := &Executor{
		deviceMaj: maj,
		deviceMin: min,
		id:        uuid.New(),
		jobs:      make(map[uint64]*process),
		mutex:     sync.RWMutex{},
		nextID:    -1,
		wg:        &sync.WaitGroup{},
		done:      make(chan struct{}),
	}
	return s, nil
}

// Get returns a process by it's ID and returns nil if ID is invalid
func (s *Executor) Get(ID uint64) *ProcInfo {
	s.mutex.RLock()
	p, found := s.jobs[ID]
	s.mutex.RUnlock()
	if !found {
		return nil
	}
	status := p.Status()
	return &ProcInfo{
		ID:           p.ID,
		CreatedAt:    status.CreatedAt,
		OsPid:        p.execCmd.Process.Pid,
		State:        status.State.String(),
		Error:        p.execCmd.Err,
		TerminatedAt: status.TerminatedAt,
	}
}

// List returns a slice of all process IDs that where managed by the instance
func (s *Executor) List() []uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var output = make([]uint64, 0, 10)
	for id := range s.jobs {
		output = append(output, id)
	}
	return output
}

// Start starts a process and executes it immediately,
// returns a process ID or error is returned if the process can not be started
// NOTE: I would normally make this call blocking if the Process intake Queue has reached capacity.
func (s *Executor) Start(c *ProcessConfig) (uint64, error) {
	id := atomic.AddInt64(&s.nextID, int64(1))
	c.deviceMajor = s.deviceMaj
	c.deviceMinor = s.deviceMin
	c.cgroupPrefix = s.id.String()
	p := newProcess(uint64(id), *c)

	if err := p.Start(); err != nil {
		return 0, err
	}
	s.mutex.Lock()
	s.jobs[p.ID] = p
	s.mutex.Unlock()
	s.wg.Add(1)

	go func() {
		<-p.Done()
		s.wg.Done()
	}()
	return p.ID, nil
}

// Stdout returns a io.Reader to the process standard output
// and will return nil if not processId is invalid
// The returned io.Reader will return EOF only when the child process terminates
// emulating the behavior of "docker logs -f"
func (s *Executor) Stdout(p uint64) (io.ReadCloser, error) {
	s.mutex.RLock()
	j, ok := s.jobs[p]
	s.mutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("job %d not found", p)
	}
	r, e := j.Stdout()
	if e != nil {
		return nil, e
	}
	return &pollingReader{j, r}, nil
}

// Wait will block until there no active processes
func (s *Executor) Wait() {
	s.wg.Wait()
}

// Stop terminates the process and cleans up it's CGroup and namespaces
func (s *Executor) Stop() {
	s.mutex.RLock()
	for _, job := range s.jobs {
		switch job.Status().State {
		case Running:
			job.Stop()
		}
	}
	s.mutex.RUnlock()
	s.wg.Wait()
}

// StopProcess terminates the process indicated by pid
func (s *Executor) StopProcess(pid uint64) {
	s.mutex.RLock()
	p, ok := s.jobs[pid]
	s.mutex.RUnlock()
	if ok {
		p.Stop()
	}
}
