package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"minidocker/executor"
	"minidocker/signal"
)

var cpuPercent = flag.Uint("cpu", 0, "cpu %, 0 means no limit is applied")
var memoryMB = flag.Uint("mem", 0, "memory in megabytes, 0 means no limits is applied")
var rbps = flag.Uint("rbps", 0, "Read bytes/s, no limit is applied if value us not bigger then 1")
var wbps = flag.Uint("wbps", 0, "Write bytes/s, no limit is applied if value us not bigger then 1")

// This is just a test utility for system testing.
func main() {
	flag.Parse()
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage:\n\t%s [flags] executable [args]\nFlags:\n",
			filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	s, e := executor.New()
	if e != nil {
		fmt.Println(e)
		os.Exit(1)
	}
	config := &executor.ProcessConfig{
		Cmd:        flag.Args()[0],
		Args:       flag.Args()[1:],
		CPUPercent: *cpuPercent,
		MemoryMB:   *memoryMB,
		ReadBPS:    *rbps,
		WriteBPS:   *wbps}

	pid, err := s.Start(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer s.Stop()
	signal.SetupSignalHandler(func(sig os.Signal) {
		fmt.Println("terminated by", sig.String())
		s.Stop()
		os.Exit(1)
	})

	r, e := s.Stdout(pid)
	if e != nil {
		fmt.Println(e)
		return
	}

	buf := make([]byte, 1024)
	for {
		n, e := r.Read(buf)
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
		if e != nil {
			break
		}
	}
	r.Close()

	if s.Get(pid).Error != nil {
		fmt.Println(s.Get(pid).Error)
		os.Exit(1)
	}
}
