package executor

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"minidocker/internal/mount"
)

const jesArgPrefix = "JES_ARG_"
const jesArgCountEnvVar = "JES_ARGC"
const jesChildEnvVar = "JES_CHILD"
const jesCmdEnvVar = "JES_CMD"

// createSandbox runs in the helper child and creates mount points before executing the Process
func createSandbox() error {
	// Evaluate ENV parameters:
	// parses JES_CHILD_STRING, JES_CHILD_STRING and the list of JES_ARG_PREFIX_X
	args, envErr := buildExecArgs(environment)
	if envErr != nil {
		return fmt.Errorf("jes sandbox: error parsing environment: %w", envErr)
	}

	// Mount proc to reduce visibility of other PIDs
	if err := mount.HideMounts(); err != nil {
		return fmt.Errorf("jes sandbox: failed to mount proc fs: %w", err)
	}

	// Exec allows us to retain PID 1 so that the output of `ps` looks cooler
	if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
		return fmt.Errorf("jes sandbox: error executing command %s, with %s", args[0], args[1:])
	}
	return nil
}

// buildExecArgs retrieves the executable parameters from ENV
// and executes a slice of Args compliant with syscall.Exec()
func buildExecArgs(e map[string]string) (args []string, err error) {

	// Verify we received the CMD environment variable
	cmd, cmdExists := e[jesCmdEnvVar]
	if !cmdExists {
		err = fmt.Errorf(jesChildEnvVar + " must exists and set to a non-empty string")
		return
	}
	args = append(args, cmd)

	n, err := strconv.Atoi(e[jesArgCountEnvVar])
	if err != nil {
		return nil, err
	}

	// limit the maxing number of arguments
	if n > 50 {
		return nil, fmt.Errorf("too many arguments, 50 is the limit")
	}

	// Discover all arguments
	for i := 0; i < n; i++ {
		// Skip empty string args
		argName := fmt.Sprintf("%s%d", jesArgPrefix, i)
		if v, exists := e[argName]; exists && len(v) > 0 {
			args = append(args, v)
		}
	}

	return
}

// isHelper returns true if JES_CHILD_ENV_VAR is found in ENV,
// which means we are being executed has a child from the main process
func isHelper(e map[string]string) bool {
	// Check if we are the helper child by checking for JES_CHILD_STRING in environment
	v, exists := e[jesChildEnvVar]
	if !exists || v != "true" {
		return false
	}
	return true
}

func init() {
	//Parent process, return and execute main()
	if !isHelper(environment) {
		return
	}

	if err := createSandbox(); err != nil {
		fmt.Fprintf(os.Stderr, "error building sandbox: %s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
