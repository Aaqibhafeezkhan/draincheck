package main

import (
	"os/signal"
	"syscall"
)

// ignoreClosedPipes keeps a write to a closed stdout or stderr from ending the
// process with SIGPIPE (shell code 141). The write fails with an error
// instead, and the run keeps the exit code it earned.
func ignoreClosedPipes() {
	signal.Ignore(syscall.SIGPIPE)
}
