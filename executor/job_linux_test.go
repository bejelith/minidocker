//go:build linux

package executor

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestCGroupCreation(t *testing.T) {
	config := ProcessConfig{
		CPUPercent:  10,
		deviceMajor: 200,
		MemoryMB:    10,
		ReadBPS:     10,
		WriteBPS:    10,
	}

	files := []struct {
		name        string
		config      ProcessConfig
		file        string
		output      string
		errExpected bool
	}{{
		name:   "WriteCpuMax",
		config: config,
		file:   "cpu.max",
		output: "10000 100000",
	}, {
		name:   "WriteMemoryHih",
		config: config,
		file:   "memory.high",
		output: "10485760",
	}, {
		name:   "WriteMemoryMax",
		config: config,
		file:   "memory.max",
		output: "10485760",
	}, {
		name:   "WriteIOMax",
		config: config,
		file:   "io.max",
		output: "200:0 rbps=10 wbps=10",
	}, {
		name:   "EnforceIOLowerBound",
		config: ProcessConfig{deviceMajor: 200, ReadBPS: 1, WriteBPS: 2},
		file:   "io.max",
		output: "200:0 wbps=2",
	}, {
		name:   "LimitIOLowerBound",
		config: ProcessConfig{MemoryMB: 5},
		file:   "memory.high",
		// This is a shortcut, for tests that do not intend to modify the file
		// i'd read the original file content and verify it hasn't mutated.
		// This is useful when the content of the file is not static, in this case is set by the OS.
		errExpected: true,
	}}

	for _, test := range files {
		t.Run(test.name, func(t *testing.T) {
			fd, cgroupDir, err := writeCgroup(0, test.config, t.TempDir())
			if err != nil {
				t.Fatal("writeCgroup return error: ", err.Error())
			}

			path := fmt.Sprintf("%s/%s", cgroupDir, test.file)
			t.Cleanup(func() { os.Remove(path) })
			if fd <= 2 {
				t.Fatalf("invalid fd number %d generated", fd)
			}

			output, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			trimmed := strings.Trim(string(output), " \n")
			if trimmed != test.output && !test.errExpected {
				t.Fatalf("expected \"%s\" but received \"%s\"", test.output, trimmed)
			}
		})
	}
}
