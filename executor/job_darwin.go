//go:build darwin

package executor

func (p *process) setupCgroup() (int, string, error) {
	return 0, "", nil
}

func rmCgroup(_ string) error {
	return nil
}
