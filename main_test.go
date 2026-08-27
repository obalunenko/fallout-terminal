package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunMainReportsVersionBeforeApplicationComposition(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	compositionStarted := false

	exitCode := runMain([]string{"--version"}, &stdout, &stderr, func() {
		compositionStarted = true
	})

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "development\n", stdout.String())
	assert.Empty(t, stderr.String())
	assert.False(t, compositionStarted)
}

func TestRunMainRejectsVersionArgumentsBeforeApplicationComposition(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	compositionStarted := false

	exitCode := runMain([]string{"--version", "unexpected"}, &stdout, &stderr, func() {
		compositionStarted = true
	})

	assert.Equal(t, 2, exitCode)
	assert.Empty(t, stdout.String())
	assert.NotEmpty(t, stderr.String())
	assert.False(t, compositionStarted)
}

func TestRunMainRejectsVersionOutputFailuresBeforeApplicationComposition(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
	}{
		{name: "version output", arguments: []string{"--version"}},
		{name: "usage output", arguments: []string{"--version", "unexpected"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compositionStarted := false
			exitCode := runMain(test.arguments, failingWriter{}, failingWriter{}, func() {
				compositionStarted = true
			})

			assert.Equal(t, 1, exitCode)
			assert.False(t, compositionStarted)
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("synthetic write failure")
}
