package buildtool

import (
	"fmt"
	"runtime"
)

const (
	goosDarwin  = "darwin"
	goosLinux   = "linux"
	goosWindows = "windows"

	goarchAMD64 = "amd64"
	goarchARM64 = "arm64"
)

// ArchiveFormat identifies the portable container produced for a target.
type ArchiveFormat string

const (
	ArchiveFormatZIP     ArchiveFormat = "zip"
	ArchiveFormatTarGzip ArchiveFormat = "tar.gz"
)

// Target is an immutable, canonical desktop build destination.
//
// Callers obtain targets through ParseTarget or DefaultTarget so aliases and
// unsupported target pairs cannot enter the build graph.
type Target struct {
	goos   string
	goarch string
}

// ParseTarget validates one explicit portable target. The existing macOS
// compatibility target is intentionally available only through DefaultTarget.
func ParseTarget(goos, goarch string) (Target, error) {
	target := Target{goos: goos, goarch: goarch}
	if !isPortableOS(goos) || !isPortableArch(goarch) {
		return Target{}, fmt.Errorf(
			"unsupported target %s (want windows/arm64, windows/amd64, linux/arm64, or linux/amd64)",
			target.String(),
		)
	}
	return target, nil
}

// DefaultTarget preserves the existing no-argument macOS arm64 build path.
func DefaultTarget() Target {
	return Target{goos: goosDarwin, goarch: goarchARM64}
}

// OS returns the target's canonical GOOS value.
func (t Target) OS() string {
	return t.goos
}

// Arch returns the target's canonical GOARCH value.
func (t Target) Arch() string {
	return t.goarch
}

// String returns the canonical GOOS/GOARCH pair.
func (t Target) String() string {
	return t.goos + "/" + t.goarch
}

// Portable reports whether the target belongs to the portable archive matrix.
func (t Target) Portable() bool {
	return isPortableOS(t.goos) && isPortableArch(t.goarch)
}

// ExecutableName returns the target-native application executable name.
func (t Target) ExecutableName() string {
	if t.goos == goosWindows {
		return applicationName + ".exe"
	}
	return applicationName
}

// ArchiveFormat returns the portable archive format for the target.
func (t Target) ArchiveFormat() ArchiveFormat {
	switch t.goos {
	case goosWindows:
		return ArchiveFormatZIP
	case goosLinux:
		return ArchiveFormatTarGzip
	default:
		return ""
	}
}

// ArchiveName returns the stable portable archive filename for the target.
func (t Target) ArchiveName() string {
	if !t.Portable() {
		return ""
	}
	extension := ".tar.gz"
	if t.ArchiveFormat() == ArchiveFormatZIP {
		extension = ".zip"
	}
	return "Fallout-Terminal-" + t.goos + "-" + t.goarch + extension
}

// BuildTags returns a fresh ordered set of compile-time tags for the target.
func (t Target) BuildTags() []string {
	return []string{"production"}
}

// NativeRuntime describes the target's required native desktop stack.
func (t Target) NativeRuntime() string {
	switch t.goos {
	case goosWindows:
		return "WebView2"
	case goosLinux:
		return "GTK4/WebKitGTK 6.0 and Secret Service"
	case goosDarwin:
		return "WebKit and Apple Keychain"
	default:
		return ""
	}
}

// Host is an immutable build-host identity. It is deliberately not restricted
// to supported targets so errors can describe the actual runtime host.
type Host struct {
	goos   string
	goarch string
}

// BuildHost names Host according to the feature's build-domain terminology.
type BuildHost = Host

// NewHost constructs a build-host identity from trusted runtime values.
func NewHost(goos, goarch string) Host {
	return Host{goos: goos, goarch: goarch}
}

// RuntimeHost returns the host executing the build command.
func RuntimeHost() Host {
	return NewHost(runtime.GOOS, runtime.GOARCH)
}

// OS returns the host's GOOS value.
func (h Host) OS() string {
	return h.goos
}

// Arch returns the host's GOARCH value.
func (h Host) Arch() string {
	return h.goarch
}

// String returns the host's GOOS/GOARCH pair.
func (h Host) String() string {
	return h.goos + "/" + h.goarch
}

// ValidateHost rejects cross-compilation before a package plan can mutate its
// staging directory. Native target builds require an exact OS and architecture
// match; the legacy macOS path remains darwin/arm64 only.
func ValidateHost(target Target, host Host) error {
	if !target.valid() {
		return fmt.Errorf("invalid build target %s", target.String())
	}
	if target.goos != host.goos || target.goarch != host.goarch {
		return fmt.Errorf(
			"target %s requires a matching native host; current host is %s",
			target.String(),
			host.String(),
		)
	}
	return nil
}

func (t Target) valid() bool {
	return t.Portable() || t == DefaultTarget()
}

func isPortableOS(goos string) bool {
	return goos == goosWindows || goos == goosLinux
}

func isPortableArch(goarch string) bool {
	return goarch == goarchARM64 || goarch == goarchAMD64
}
