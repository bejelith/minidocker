package executor

import "testing"

func TestIsHelper(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		success bool
	}{
		{"nilMap", nil, false},
		{"emptyEnv", map[string]string{}, false},
		{"handleEmptyVar", map[string]string{"JES_CHILD_ENV_VAR": ""}, false},
		{"happyCase", map[string]string{jesChildEnvVar: "true"}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if isHelper(test.env) != test.success {
				t.Fatalf("Expected %v", test.success)
			}
		})
	}
}

func TestBuildExecArgsShould(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		argc    int
		success bool
	}{
		{"nilMap", nil, 0, false},
		{"emptyEnv", map[string]string{}, 0, false},
		{"happyCase", map[string]string{
			jesArgCountEnvVar: "1",
			"JES_ARG_0":       "arg1",
			jesCmdEnvVar:      "cmd",
		}, 2, true},
		{"skipEmptyVars", map[string]string{
			jesArgCountEnvVar: "2",
			"JES_ARG_0":       "arg1",
			jesCmdEnvVar:      "cmd",
		}, 2, true},
		{"missingVars", map[string]string{jesChildEnvVar: "true"}, 0, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := buildExecArgs(test.env)
			if (err == nil) != test.success {
				t.Fatalf("Expected %v, %v", test.success, err)
			}
			if len(args) != test.argc {
				t.Fatalf("Expected %v but found %v args", test.argc, len(args))
			}
		})
	}
}
