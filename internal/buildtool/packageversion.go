package buildtool

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const (
	packageVersionEnvironmentCanonical       = "VERSION"
	packageVersionEnvironmentNumericCore     = "VERSION_NUMERIC_CORE"
	packageVersionEnvironmentNumericFourPart = "VERSION_NUMERIC_FOUR_PART"

	packageVersionTokenCanonical       = "{{VERSION}}"
	packageVersionTokenNumericCore     = "{{NUMERIC_CORE}}"
	packageVersionTokenNumericFourPart = "{{NUMERIC_FOUR_PART}}"
)

// NativeVersionEvidence is the target-native version identity read from one
// extracted packaged executable. Darwin bundle metadata is inspected directly
// from Info.plist and therefore is not duplicated here.
type NativeVersionEvidence struct {
	ExecutableOutput    string
	ExecutableStderr    string
	FileVersion         string
	ProductVersion      string
	FixedFileVersion    string
	FixedProductVersion string
	AssemblyVersion     string
}

type nativeVersionProbe func(
	context.Context,
	Target,
	string,
	[]string,
) (NativeVersionEvidence, error)

func versionTemplateStep(name, source, destination string, version ReleaseVersion) Step {
	return Step{
		Name:        name,
		Operation:   renderTemplate,
		Source:      source,
		Destination: destination,
		Mode:        0o644,
		Environment: map[string]string{
			packageVersionEnvironmentCanonical:       version.Canonical,
			packageVersionEnvironmentNumericCore:     version.NumericCore,
			packageVersionEnvironmentNumericFourPart: version.NumericFourPart,
		},
	}
}

func validatePackageVersion(version ReleaseVersion) error {
	input := version.Canonical
	if !version.IsRelease {
		input = ""
	}
	resolved, err := ResolveBuildVersion(input)
	if err != nil {
		return err
	}
	if resolved != version {
		return fmt.Errorf("inconsistent package version representations for %q", version.Canonical)
	}
	return nil
}

func probeNativeVersion(
	ctx context.Context,
	target Target,
	executablePath string,
	arguments []string,
) (NativeVersionEvidence, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, executablePath, arguments...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	evidence := NativeVersionEvidence{}
	if err := command.Run(); err != nil {
		evidence.ExecutableOutput = stdout.String()
		evidence.ExecutableStderr = stderr.String()
		return evidence, fmt.Errorf("execute packaged application version report: %w", err)
	}
	evidence.ExecutableOutput = stdout.String()
	evidence.ExecutableStderr = stderr.String()
	if target.OS() != goosWindows {
		return evidence, nil
	}

	windowsEvidence, err := readWindowsVersionEvidence(executablePath)
	if err != nil {
		return evidence, err
	}
	windowsEvidence.ExecutableOutput = evidence.ExecutableOutput
	windowsEvidence.ExecutableStderr = evidence.ExecutableStderr
	return windowsEvidence, nil
}

func validateNativeVersionEvidence(target Target, expected ReleaseVersion, evidence NativeVersionEvidence) error {
	if evidence.ExecutableStderr != "" {
		return fmt.Errorf("packaged executable --version wrote unexpected stderr: %q", evidence.ExecutableStderr)
	}
	if evidence.ExecutableOutput != expected.Canonical+"\n" {
		return fmt.Errorf(
			"packaged executable --version mismatch: want %q, got %q",
			expected.Canonical+"\n",
			evidence.ExecutableOutput,
		)
	}
	reported, err := ResolveBuildVersion(strings.TrimSuffix(evidence.ExecutableOutput, "\n"))
	if err != nil || !reported.IsRelease || reported.Canonical != expected.Canonical {
		return fmt.Errorf("packaged executable reported an invalid release version %q", evidence.ExecutableOutput)
	}
	if target.OS() != goosWindows {
		return nil
	}

	for name, actual := range map[string]string{
		"FileVersion":    evidence.FileVersion,
		"ProductVersion": evidence.ProductVersion,
	} {
		if actual != expected.Canonical {
			return fmt.Errorf("windows %s mismatch: want %q, got %q", name, expected.Canonical, actual)
		}
	}
	for name, actual := range map[string]string{
		"fixed file version":    evidence.FixedFileVersion,
		"fixed product version": evidence.FixedProductVersion,
		"assembly version":      evidence.AssemblyVersion,
	} {
		if actual != expected.NumericFourPart {
			return fmt.Errorf("windows %s mismatch: want %q, got %q", name, expected.NumericFourPart, actual)
		}
	}
	return nil
}

