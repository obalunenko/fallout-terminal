package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/buildtool"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build: resolve repository root: %v\n", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, root, os.Args[1:]); err != nil {
		if commandUsageError, ok := errors.AsType[*usageError](err); ok {
			fmt.Fprintf(os.Stderr, "build: %v\n", commandUsageError)
			usage()
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "build: %v\n", err)
		os.Exit(1)
	}
}

type usageError struct {
	message string
}

func (e *usageError) Error() string {
	return e.message
}

type packageAllOptions struct {
	output string
}

var inspectReleaseArchiveVersion = buildtool.InspectReleaseArchiveVersion

func run(ctx context.Context, root string, arguments []string) error {
	if len(arguments) == 0 {
		return newUsageError("missing action")
	}

	action := arguments[0]
	actionArguments := arguments[1:]
	switch action {
	case "dev":
		if containsTargetFlag(actionArguments) {
			return newUsageError("--target is supported only for build and package")
		}
		return buildtool.Run(ctx, root, action, actionArguments)
	case "prepare":
		if len(actionArguments) != 0 {
			return newUsageError("prepare does not accept arguments")
		}
		return buildtool.Run(ctx, root, action, nil)
	case "build", "package":
		target, explicit, err := parseTargetFlag(action, actionArguments)
		if err != nil {
			return err
		}
		if !explicit {
			return buildtool.Run(ctx, root, action, nil)
		}
		return buildtool.RunForTarget(ctx, root, action, target, nil)
	case "package-container":
		target, explicit, err := parseTargetFlag(action, actionArguments)
		if err != nil {
			return err
		}
		if !explicit {
			return newUsageError("package-container requires --target GOOS/GOARCH")
		}
		return buildtool.RunPortablePackageInContainer(ctx, root, target)
	case "package-all-docker":
		options, err := parsePackageAllOptions(action, actionArguments, filepath.Join("build", "dist"))
		if err != nil {
			return err
		}
		return runPackageAllDocker(ctx, root, options)
	case "validate-release-tag":
		tag, err := parseRequiredStringFlag(action, actionArguments, "tag")
		if err != nil {
			return err
		}
		version, err := buildtool.ParseReleaseTag(tag)
		if err != nil {
			return err
		}
		fmt.Println(version.Canonical)
		return nil
	case "inspect-release-archive":
		target, archive, version, err := parseReleaseArchiveOptions(action, actionArguments)
		if err != nil {
			return err
		}
		if err := inspectReleaseArchiveVersion(ctx, target, archive, version); err != nil {
			return err
		}
		fmt.Printf("==> verified release archive: %s %s version=%s\n", target, archive, version)
		return nil
	case "inspect-release-inventory":
		directory, err := parseRequiredStringFlag(action, actionArguments, "directory")
		if err != nil {
			return err
		}
		if err := buildtool.InspectReleaseInventory(ctx, directory); err != nil {
			return err
		}
		fmt.Printf("==> exact release inventory: %s\n", directory)
		return nil
	default:
		return newUsageError(
			fmt.Sprintf(
				"unknown action %q (want dev, build, package, package-all-docker, validate-release-tag, inspect-release-archive, inspect-release-inventory, or prepare)",
				action,
			),
		)
	}
}

func parseRequiredStringFlag(action string, arguments []string, name string) (string, error) {
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var value string
	flags.StringVar(&value, name, "", "required value")
	if err := flags.Parse(arguments); err != nil {
		return "", newUsageError(fmt.Sprintf("%s flags: %v", action, err))
	}
	if flags.NArg() != 0 {
		return "", newUsageError(fmt.Sprintf("%s accepts only --%s; unexpected argument %q", action, name, flags.Arg(0)))
	}
	if value == "" {
		return "", newUsageError(fmt.Sprintf("--%s must not be empty", name))
	}
	return value, nil
}

func parseReleaseArchiveOptions(action string, arguments []string) (buildtool.Target, string, string, error) {
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var rawTarget string
	var archive string
	var version string
	flags.StringVar(&rawTarget, "target", "", "exact GOOS/GOARCH release target")
	flags.StringVar(&archive, "archive", "", "release archive path")
	flags.StringVar(&version, "version", "", "expected canonical release version")
	if err := flags.Parse(arguments); err != nil {
		return buildtool.Target{}, "", "", newUsageError(fmt.Sprintf("%s flags: %v", action, err))
	}
	if flags.NArg() != 0 {
		return buildtool.Target{}, "", "", newUsageError(fmt.Sprintf("%s accepts only --target, --archive, and --version", action))
	}
	versionProvided := false
	flags.Visit(func(parsed *flag.Flag) {
		versionProvided = versionProvided || parsed.Name == "version"
	})
	if !versionProvided {
		return buildtool.Target{}, "", "", newUsageError("inspect-release-archive requires --version <canonical>")
	}
	if version == "" {
		return buildtool.Target{}, "", "", newUsageError("--version must not be empty")
	}
	if rawTarget == "" || archive == "" {
		return buildtool.Target{}, "", "", newUsageError("inspect-release-archive requires non-empty --target, --archive, and --version values")
	}
	goos, goarch, found := strings.Cut(rawTarget, "/")
	if !found || strings.Contains(goarch, "/") {
		return buildtool.Target{}, "", "", newUsageError(fmt.Sprintf("invalid --target %q (want GOOS/GOARCH)", rawTarget))
	}
	target, err := buildtool.ParseTarget(goos, goarch)
	if err != nil {
		return buildtool.Target{}, "", "", newUsageError(err.Error())
	}
	resolved, err := buildtool.ResolveBuildVersion(version)
	if err != nil || !resolved.IsRelease {
		return buildtool.Target{}, "", "", newUsageError(fmt.Sprintf("invalid --version %q: %v", version, err))
	}
	return target, archive, resolved.Canonical, nil
}

