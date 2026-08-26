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

	"github.com/obalunenko/Fallout-Terminal/internal/buildtool"
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
		var commandUsageError *usageError
		if errors.As(err, &commandUsageError) {
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

func run(ctx context.Context, root string, arguments []string) error {
	if len(arguments) == 0 {
		return newUsageError("missing action")
	}

	action := arguments[0]
	actionArguments := arguments[1:]
	switch action {
	case "dev", "run":
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
		options, err := parsePackageAllOptions(actionArguments)
		if err != nil {
			return err
		}
		return runPackageAllDocker(ctx, root, options)
	case "package-all":
		options, err := parsePackageAllOptions(actionArguments)
		if err != nil {
			return err
		}
		return runPackageAll(ctx, root, options)
	default:
		return newUsageError(
			fmt.Sprintf(
				"unknown action %q (want dev, build, package, package-all-docker, package-all, run, or prepare)",
				action,
			),
		)
	}
}

func runPackageAllDocker(ctx context.Context, root string, options packageAllOptions) error {
	result, err := buildtool.PackageAllDocker(ctx, root, options.output, func(record buildtool.AggregateTargetRecord) {
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
	for _, artifact := range result.Artifacts {
		fmt.Printf(
			"==> %s: %s (%s)\n",
			artifact.Target(),
			artifact.ArchiveName(),
			artifact.Checksum(),
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

func parsePackageAllOptions(arguments []string) (packageAllOptions, error) {
	flags := flag.NewFlagSet("package-all", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	options := packageAllOptions{output: filepath.Join("build", "dist")}
	flags.StringVar(&options.output, "output", options.output, "aggregate artifact output directory")
	if err := flags.Parse(arguments); err != nil {
		return packageAllOptions{}, newUsageError(fmt.Sprintf("package-all flags: %v", err))
	}
	if flags.NArg() != 0 {
		return packageAllOptions{}, newUsageError(
			fmt.Sprintf("package-all accepts only --output; unexpected argument %q", flags.Arg(0)),
		)
	}
	if options.output == "" {
		return packageAllOptions{}, newUsageError("--output must not be empty")
	}
	return options, nil
}

func runPackageAll(ctx context.Context, root string, options packageAllOptions) error {
	result, err := buildtool.PackageAll(ctx, root, options.output, func(record buildtool.AggregateTargetRecord) {
		if record.Failure != nil {
			fmt.Printf("==> %s: %s (%v)\n", record.Target, record.Status, record.Failure)
			return
		}
		fmt.Printf("==> %s: %s\n", record.Target, record.Status)
	})
	if err != nil {
		if result.RunURL != "" {
			fmt.Printf("==> workflow: %s\n", result.RunURL)
		}
		return err
	}

	fmt.Printf("==> workflow: %s\n", result.RunURL)
	fmt.Printf("==> source revision: %s\n", result.SourceSHA)
	fmt.Printf("==> aggregate output: %s\n", result.OutputDirectory)
	for _, artifact := range result.Artifacts {
		fmt.Printf(
			"==> %s: %s (%s)\n",
			artifact.Target(),
			artifact.ArchiveName(),
			artifact.Checksum(),
		)
	}
	return nil
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
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build <dev|run> [application arguments]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build <build|package> [--target GOOS/GOARCH]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build package-container --target GOOS/GOARCH")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build prepare")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build package-all-docker [--output <directory>]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/build package-all [--output <directory>]")
}
