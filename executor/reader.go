package executor

import (
	"io"
	"time"
)

// PollingReader implements io.Reader and will try Reader.Read() against a Process until it terminates
type pollingReader struct {
	p      *process
	reader io.ReadCloser
}

// Read will read n bytes into b buffer and return EOF only when Process p has terminated
func (r *pollingReader) Read(b []byte) (n int, err error) {
	n, err = r.reader.Read(b)
	// Ignore EOF in this case the process is still running
	if n == 0 && err == io.EOF && r.p.Status().State == Running {
		err = nil
		if n == 0 {
			<-time.After(250 * time.Millisecond)
		}
	}
	return
}

// Close closes the underlying FD
func (r *pollingReader) Close() error {
	return r.reader.Close()
}
