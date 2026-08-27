//go:build windows

package platform

import (
	"os"

	"golang.org/x/sys/windows"
)

type nativeDirectoryProvider struct{}

func (nativeDirectoryProvider) HomeDirectory() (string, error) {
	return os.UserHomeDir()
}

func (nativeDirectoryProvider) DocumentsDirectory() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DEFAULT)
}

func (nativeDirectoryProvider) ApplicationDataDirectory() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, windows.KF_FLAG_DEFAULT)
}
