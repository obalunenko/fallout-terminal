//go:build !windows

package update

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const helperParentPollInterval = 50 * time.Millisecond

func platformWaitForParent(ctx context.Context, parentPID int) error {
	process, err := os.FindProcess(parentPID)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(helperParentPollInterval)
	defer ticker.Stop()

	for {
		err = process.Signal(syscall.Signal(0))
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func platformRelaunch(ctx context.Context, unit, relativePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	command := exec.Command(filepath.Join(unit, relativePath))
	command.Dir = unit
	command.Env = sanitizedHelperEnvironment(os.Environ())
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func platformStartHelper(executable string, environment []string) error {
	command := exec.Command(executable)
	command.Env = environment
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
