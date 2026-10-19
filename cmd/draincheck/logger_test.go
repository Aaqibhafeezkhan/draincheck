package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLoggerDebugWritesDebugLines(t *testing.T) {
	var buf bytes.Buffer

	newLogger(true, &buf).Debug("hello", "key", "value")

	got := buf.String()
	for _, want := range []string{"level=DEBUG", "msg=hello", "key=value"} {
		if !strings.Contains(got, want) {
			t.Errorf("log line %q is missing %q", got, want)
		}
	}
}

func TestNewLoggerWithoutDebugWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(false, &buf)

	logger.Debug("hello")
	logger.Info("hello")
	logger.Error("hello")

	if buf.Len() != 0 {
		t.Errorf("logger wrote without --debug: %q", buf.String())
	}
}
