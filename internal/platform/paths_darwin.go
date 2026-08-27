//go:build darwin

package platform

import "os"

type nativeDirectoryProvider struct{}

func (nativeDirectoryProvider) HomeDirectory() (string, error) {
	return os.UserHomeDir()
}

func (nativeDirectoryProvider) DocumentsDirectory() (string, error) {
	return "", nil
}

func (nativeDirectoryProvider) ApplicationDataDirectory() (string, error) {
	return "", nil
}
