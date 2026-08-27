//go:build !darwin

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func registerNativeWindowCloseFallback(_ any, _ func(*application.WindowEvent)) {}
