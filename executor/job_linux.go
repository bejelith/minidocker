//go:build linux

package executor

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

const cgroupPath = "/sys/fs/cgroup"

var maxCPUTime = uint(100000)

// setupCgroup can create create only children to the current cgroup
func (p *process) setupCgroup() (int, string, error) {
	return writeCgroup(p.ID, p.config, cgroupPath)
}

// writeCgroup builds and configures are new Cgroup for the Process (pid)
// at the moment applies the values and does not verify they are coherent with system resources or
// for fairness with other processes.
func writeCgroup(pid uint64, c ProcessConfig, basePath string) (fd int, cgroupPath string, err error) {

	// Creates a root CGroup under /sys/fs/cgroup: only leaf nodes can hold processes that's the reason
	// we are avoiding nesting, simplicity.
	// TODO: Don't just use the process ID to avoid conflicts with other instances of Executor
	cgroupPath = fmt.Sprintf("%s/%s-%d", basePath, c.cgroupPrefix, pid)
	err = os.MkdirAll(cgroupPath, 0755)
	if err != nil && err != syscall.EEXIST {
		return
	}

	// We need unix.Open to get a FD in int format.
	// Also seems syscall is being deprecated too
	fd, err = unix.Open(cgroupPath, unix.O_PATH, 0)

	if c.CPUPercent > 0 {
		cpuMaxFile := []byte(fmt.Sprintf("%d %d", maxCPUTime/100*c.CPUPercent, maxCPUTime))
		if err = os.WriteFile(cgroupPath+"/cpu.max", cpuMaxFile, 0664); err != nil {
			return
		}
	}

	if c.MemoryMB > 0 {
		memFile := []byte(fmt.Sprintf("%d\n", c.MemoryMB*1024*1024))
		if err = os.WriteFile(cgroupPath+"/memory.max", memFile, 0664); err != nil {
			return
		}
		if err = os.WriteFile(cgroupPath+"/memory.high", memFile, 0664); err != nil {
			return
		}
	}

	// Minimum acceptable value is 2
	// write the file only if there at least one bounded value
	// Tested:
	// [root@ip-172-31-20-250 0]# echo "202:0 rbps=1" >io.max
	// -bash: echo: write error: Invalid argument
	// [root@ip-172-31-20-250 0]# echo "202:0 rbps=2" >io.max
	// [root@ip-172-31-20-250 0]#
	if c.deviceMajor > 1 && (c.ReadBPS >= 2 || c.WriteBPS >= 2) {
		// CGroup does not accept partitions so we only use the major for block devices.
		// There would be the need of supporting other types of devices depending on where
		// the workload is scheduled
		stringBuffer := bytes.NewBufferString(fmt.Sprintf("%d:%d", c.deviceMajor, 0))
		if c.ReadBPS > 1 {
			stringBuffer.WriteString(" ")
			stringBuffer.WriteString("rbps=")
			stringBuffer.WriteString(strconv.Itoa(int(c.ReadBPS)))
		}
		if c.WriteBPS > 1 {
			stringBuffer.WriteString(" ")
			stringBuffer.WriteString("wbps=")
			stringBuffer.WriteString(strconv.Itoa(int(c.WriteBPS)))
		}
		if err = os.WriteFile(cgroupPath+"/io.max", stringBuffer.Bytes(), 0600); err != nil {
			return
		}
	}
	return
}

func rmCgroup(path string) error {
	return unix.Rmdir(path)
}
