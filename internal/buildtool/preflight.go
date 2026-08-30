package buildtool

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
)

type preflightKind uint8

const (
	verifyProtobufAndGeneratedClients preflightKind = iota + 1
	verifyPlayerFrontend
	verifyOverseerFrontend
	verifyNativeBuildPrerequisites
)

var browserGeneratedContractFiles = []string{
	"hacking_pb.ts",
	"navigation_pb.ts",
	"player_pb.ts",
	"sound_pb.ts",
	"terminal_pb.ts",
}

type preflightCommand struct {
	name      string
	program   string
	arguments []string
	discard   bool
}

func executePreflight(ctx context.Context, root string, kind preflightKind, target Target) error {
	switch kind {
	case verifyProtobufAndGeneratedClients:
		return verifyGeneratedContracts(ctx, root)
	case verifyPlayerFrontend:
		return verifyFrontend(ctx, root, "Player", "client")
	case verifyOverseerFrontend:
		return verifyFrontend(ctx, root, "Overseer", "overseer")
	case verifyNativeBuildPrerequisites:
		return verifyNativePrerequisites(ctx, root, target)
	default:
		return fmt.Errorf("unsupported preflight operation %d", kind)
	}
}

func verifyGeneratedContracts(ctx context.Context, root string) error {
	environment := portablePreflightEnvironment()
	commands := []preflightCommand{
		{name: "format protobuf contracts", program: "go", arguments: []string{"tool", "-modfile=tools/buf/go.mod", "buf", "format", "proto", "--diff", "--exit-code"}},
		{name: "lint protobuf contracts", program: "go", arguments: []string{"tool", "-modfile=tools/buf/go.mod", "buf", "lint", "proto"}},
		{name: "build protobuf contracts", program: "go", arguments: []string{"tool", "-modfile=tools/buf/go.mod", "buf", "build", "proto"}, discard: true},
	}
	if err := runPreflightCommands(ctx, root, environment, commands); err != nil {
		return err
	}

	if err := verifyBrowserGeneratedContracts(root); err != nil {
		return fmt.Errorf("inspect checked-in generated browser contracts: %w", err)
	}
	checkedInRevision, err := generatedTreeRevision(root)
	if err != nil {
		return fmt.Errorf("inspect checked-in generated contracts: %w", err)
	}
	generation := []preflightCommand{
		{name: "generate Go protobuf contracts", program: "go", arguments: []string{"tool", "-modfile=tools/buf/go.mod", "buf", "generate", "--template", "proto/buf.gen.go.yaml"}},
		{name: "generate browser protobuf contracts", program: "go", arguments: []string{"tool", "-modfile=tools/buf/go.mod", "buf", "generate", "--template", "proto/buf.gen.es.yaml"}},
	}
	if err := runPreflightCommands(ctx, root, environment, generation); err != nil {
		return err
	}
	if err := verifyBrowserGeneratedContracts(root); err != nil {
		return fmt.Errorf("inspect first generated browser contracts: %w", err)
	}
	firstRevision, err := generatedTreeRevision(root)
	if err != nil {
		return fmt.Errorf("inspect first generated contracts: %w", err)
	}
	if checkedInRevision != firstRevision {
		return errors.New("checked-in generated contracts drift from a clean pinned-Buf generation")
	}
	if err := runPreflightCommands(ctx, root, environment, generation); err != nil {
		return err
	}
	if err := verifyBrowserGeneratedContracts(root); err != nil {
		return fmt.Errorf("inspect second generated browser contracts: %w", err)
	}
	secondRevision, err := generatedTreeRevision(root)
	if err != nil {
		return fmt.Errorf("inspect second generated contracts: %w", err)
	}
	if firstRevision != secondRevision {
		return errors.New("two clean protobuf generations produced different artifacts")
	}

	verification := []preflightCommand{
		{name: "compile player contract", program: "go", arguments: []string{"test", "./internal/player", "-run", "^$"}},
		{name: "test session and player configuration contracts", program: "go", arguments: []string{"test", "./internal/session", "./internal/playerconfig"}},
		{name: "test platform protobuf contracts", program: "go", arguments: []string{"test", "./internal/platform", "-run", "^TestProtobuf"}},
		{name: "test private protobuf boundaries", program: "go", arguments: []string{"test", ".", "-run", "^TestPrivate"}},
	}
	return runPreflightCommands(ctx, root, environment, verification)
}

