//go:build windows

package update

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func platformWaitForParent(ctx context.Context, parentPID int) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(parentPID))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	done := make(chan error, 1)
	go func() {
		result, waitErr := windows.WaitForSingleObject(handle, windows.INFINITE)
		if waitErr == nil && result != windows.WAIT_OBJECT_0 {
			waitErr = errors.New("unexpected parent wait result")
		}
		done <- waitErr
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
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
