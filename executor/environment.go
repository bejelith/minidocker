package executor

import (
	"fmt"
	"os"
	"strings"
)

var environment map[string]string = environmentToMap()

// generateEnv builds the JES_CHILD_STRING, CMD and ARGS_$i environment variables in a slice
// takes in input an executable c and a slice of arguments
func generateEnv(c string, a []string) []string {
	var b []string
	for i, s := range a {
		b = append(b, fmt.Sprintf("%s%d=%s", jesArgPrefix, i, s))
	}
	b = append(b, fmt.Sprintf("%s=%s", jesCmdEnvVar, c))
	b = append(b, fmt.Sprintf("%s=true", jesChildEnvVar))
	b = append(b, fmt.Sprintf("%s=%d", jesArgCountEnvVar, len(a)))
	return b
}

// environmentToMap reduces the Environment slice to Map[string]string for easier access
func environmentToMap() map[string]string {
	env := map[string]string{}
	for _, e := range os.Environ() {
		t := strings.SplitN(e, "=", 2)
		// Filter out environment flags set to empty string
		if len(t) > 0 {
			env[t[0]] = t[1]
		}
	}
	return env
}
