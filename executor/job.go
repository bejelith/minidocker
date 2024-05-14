package executor

import (
	"context"
	"fmt"
	"io"
	"minidocker/internal/mount"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ProcessConfig represents any process that can be started, stopped and monitored
type ProcessConfig struct {
	// Binary executable to start
	Cmd string
	// Args is a slice of arguments to pass to Cmd
	Args []string
	// cgroupPrefix is used to avoid cgroup naming collisions with other processes
	cgroupPrefix string
	// CPUPercent represents the quota of cpu to use for all cores. We would't assume the user knows
	// the number of cores available so the minimum value is 1 and max is 100.
	CPUPercent  uint
	deviceMajor uint
	deviceMinor uint
	// MemoryMB represents the quota of memory to in Megabytes, it will be applied as Memory High in the CGroup
	MemoryMB uint
	// ReadBPS represents the maximum bytes for second the process can read
	ReadBPS uint
	// WriteBPS represents the maximum bytes for second the process can read
	WriteBPS uint
}

type process struct {
	// config the configuration struct
	config ProcessConfig
	// cgroupPath keeps track of the cgroup location for later deletion
	cgroupPath string
	// done is used to signal Process termination
	done chan struct{}
	// ID is the process identification.
	ID uint64
	// execCmd returns the os.Process of the child
	execCmd *exec.Cmd
	// outputFile is the FD of a file holding the stdout and stderr of the process
	outputFile *os.File
	// started is used to not start the same process twice with atomic CompareSwap/IncrementAndGet pattern
	started int32
	// status represents the Process status and exit code
	status *Status
}

func newProcess(pid uint64, c ProcessConfig) *process {
	return &process{
		ID:     pid,
		config: c,
		done:   make(chan struct{}),
		status: &Status{
			CreatedAt:    time.Now(),
			Mutex:        &sync.Mutex{},
			Pid:          -1,
			StartedAt:    time.Time{},
			State:        Queued,
			TerminatedAt: time.Time{},
		},
	}
}

func (p *process) Start() error {
	if !atomic.CompareAndSwapInt32(&p.started, 0, 1) {
		return fmt.Errorf("job %d has been started already", p.ID)
	}

	p.status.Mutex.Lock()
	defer p.status.Mutex.Unlock()

	p.status.StartedAt = time.Now()

	ctx := context.Background()

	var err error
	if p.execCmd, err = p.execute(ctx); err != nil {
		p.status.State = Failed
		return err
	}

	p.status.Pid = p.execCmd.Process.Pid
	p.status.State = Running

	go p.cleanUp()
	return nil
}

// cleanUp waits for the child to exit to cleanup Cgroups and signal listeners
func (p *process) cleanUp() {
	state, _ := p.execCmd.Process.Wait()
	p.outputFile.Sync()

	p.status.Mutex.Lock()
	defer p.status.Mutex.Unlock()

	p.status.TerminatedAt = time.Now()
	if state.ExitCode() > 0 {
		p.status.State = Failed
		p.status.err = fmt.Errorf(state.String())
	} else {
		p.status.State = Completed
	}

	_ = rmCgroup(p.cgroupPath)
	close(p.done)
}

// Error returns and error if child process terminated unsuccessfully, nil otherwise
func (p *process) Error() error {
	return p.Status().err
}

// Stop will try to terminate the underling process
// and wait for it's termination until WaitDelay is reached.
// If the child ignores SIGTERM, SIGKILL is sent twice at WaitDelay interval.
func (p *process) Stop() {
	if p.Status().State == Completed || p.Status().State == Failed {
		return
	}
	p.execCmd.Cancel()

	// NewTicker duration must be a positive number
	ticker := time.NewTicker(max(1, p.execCmd.WaitDelay))
	defer ticker.Stop()
	for c := 0; c < 2; c++ {
		select {
		case <-p.Done():
			return
		case <-ticker.C:
			p.execCmd.Process.Signal(syscall.SIGKILL)
		}
	}
}

func (p *process) Stdout() (io.ReadCloser, error) {
	if p.started == 0 {
		return nil, fmt.Errorf("job not started yet")
	}
	return os.OpenFile(p.outputFile.Name(), os.O_RDONLY, 0660)
}

func (p *process) Status() Status {
	p.status.Mutex.Lock()
	status := *p.status
	p.status.Mutex.Unlock()
	return status
}

func (p *process) Done() <-chan struct{} {
	return p.done
}

func (p *process) execute(ctx context.Context) (*exec.Cmd, error) {
	path, pathErr := exec.LookPath(p.config.Cmd)
	if pathErr != nil {
		return nil, pathErr
	}

	cmd := exec.CommandContext(ctx, "/proc/self/exe")
	stdout, openErr := os.CreateTemp("", "*")
	if openErr != nil {
		return nil, openErr
	}

	// We replace Cancel to use SIGTERM instead of the default behavior
	// which uses SIGKILL
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}

	cgroupFD, cgroupPath, _ := p.setupCgroup()
	p.cgroupPath = cgroupPath
	p.outputFile = stdout
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	cmd.SysProcAttr = mount.NewSysProcAttr(cgroupFD)
	cmd.Env = generateEnv(path, p.config.Args)

	// We set an arbitrary deadline if the child ignores SIGTERM
	cmd.WaitDelay = time.Second * 30

	// Append PATH so that we don't need full paths for common executables
	cmd.Env = append(cmd.Env, "PATH="+environment["PATH"])

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}
