// Package version owns the application identity embedded by release builds.
package version

const development = "development"

// value is intentionally unexported so release builds have one linker-owned
// assignment target without exposing mutable version state to the application.
var value = development

// Current returns the embedded application identity.
func Current() string {
	return value
}

// IsRelease reports whether the embedded identity came from a release build.
func IsRelease() bool {
	return value != development
}
