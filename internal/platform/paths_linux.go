//go:build linux

package platform

import (
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type nativeDirectoryProvider struct{}

func (nativeDirectoryProvider) HomeDirectory() (string, error) {
	return os.UserHomeDir()
}

func (nativeDirectoryProvider) DocumentsDirectory() (string, error) {
	return application.Path(application.PathDocuments), nil
}

func (nativeDirectoryProvider) ApplicationDataDirectory() (string, error) {
	return application.Path(application.PathConfigHome), nil
}
