package mount

import (
	"bufio"
	"bytes"
	"testing"
)

func TestMountInfoReader(t *testing.T) {
	mountInfo :=
		`27 20 0:24 / /sys/fs/pstore rw,nosuid,nodev,noexec,relatime shared:5 - pstore pstore rw,seclabel
		28 20 0:25 / /sys/fs/bpf rw,nosuid,nodev,noexec,relatime shared:6 - bpf bpf rw,mode=700
		59 1 202:1 / / rw,noatime shared:1 - xfs /dev/xvda1 rw,seclabel,attr2,inode64,logbufs=8,logbsize=32k,sunit=1024,swidth=1024,noquota
		30 20 0:17 / /sys/fs/selinux rw,nosuid,noexec,relatime shared:7 - selinuxfs selinuxfs rw
		31 19 0:26 / /proc/sys/fs/binfmt_misc rw,relatime shared:13 - autofs systemd-1 rw,fd=29,pgrp=1,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=12974
		32 21 0:27 / /dev/hugepages rw,relatime shared:14 - hugetlbfs hugetlbfs rw,seclabel,pagesize=2M
		33 21 0:16 / /dev/mqueue rw,nosuid,nodev,noexec,relatime shared:15 - mqueue mqueue rw,seclabel
		34 20 0:7 / /sys/kernel/debug rw,nosuid,nodev,noexec,relatime shared:16 - debugfs debugfs rw,seclabel`

	r := bufio.NewReader(bytes.NewBuffer([]byte(mountInfo)))
	major, minor, err := ReadRootFSMount(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if major != 202 {
		t.Fatalf("expected major 202 but %d was found", major)
	}
	if minor != 1 {
		t.Fatalf("expected major 1 but %d was found", minor)
	}
}

func TestMountInfoReaderFailure(t *testing.T) {
	mountInfo :=
		`27 20 0:24 / /sys/fs/pstore rw,nosuid,nodev,noexec,relatime shared:5 - pstore pstore rw,seclabel
		28 20 0:25 / /sys/fs/bpf rw,nosuid,nodev,noexec,relatime shared:6 - bpf bpf rw,mode=700
		34 20 0:7 / /sys/kernel/debug rw,nosuid,nodev,noexec,relatime shared:16 - debugfs debugfs rw,seclabel`

	r := bufio.NewReader(bytes.NewBuffer([]byte(mountInfo)))
	_, _, err := ReadRootFSMount(r)
	if err != nil {
		return
	}
	t.Fatalf("error was expected but readMountInfo returned nil")
}
