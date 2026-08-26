//go:build darwin

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type nativeOverseerWindowCloseRegistrar interface {
	OnWindowEvent(events.WindowEventType, func(*application.WindowEvent)) func()
}

func registerNativeWindowCloseFallback(window any, requestQuit func(*application.WindowEvent)) {
	nativeWindow, ok := window.(nativeOverseerWindowCloseRegistrar)
	if !ok {
		return
	}

	// Wails v3 beta may close its final Darwin NSWindow without asking
	// NSApplication to terminate. A native/scripted close can also bypass
	// WindowShouldClose, so observe AppKit's post-close notification while
	// sharing the common hook's exactly-once quit intent.
	nativeWindow.OnWindowEvent(events.Mac.WindowWillClose, requestQuit)
}
