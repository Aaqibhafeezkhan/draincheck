package main

import "errors"

// Process exit codes.
const (
	// exitFindings means the analysis ran and found something at or above
	// the --fail-on threshold.
	exitFindings = 1
	// exitFailure means draincheck could not complete (bad flags, cluster
	// unreachable, ...).
	exitFailure = 2
)

// exitError carries the exit code main should use. An empty message means
// the failure was already reported and nothing more is printed.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// exitCodeOf returns the exit code carried by err, or exitFailure.
func exitCodeOf(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return exitFailure
}