func validateDarwinVersionMetadata(raw []byte, expected ReleaseVersion) error {
	values, err := parsePlistStringValues(raw)
	if err != nil {
		return err
	}
	humanReadable, found := values["CFBundleShortVersionString"]
	if !found {
		return fmt.Errorf("darwin metadata is missing CFBundleShortVersionString")
	}
	if humanReadable != expected.Canonical {
		return fmt.Errorf(
			"darwin CFBundleShortVersionString mismatch: want %q, got %q",
			expected.Canonical,
			humanReadable,
		)
	}
	numeric, found := values["CFBundleVersion"]
	if !found {
		return fmt.Errorf("darwin metadata is missing CFBundleVersion")
	}
	if numeric != expected.NumericCore {
		return fmt.Errorf("darwin CFBundleVersion mismatch: want %q, got %q", expected.NumericCore, numeric)
	}
	return nil
}

func parsePlistStringValues(raw []byte) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	values := make(map[string]string)
	pendingKey := ""
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse Darwin Info.plist: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "key":
			if err := decoder.DecodeElement(&pendingKey, &start); err != nil {
				return nil, fmt.Errorf("parse Darwin Info.plist key: %w", err)
			}
		case "string":
			if pendingKey == "" {
				continue
			}
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return nil, fmt.Errorf("parse Darwin Info.plist value for %s: %w", pendingKey, err)
			}
			if _, duplicate := values[pendingKey]; duplicate {
				return nil, fmt.Errorf("darwin Info.plist contains duplicate key %s", pendingKey)
			}
			values[pendingKey] = value
			pendingKey = ""
		}
	}
	return values, nil
}

func readWindowsVersionEvidence(executablePath string) (NativeVersionEvidence, error) {
	raw, err := os.ReadFile(executablePath)
	if err != nil {
		return NativeVersionEvidence{}, fmt.Errorf("read Windows packaged executable metadata: %w", err)
	}
	fileVersion, err := windowsUTF16VersionValue(raw, "FileVersion")
	if err != nil {
		return NativeVersionEvidence{}, err
	}
	productVersion, err := windowsUTF16VersionValue(raw, "ProductVersion")
	if err != nil {
		return NativeVersionEvidence{}, err
	}
	fixedFile, fixedProduct, err := windowsFixedVersions(raw)
	if err != nil {
		return NativeVersionEvidence{}, err
	}
	assemblyVersion, err := windowsAssemblyVersion(raw)
	if err != nil {
		return NativeVersionEvidence{}, err
	}
	return NativeVersionEvidence{
		FileVersion:         fileVersion,
		ProductVersion:      productVersion,
		FixedFileVersion:    fixedFile,
		FixedProductVersion: fixedProduct,
		AssemblyVersion:     assemblyVersion,
	}, nil
}

