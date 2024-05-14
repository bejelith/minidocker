//go:build linux

package mount

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// HideMounts makes all mounts points private (MS_REC) so that the child can then mount tmpfs/proc or any other fs
// without spilling in the root mount namespace
func HideMounts() error {
	// Recursively shop sharing mounts
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("could not make / private: %v", err)
	}
	// Then mount the process proc fs in /proc
	if err := unix.Mount("", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("error mounting /proc: %v", err)
	}
	// Then mount the process tmpfs in /tmp
	if err := unix.Mount("", "/tmp", "tmpfs", 0, ""); err != nil {
		return fmt.Errorf("error mounting /tmp: %v", err)
	}
	return nil
}

// newSysProcAttr builds SysProcAttr to support namespaces and Cgroup association
// if cgroupFD is 0 means we are not creating a new cgroup
func NewSysProcAttr(cgroupFD int) *syscall.SysProcAttr {
	return &unix.SysProcAttr{
		Cloneflags:  unix.CLONE_NEWNS | unix.CLONE_NEWPID | unix.CLONE_NEWNET,
		CgroupFD:    cgroupFD,
		UseCgroupFD: cgroupFD != 0,
	}
}

// getRootDeviceMajorMinor automates the process of getting Major and Minor numbers of the block
// NOTE: As i looking at simplifying the project i simply think makes the process of using the library easier
// approaches would be to apply the limit for all mounted devices if we control Chroot and FS namespaces
// or let the user pick the device the limit applies to by device name (/dev/xxx)
func GetRootDeviceMajorMinor() (uint, uint, error) {
	if b, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		return ReadRootFSMount(bufio.NewReader(bytes.NewReader(b)))
	} else {
		return 0, 0, err
	}
}