func verifyFrontend(ctx context.Context, root, application, scriptSuffix string) error {
	commands := []preflightCommand{
		{name: "type-check " + application + " frontend", program: "npm", arguments: []string{"run", "typecheck:" + scriptSuffix, "--prefix", "frontend"}},
		{name: "build " + application + " frontend", program: "npm", arguments: []string{"run", "build:" + scriptSuffix, "--prefix", "frontend"}},
	}
	return runPreflightCommands(ctx, root, portablePreflightEnvironment(), commands)
}

func verifyBrowserGeneratedContracts(root string) error {
	directory := filepath.Join(root, "frontend", "client", "gen", "fallout", "terminal", "player", "v1")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	if !slices.Equal(files, browserGeneratedContractFiles) {
		return fmt.Errorf("generated browser contract inventory is %q, want %q", files, browserGeneratedContractFiles)
	}
	return nil
}

func verifyNativePrerequisites(ctx context.Context, root string, target Target) error {
	if !target.Portable() {
		return fmt.Errorf("native portable preflight requires a portable target, got %s", target)
	}
	if target.OS() != goosLinux {
		return nil
	}
	if _, err := exec.LookPath("pkg-config"); err != nil {
		return fmt.Errorf("%s requires pkg-config, GTK4, and WebKitGTK 6.0 development packages: %w", target, err)
	}
	command := preflightCommand{
		name:      "resolve Linux desktop development packages",
		program:   "pkg-config",
		arguments: []string{"--exists", "gtk4", "webkitgtk-6.0"},
	}
	if err := runPreflightCommands(ctx, root, portablePreflightEnvironment(), []preflightCommand{command}); err != nil {
		return fmt.Errorf("%s requires GTK4 and WebKitGTK 6.0 development packages: %w", target, err)
	}
	return nil
}

func runPreflightCommands(ctx context.Context, root string, environment []string, commands []preflightCommand) error {
	for _, command := range commands {
		process := exec.CommandContext(ctx, command.program, command.arguments...)
		process.Dir = root
		process.Env = environment
		process.Stdin = os.Stdin
		if command.discard {
			process.Stdout = io.Discard
		} else {
			process.Stdout = os.Stdout
		}
		process.Stderr = os.Stderr
		if err := process.Run(); err != nil {
			return fmt.Errorf("%s: %w", command.name, err)
		}
	}
	return nil
}

func portablePreflightEnvironment() []string {
	base := environmentWithout(os.Environ(), map[string]struct{}{
		"CGO_ENABLED":              {},
		"CGO_CFLAGS":               {},
		"CGO_LDFLAGS":              {},
		"GOARCH":                   {},
		"GOOS":                     {},
		"MACOSX_DEPLOYMENT_TARGET": {},
	})
	overrides := make(map[string]string, 5)
	if os.Getenv("BUF_CACHE_DIR") == "" {
		overrides["BUF_CACHE_DIR"] = filepath.Join(os.TempDir(), "fallout-terminal-buf-cache")
	}
	if os.Getenv("GOCACHE") == "" {
		overrides["GOCACHE"] = filepath.Join(os.TempDir(), "fallout-terminal-go-cache")
	}
	if runtime.GOOS == goosDarwin {
		overrides["MACOSX_DEPLOYMENT_TARGET"] = environmentOrDefault("MACOSX_DEPLOYMENT_TARGET", minimumMacOS)
		overrides["CGO_CFLAGS"] = environmentOrDefault("CGO_CFLAGS", "-mmacosx-version-min="+overrides["MACOSX_DEPLOYMENT_TARGET"])
		overrides["CGO_LDFLAGS"] = environmentOrDefault("CGO_LDFLAGS", macOSCGOLinkerFlags(overrides["MACOSX_DEPLOYMENT_TARGET"]))
	}
	return mergeEnvironment(base, overrides)
}

func environmentWithout(environment []string, excluded map[string]struct{}) []string {
	filtered := make([]string, 0, len(environment))
	for _, item := range environment {
		key, _, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		if _, remove := excluded[key]; remove {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func environmentOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func generatedTreeRevision(root string) ([sha256.Size]byte, error) {
	files := make([]string, 0)
	for _, tree := range []string{filepath.Join(root, "internal", "gen"), filepath.Join(root, "frontend", "client", "gen")} {
		err := filepath.WalkDir(tree, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type().IsRegular() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return [sha256.Size]byte{}, err
		}
	}
	sort.Strings(files)
	digest := sha256.New()
	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		fileDigest := sha256.Sum256(contents)
		if _, err := fmt.Fprintf(digest, "%x  %s\n", fileDigest, filepath.ToSlash(relative)); err != nil {
			return [sha256.Size]byte{}, err
		}
	}
	var revision [sha256.Size]byte
	copy(revision[:], digest.Sum(nil))
	return revision, nil
}