func windowsUTF16VersionValue(raw []byte, key string) (string, error) {
	encodedKey := encodeUTF16LE(key + "\x00")
	index := bytes.Index(raw, encodedKey)
	if index < 0 {
		return "", fmt.Errorf("windows executable is missing %s string metadata", key)
	}
	valueStart := index + len(encodedKey)
	for valueStart+1 < len(raw) && raw[valueStart] == 0 && raw[valueStart+1] == 0 {
		valueStart += 2
	}
	valueStart = (valueStart + 3) &^ 3
	units := make([]uint16, 0, 32)
	for offset := valueStart; offset+1 < len(raw); offset += 2 {
		unit := binary.LittleEndian.Uint16(raw[offset : offset+2])
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	if len(units) == 0 {
		return "", fmt.Errorf("windows executable has an empty %s string metadata value", key)
	}
	return string(utf16.Decode(units)), nil
}

func windowsFixedVersions(raw []byte) (string, string, error) {
	signature := []byte{0xbd, 0x04, 0xef, 0xfe}
	index := bytes.Index(raw, signature)
	if index < 0 || index+24 > len(raw) {
		return "", "", fmt.Errorf("windows executable is missing fixed version metadata")
	}
	fileMS := binary.LittleEndian.Uint32(raw[index+8 : index+12])
	fileLS := binary.LittleEndian.Uint32(raw[index+12 : index+16])
	productMS := binary.LittleEndian.Uint32(raw[index+16 : index+20])
	productLS := binary.LittleEndian.Uint32(raw[index+20 : index+24])
	return formatWindowsFixedVersion(fileMS, fileLS), formatWindowsFixedVersion(productMS, productLS), nil
}

func formatWindowsFixedVersion(ms, ls uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", ms>>16, ms&0xffff, ls>>16, ls&0xffff)
}

func windowsAssemblyVersion(raw []byte) (string, error) {
	for _, document := range []string{string(raw), decodeUTF16LE(raw)} {
		remaining := document
		for {
			start := strings.Index(remaining, "<assemblyIdentity")
			if start < 0 {
				break
			}
			remaining = remaining[start:]
			end := strings.IndexByte(remaining, '>')
			if end < 0 {
				break
			}
			element := remaining[:end+1]
			remaining = remaining[end+1:]
			if !strings.Contains(element, `name="com.vaulttec.fallout-terminal"`) {
				continue
			}
			const marker = `version="`
			_, after, ok := strings.Cut(element, marker)
			if !ok {
				return "", fmt.Errorf("windows application assembly identity has no version")
			}
			version := after
			versionEnd := strings.IndexByte(version, '"')
			if versionEnd < 0 || versionEnd == 0 {
				return "", fmt.Errorf("windows application assembly identity has an invalid version")
			}
			return version[:versionEnd], nil
		}
	}
	return "", fmt.Errorf("windows executable is missing the application assembly identity")
}

func encodeUTF16LE(value string) []byte {
	units := utf16.Encode([]rune(value))
	encoded := make([]byte, len(units)*2)
	for index, unit := range units {
		binary.LittleEndian.PutUint16(encoded[index*2:], unit)
	}
	return encoded
}

func decodeUTF16LE(raw []byte) string {
	units := make([]uint16, len(raw)/2)
	for index := range units {
		units[index] = binary.LittleEndian.Uint16(raw[index*2 : index*2+2])
	}
	return string(utf16.Decode(units))
}

func renderVersionTemplate(ctx context.Context, root string, step Step) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	source, err := resolvePath(root, step.Source)
	if err != nil {
		return err
	}
	destination, err := resolvePath(root, step.Destination)
	if err != nil {
		return err
	}
	if source == destination {
		return fmt.Errorf("version template destination must differ from source %q", step.Source)
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat version template %q: %w", step.Source, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("version template %q is not a regular file", step.Source)
	}
	version := ReleaseVersion{
		Canonical:       step.Environment[packageVersionEnvironmentCanonical],
		NumericCore:     step.Environment[packageVersionEnvironmentNumericCore],
		NumericFourPart: step.Environment[packageVersionEnvironmentNumericFourPart],
		IsRelease:       step.Environment[packageVersionEnvironmentCanonical] != developmentBuildVersion,
	}
	version.Prerelease = version.IsRelease && strings.Contains(version.Canonical, "-")
	if err := validatePackageVersion(version); err != nil {
		return fmt.Errorf("render version template %q: %w", step.Source, err)
	}

	template, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read version template %q: %w", step.Source, err)
	}
	rendered := string(template)
	for token, value := range map[string]string{
		packageVersionTokenCanonical:       version.Canonical,
		packageVersionTokenNumericCore:     version.NumericCore,
		packageVersionTokenNumericFourPart: version.NumericFourPart,
	} {
		rendered = strings.ReplaceAll(rendered, token, value)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create version metadata directory for %q: %w", step.Destination, err)
	}
	if err := os.WriteFile(destination, []byte(rendered), step.Mode); err != nil {
		return fmt.Errorf("write rendered version metadata %q: %w", step.Destination, err)
	}
	if err := os.Chmod(destination, step.Mode); err != nil {
		return fmt.Errorf("set rendered version metadata mode %q: %w", step.Destination, err)
	}
	return nil
}

func windowsMetadataRoot(target Target) string {
	return filepath.Join("build", "bin", target.OS()+"-"+target.Arch(), "metadata")
}

func windowsInfoPath(target Target) string {
	return filepath.Join(windowsMetadataRoot(target), "info.json")
}

func windowsManifestPath(target Target) string {
	return filepath.Join(windowsMetadataRoot(target), "app.manifest")
}