func runPackageAllDocker(ctx context.Context, root string, options packageAllOptions) error {
	result, err := buildtool.PackageAllDocker(ctx, root, options.output, func(record buildtool.LocalPackageTargetRecord) {
		if record.Failure != nil {
			fmt.Printf("==> %s: %s (%v)\n", record.Target, record.Status, record.Failure)
			return
		}
		fmt.Printf("==> %s: %s\n", record.Target, record.Status)
	})
	if err != nil {
		return err
	}

	fmt.Printf("==> source revision: %s\n", result.SourceSHA)
	fmt.Printf("==> aggregate output: %s\n", result.OutputDirectory)
	fmt.Printf("==> darwin/arm64 application: %s\n", result.DarwinBundlePath)
	for _, artifact := range result.Artifacts {
		fmt.Printf(
			"==> %s: %s (%s)\n",
			artifact.Target(),
			artifact.ArchiveName(),
			artifact.Checksum(),
		)
		fmt.Printf(
			"==> %s executable: %s\n",
			artifact.Target(),
			filepath.Join(
				result.OutputDirectory,
				"bin",
				artifact.Target().OS()+"-"+artifact.Target().Arch(),
				artifact.Target().ExecutableName(),
			),
		)
	}
	return nil
}

func parseTargetFlag(action string, arguments []string) (buildtool.Target, bool, error) {
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var rawTarget string
	flags.StringVar(&rawTarget, "target", "", "exact GOOS/GOARCH build target")
	if err := flags.Parse(arguments); err != nil {
		return buildtool.Target{}, false, newUsageError(fmt.Sprintf("%s flags: %v", action, err))
	}
	if flags.NArg() != 0 {
		return buildtool.Target{}, false, newUsageError(
			fmt.Sprintf("%s accepts only --target GOOS/GOARCH; unexpected argument %q", action, flags.Arg(0)),
		)
	}
	explicit := false
	flags.Visit(func(parsed *flag.Flag) {
		explicit = explicit || parsed.Name == "target"
	})
	if !explicit {
		return buildtool.Target{}, false, nil
	}
	if rawTarget == "" {
		return buildtool.Target{}, false, newUsageError("--target requires a non-empty GOOS/GOARCH value")
	}

	goos, goarch, found := strings.Cut(rawTarget, "/")
	if !found || strings.Contains(goarch, "/") {
		return buildtool.Target{}, false, newUsageError(
			fmt.Sprintf("invalid --target %q (want GOOS/GOARCH)", rawTarget),
		)
	}
	target, err := buildtool.ParseTarget(goos, goarch)
	if err != nil {
		return buildtool.Target{}, false, newUsageError(err.Error())
	}
	return target, true, nil
}

func parsePackageAllOptions(action string, arguments []string, defaultOutput string) (packageAllOptions, error) {
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options := packageAllOptions{output: defaultOutput}
	flags.StringVar(&options.output, "output", options.output, "aggregate artifact output directory")
	if err := flags.Parse(arguments); err != nil {
		return packageAllOptions{}, newUsageError(fmt.Sprintf("%s flags: %v", action, err))
	}
	if flags.NArg() != 0 {
		return packageAllOptions{}, newUsageError(
			fmt.Sprintf("%s accepts only --output; unexpected argument %q", action, flags.Arg(0)),
		)
	}
	if options.output == "" {
		return packageAllOptions{}, newUsageError("--output must not be empty")
	}
	return options, nil
}

func containsTargetFlag(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--target" || strings.HasPrefix(argument, "--target=") {
			return true
		}
	}
	return false
}

func newUsageError(message string) error {
	return &usageError{message: message}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build")
	fmt.Fprintln(os.Stderr, "    dev [application arguments]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build <build|package> [--target GOOS/GOARCH]")
	fmt.Fprintln(os.Stderr, "    explicit targets: windows/amd64, windows/arm64, linux/amd64, linux/arm64, darwin/arm64")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build package-container --target GOOS/GOARCH")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build prepare")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build package-all-docker [--output <directory>]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build validate-release-tag --tag <v2.MINOR.PATCH[-PRERELEASE]>")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build inspect-release-archive --target GOOS/GOARCH --archive <path> --version <canonical>")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build inspect-release-inventory --directory <path>")
}
