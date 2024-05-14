//go:build darwin

package mount

import (
	"fmt"
	"syscall"
)

// NOTE: This is just to make me able to build the project on darwin

// NoAction mount for non-linux builds
func HideMounts() error {
	return nil
}

// newSysProcAttr returns default struct for non-linux builds
func NewSysProcAttr(_ int) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func GetRootDeviceMajorMinor() (uint, uint, error) {
	return 0, 0, fmt.Errorf("not supported on darwin")
}

func SetRootMountPrivate() error {
	return nil
}
