package signal

import (
	"os"
	"os/signal"
	"syscall"
)

func SetupSignalHandler(f func(os.Signal)) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigc
		f(s)
	}()
}
