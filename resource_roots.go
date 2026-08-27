package main

import (
	"os"
	"path/filepath"
	"runtime"
)

func isPackagedApplication() bool {
	return packagedBuild
}

func applicationResourceRoot() string {
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		workingDirectory = ""
	}
	return applicationResourceRootFor(packagedBuild, runtime.GOOS, executable, workingDirectory)
}

func applicationResourceRootFor(packaged bool, goos, executable, workingDirectory string) string {
	if !packaged {
		if workingDirectory != "" {
			return workingDirectory
		}
		if executable != "" {
			return filepath.Dir(executable)
		}
		return ""
	}

	if executable == "" || !filepath.IsAbs(executable) {
		return ""
	}
	switch goos {
	case "darwin":
		macOSDirectory := filepath.Dir(executable)
		contentsDirectory := filepath.Dir(macOSDirectory)
		if filepath.Base(macOSDirectory) != "MacOS" || filepath.Base(contentsDirectory) != "Contents" {
			return ""
		}
		return filepath.Join(contentsDirectory, "Resources")
	case "windows", "linux":
		return filepath.Join(filepath.Dir(executable), "resources")
	default:
		return ""
	}
}
