package executor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

var tests = []struct {
	name         string
	command      string
	args         []string
	err          bool
	initialState State
	endState     State
}{
	{"NormalExecution",
		"bash",
		[]string{"-c", "echo hello; sleep 2"},
		false,
		Running,
		Completed,
	},
	{
		"NonZeroExit",
		"bash",
		[]string{"-c", "echo hello; exit 1;"},
		false,
		Running,
		Failed,
	},
	{
		"FailedExecution",
		"bashh",
		[]string{"-c", "echo hello; exit 0;"},
		true,
		Failed,
		Failed,
	},
}

func TestJobExecution(t *testing.T) {
	for i, test := range tests {
		f := func(t *testing.T) {
			proc := newProcess(uint64(i), ProcessConfig{Cmd: test.command, Args: test.args})
			err := proc.Start()
			if err == nil == test.err {
				t.Fatalf("start returned error: %v", err)
			} else if err != nil {
				return
			}
			status := proc.Status().State
			if status != test.initialState {
				t.Fatalf("job status should be %s but %s found", test.initialState, status)
			}
			<-proc.Done()
			status = proc.Status().State
			if status != test.endState {
				t.Fatalf("job status should be %s but %s found", test.endState, status)
			}
		}
		t.Run(test.name, f)
	}
}

func TestJobStop(t *testing.T) {
	// bash's sleep ignores SIGTERM so it's necessary to use "sleep & wait"
	script := `"trap 'echo trapped the TERM signal;sleep 1; exit 1;
				while true;do sleep 0.1& wait; done; exit 1`
	job := newProcess(1, ProcessConfig{Cmd: "bash", Args: []string{"-c", script}})

	err := job.Start()
	if err != nil {
		t.Fatalf("start returned error: %v", err)
	}

	status := job.Status()
	if status.State != Running {
		t.Fatalf("job status should be %s but %s found", Running, status.State)
	}

	job.Stop()
	status = job.Status()
	if status.State != Failed {
		t.Fatalf("job status should be %s but %s found", Failed, status.State)
	}

}

func TestStdout(t *testing.T) {
	expectedOutput := "hello\n"
	args := []string{"-c", fmt.Sprintf("echo -e \"%s\"; sleep 2", expectedOutput)}
	job := newProcess(1, ProcessConfig{Cmd: "bash", Args: args})

	startError := job.Start()
	if startError != nil {
		t.Fatalf("can't start job: %v", startError)
	}
	defer os.Remove(job.outputFile.Name())
	reader, startError := job.Stdout()
	if startError != nil {
		t.Fatal(startError)
	}
	buf := make([]byte, 1)
	stringBuffer := bytes.NewBufferString("")
	for n, readErr := reader.Read(buf); stringBuffer.Len() < len(expectedOutput); n, readErr = reader.Read(buf) {
		if n > 0 {
			stringBuffer.Write(buf[:n])
		}
		if readErr != io.EOF && readErr != nil {
			t.Fatalf("expected EOF but '%v' was returned", readErr)
		}
	}
	job.Stop()
	if stringBuffer.String() != "hello\n" {
		t.Fatalf("Expected hello but found '%s'", stringBuffer.String())
	}

}
