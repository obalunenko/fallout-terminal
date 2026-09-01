package platform

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	_ "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/config/v1"
	_ "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/persistence/v1"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	privatev1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/private/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/testing/prototest"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

var browserFileDescriptorPattern = regexp.MustCompile(`fileDesc\("([A-Za-z0-9+/=]+)"(?:, \[([^]]*)\])?\)`)

func TestCanonicalFrontendWorkspaceLayout(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	assertNonEmptyFiles(t, root, []string{
		"frontend/package.json",
		"frontend/package-lock.json",
		"frontend/client/package.json",
		"frontend/client/index.html",
		"frontend/client/src/main.ts",
		"frontend/client/src/mount.ts",
		"frontend/client/src/App.vue",
		"frontend/client/src/adapters/player-rpc.ts",
		"frontend/client/client.css",
		"frontend/client/fonts/Fixedsys.ttf",
		"frontend/overseer/package.json",
		"frontend/overseer/src/index.html",
		"frontend/overseer/src/main.ts",
		"frontend/overseer/src/mount.ts",
		"frontend/overseer/src/App.vue",
		"frontend/overseer/src/controllers/overseer-controller.ts",
		"frontend/overseer/src/adapters/desktop-api.ts",
		"frontend/overseer/src/overseer.css",
		"frontend/overseer/src/Fixedsys.ttf",
	})
	for _, removed := range []string{
		"frontend/client/client.js",
		"frontend/client/sound.js",
		"frontend/client/presentation-uplink.js",
		"frontend/client/test-fixtures/index.html",
		"frontend/client/test-fixtures/candidate-main.ts",
		"frontend/client/tsconfig.legacy.json",
		"frontend/overseer/src/overseer.js",
		"frontend/overseer/src/desktop-api.js",
		"frontend/overseer/test-fixtures/index.html",
		"frontend/overseer/test-fixtures/candidate-main.ts",
	} {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(removed)))
		assert.ErrorIs(t, err, os.ErrNotExist, removed)
	}
	_, err := os.Stat(filepath.Join(root, "client"))
	assert.True(t, errors.Is(err, os.ErrNotExist), "top-level client application must be removed")
}

func TestProtobufContractShapeAndSeparation(t *testing.T) {
	t.Parallel()

	service := playerv1.File_fallout_terminal_player_v1_player_proto.Services().ByName("PlayerService")
	require.False(t, service == nil,
		"public descriptor is missing PlayerService")

	wantMethods := []string{"Subscribe", "SelectCharacter", "Navigate", "Guess", "ActivatePattern", "SetPresentation", "PresentationUplink", "SoundManifest"}
	require.Falsef(t, service.Methods().Len() != len(wantMethods),
		"PlayerService methods = %d, want %d", service.Methods().Len(), len(wantMethods))

	for index, want := range wantMethods {
		method := service.Methods().Get(index)
		{
			got := string(method.Name())
			assert.Falsef(t, got != want,
				"PlayerService method %d = %q, want %q", index, got, want)
		}

		switch want {
		case "Subscribe":
			assert.Falsef(t, method.IsStreamingClient() || !method.IsStreamingServer(),
				"Subscribe must be server-streaming only")
		case "PresentationUplink":
			assert.Falsef(t, !method.IsStreamingClient() || method.IsStreamingServer(),
				"PresentationUplink must be client-streaming only")
		default:
			assert.Falsef(t, method.IsStreamingClient() || method.IsStreamingServer(),
				"%s must be unary", want)
		}

	}

	var publicFiles []protoreflect.FileDescriptor
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if strings.HasPrefix(string(file.Package()), "fallout.terminal.player.v1") {
			publicFiles = append(publicFiles, file)
		}
		return true
	})
	require.False(t, len(publicFiles) == 0,
		"generated public descriptor graph is empty")

	for _, file := range publicFiles {
		imports := file.Imports()
		for index := 0; index < imports.Len(); index++ {
			imported := imports.Get(index)
			assert.Falsef(t, !strings.HasPrefix(imported.Path(), "fallout/terminal/player/v1/"),
				"public descriptor %s imports non-public schema %s", file.Path(), imported.Path())

		}
	}

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(file.Package()), "fallout.terminal.") {
			return true
		}
		checkEnumZeroValues(t, file.Enums())
		checkMessageContractShape(t, file.Messages())
		return true
	})

	optionalFields := map[protoreflect.FullName]bool{
		"fallout.terminal.player.v1.PlayerState.broadcast_id":                          true,
		"fallout.terminal.player.v1.PlayerState.active_terminal_id":                    true,
		"fallout.terminal.player.v1.SubscribeRequest.recognition_handle":               true,
		"fallout.terminal.player.v1.SubscribeRequest.client_instance_id":               true,
		"fallout.terminal.player.v1.NavigationState.view_entry_id":                     true,
		"fallout.terminal.player.v1.NavigationState.command_node_id":                   true,
		"fallout.terminal.persistence.v1.Session.player_config":                        true,
		"fallout.terminal.config.v1.PublicAccessPreferences.reserved_domain":           true,
		"fallout.terminal.private.v1.CharacterState.logical_session_id":                true,
		"fallout.terminal.private.v1.LogicalSessionState.character_id":                 true,
		"fallout.terminal.private.v1.BroadcastState.active_controller_session_id":      true,
		"fallout.terminal.private.v1.BroadcastState.active_terminal_id":                true,
		"fallout.terminal.private.v1.PublicAccessStatus.public_url":                    true,
		"fallout.terminal.private.v1.PublicAccessStatus.error_message":                 true,
		"fallout.terminal.private.v1.PublicAccessCommandResult.error":                  true,
		"fallout.terminal.private.v1.SavePublicAccessSettingsRequest.reserved_domain":  true,
		"fallout.terminal.private.v1.GeneratedPlayerPasswordResult.error":              true,
		"fallout.terminal.private.v1.GeneratedPlayerPasswordResult.generated_password": true,
	}
	for name := range optionalFields {
		descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
		if err != nil {
			assert.Failf(t, "assertion failed", "semantic optional field %s missing: %v", name, err)
			continue
		}
		field, ok := descriptor.(protoreflect.FieldDescriptor)
		assert.Falsef(t, !ok || !field.HasOptionalKeyword(),
			"%s must use proto3 optional presence", name)

	}

	for _, name := range []protoreflect.FullName{
		"fallout.terminal.player.v1.SubscriptionMessage.payload",
		"fallout.terminal.player.v1.NavigateRequest.action",
		"fallout.terminal.player.v1.GuessRequest.target",
		"fallout.terminal.player.v1.ContentNode.content",
		"fallout.terminal.player.v1.TerminalPresentation.presentation",
		"fallout.terminal.persistence.v1.ContentNode.content",
		"fallout.terminal.private.v1.SavePublicAccessSettingsRequest.provider_token_change",
		"fallout.terminal.private.v1.SavePublicAccessSettingsRequest.player_password_change",
	} {
		descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
		if err != nil {
			assert.Failf(t, "assertion failed", "required oneof %s missing: %v", name, err)
			continue
		}
		{
			_, ok := descriptor.(protoreflect.OneofDescriptor)
			assert.Falsef(t, !ok,
				"%s is not a oneof descriptor", name)
		}

	}
}

func TestAssetsGeneratedProtobufIdentityChangesOnlyGoPackageForV2(t *testing.T) {
	t.Parallel()

	type contractFile struct {
		packageName string
		goPackage   string
		browserFile string
	}

	const module = "github.com/obalunenko/Fallout-Terminal/v2"
	contract := func(area, alias, browserFile string) contractFile {
		return contractFile{
			packageName: "fallout.terminal." + area + ".v1",
			goPackage:   module + "/internal/gen/fallout/terminal/" + area + "/v1;" + alias,
			browserFile: browserFile,
		}
	}
	contractFiles := map[string]contractFile{
		"fallout/terminal/config/v1/config.proto":             contract("config", "configv1", ""),
		"fallout/terminal/config/v1/public_access.proto":      contract("config", "configv1", ""),
		"fallout/terminal/persistence/v1/player_config.proto": contract("persistence", "persistencev1", ""),
		"fallout/terminal/persistence/v1/session.proto":       contract("persistence", "persistencev1", ""),
		"fallout/terminal/player/v1/hacking.proto":            contract("player", "playerv1", "hacking_pb.ts"),
		"fallout/terminal/player/v1/navigation.proto":         contract("player", "playerv1", "navigation_pb.ts"),
		"fallout/terminal/player/v1/player.proto":             contract("player", "playerv1", "player_pb.ts"),
		"fallout/terminal/player/v1/sound.proto":              contract("player", "playerv1", "sound_pb.ts"),
		"fallout/terminal/player/v1/terminal.proto":           contract("player", "playerv1", "terminal_pb.ts"),
		"fallout/terminal/private/v1/coordination.proto":      contract("private", "privatev1", ""),
		"fallout/terminal/private/v1/desktop.proto":           contract("private", "privatev1", ""),
		"fallout/terminal/private/v1/public_access.proto":     contract("private", "privatev1", ""),
		"fallout/terminal/private/v1/runtime.proto":           contract("private", "privatev1", ""),
		"fallout/terminal/private/v1/update.proto":            contract("private", "privatev1", ""),
	}

	descriptors := make(map[string]protoreflect.FileDescriptor)
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if strings.HasPrefix(file.Path(), "fallout/terminal/") {
			descriptors[file.Path()] = file
		}
		return true
	})
	require.Equal(t, sortedMapKeys(contractFiles), sortedMapKeys(descriptors),
		"generated protobuf file inventory changed")

	root := assetRepositoryRoot(t)
	packagePattern := regexp.MustCompile(`(?m)^package ([^;]+);$`)
	goPackagePattern := regexp.MustCompile(`(?m)^option go_package = "([^"]+)";$`)
	descriptorHash := sha256.New()
	wantBrowserFiles := make([]string, 0, 5)
	for _, path := range sortedMapKeys(contractFiles) {
		contract := contractFiles[path]
		descriptor := descriptors[path]
		require.Equal(t, protoreflect.FullName(contract.packageName), descriptor.Package(), path)

		source, err := os.ReadFile(filepath.Join(root, "proto", filepath.FromSlash(path)))
		require.NoError(t, err)
		packageMatches := packagePattern.FindAllSubmatch(source, -1)
		require.Len(t, packageMatches, 1, "%s must declare exactly one protobuf package", path)
		require.Equal(t, contract.packageName, string(packageMatches[0][1]), path)
		matches := goPackagePattern.FindAllSubmatch(source, -1)
		require.Len(t, matches, 1, "%s must declare exactly one go_package option", path)
		require.Equal(t, contract.goPackage, string(matches[0][1]), path)

		canonical := protodesc.ToFileDescriptorProto(descriptor)
		require.Equal(t, contract.goPackage, canonical.GetOptions().GetGoPackage(), path)
		wantBrowserDependencies := make([]string, 0, len(canonical.Dependency))
		for _, dependency := range canonical.Dependency {
			wantBrowserDependencies = append(wantBrowserDependencies, browserDescriptorName(dependency))
		}
		canonical.Options.GoPackage = nil
		canonical.SourceCodeInfo = nil
		encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(canonical)
		require.NoError(t, err)
		descriptorHash.Write([]byte(path))
		descriptorHash.Write([]byte{0})
		descriptorHash.Write(encoded)

		if contract.browserFile == "" {
			continue
		}
		wantBrowserFiles = append(wantBrowserFiles, contract.browserFile)
		browser, browserDependencies := readBrowserFileDescriptor(t, root, contract.browserFile)
		require.Equal(t, path, browser.GetName(), contract.browserFile)
		require.Equal(t, contract.packageName, browser.GetPackage(), contract.browserFile)
		require.Equal(t, contract.goPackage, browser.GetOptions().GetGoPackage(), contract.browserFile)
		require.Equal(t, wantBrowserDependencies, browserDependencies, contract.browserFile)
		browserComparable := proto.Clone(canonical).(*descriptorpb.FileDescriptorProto)
		normalizeBrowserDescriptor(browserComparable)
		normalizeBrowserDescriptor(browser)
		require.Empty(t, cmp.Diff(browserComparable, browser, protocmp.Transform()),
			"browser descriptor %s diverged from the Go descriptor", contract.browserFile)
	}

	const stableDescriptorShape = "c67aeab7ab4e245ef42987e5b1348082304c0d4395aeaebbd9b3765d710e4e02"
	require.Equal(t, stableDescriptorShape, hex.EncodeToString(descriptorHash.Sum(nil)),
		"protobuf packages, fields, services, or RPC directions changed")
	sort.Strings(wantBrowserFiles)
	generatedDirectory := filepath.Join(root, "frontend", "client", "gen", "fallout", "terminal", "player", "v1")
	gotBrowserPaths, err := filepath.Glob(filepath.Join(generatedDirectory, "*_pb.ts"))
	require.NoError(t, err)
	gotBrowserFiles := make([]string, 0, len(gotBrowserPaths))
	for _, path := range gotBrowserPaths {
		gotBrowserFiles = append(gotBrowserFiles, filepath.Base(path))
	}
	require.Equal(t, wantBrowserFiles, gotBrowserFiles, "browser descriptor inventory changed")
	staleJavaScript, err := filepath.Glob(filepath.Join(generatedDirectory, "*_pb.js"))
	require.NoError(t, err)
	require.Empty(t, staleJavaScript, "stale generated JavaScript must not remain beside TypeScript contracts")
}

func sortedMapKeys[T any](files map[string]T) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func readBrowserFileDescriptor(t *testing.T, root, name string) (*descriptorpb.FileDescriptorProto, []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "frontend", "client", "gen", "fallout", "terminal", "player", "v1", name))
	require.NoError(t, err)
	match := browserFileDescriptorPattern.FindSubmatch(raw)
	require.Len(t, match, 3, "%s is missing its encoded file descriptor", name)
	encoded, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(string(match[1]), "="))
	require.NoError(t, err)
	descriptor := &descriptorpb.FileDescriptorProto{}
	require.NoError(t, proto.Unmarshal(encoded, descriptor))
	dependencies := make([]string, 0)
	if rawDependencies := strings.TrimSpace(string(match[2])); rawDependencies != "" {
		for dependency := range strings.SplitSeq(rawDependencies, ",") {
			dependencies = append(dependencies, strings.TrimSpace(dependency))
		}
	}
	return descriptor, dependencies
}

func browserDescriptorName(path string) string {
	path = strings.TrimSuffix(path, ".proto")
	return "file_" + strings.ReplaceAll(path, "/", "_")
}

func normalizeBrowserDescriptor(descriptor *descriptorpb.FileDescriptorProto) {
	descriptor.Options.GoPackage = nil
	descriptor.SourceCodeInfo = nil
	descriptor.Dependency = nil
	descriptor.PublicDependency = nil
	descriptor.WeakDependency = nil
	for _, message := range descriptor.MessageType {
		normalizeMessageDescriptor(message)
	}
	for _, extension := range descriptor.Extension {
		extension.JsonName = nil
	}
}

func normalizeMessageDescriptor(message *descriptorpb.DescriptorProto) {
	for _, field := range message.Field {
		field.JsonName = nil
	}
	for _, extension := range message.Extension {
		extension.JsonName = nil
	}
	for _, nested := range message.NestedType {
		normalizeMessageDescriptor(nested)
	}
}

func TestProtobufSchemaRevisionMatchesSources(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "proto", "fallout", "terminal", "*", "v1", "*.proto"))
	if err != nil {
		require.NoError(t, err)
	}
	sort.Strings(paths)
	outer := sha256.New()
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			require.NoError(t, err)
		}
		inner := sha256.Sum256(raw)
		relative, err := filepath.Rel(root, path)
		if err != nil {
			require.NoError(t, err)
		}
		outer.Write([]byte(hex.EncodeToString(inner[:]) + "  " + filepath.ToSlash(relative) + "\n"))
	}
	wantRaw, err := os.ReadFile(filepath.Join(root, "proto", "schema-revision.txt"))
	if err != nil {
		require.NoError(t, err)
	}
	{
		got, want := hex.EncodeToString(outer.Sum(nil)), strings.TrimSpace(string(wantRaw))
		require.Falsef(t, got != want,
			"schema revision = %s, want %s; run scripts/proto-generate.sh after schema edits", got, want)
	}

}

func TestBundledDemoShowcasesEveryCommandBehavior(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "sessions", "demo.json"))
	require.NoError(t, err)
	demo, err := domain.DecodeSession(raw)
	require.NoError(t, err)

	ordinaryCommands := 0
	stateChangingCommands := 0
	completedCommandStates := 0
	terminalTransitions := 0
	for _, terminal := range demo.Terminals {
		var visit func(domain.ContentNode)
		visit = func(node domain.ContentNode) {
			if node.Type == domain.NodeCommand {
				switch {
				case node.StateChange != nil:
					stateChangingCommands++
					if _, completed := terminal.CommandStates[node.ID]; completed {
						completedCommandStates++
					}
				case node.TerminalTransition != nil:
					terminalTransitions++
				default:
					ordinaryCommands++
				}
			}
			for _, child := range node.Children {
				visit(child)
			}
		}
		visit(terminal.Root)
	}
	require.GreaterOrEqual(t, ordinaryCommands, 1,
		"bundled demo must showcase an ordinary command")
	require.GreaterOrEqual(t, stateChangingCommands, 1,
		"bundled demo must showcase a state-changing command")
	require.GreaterOrEqual(t, completedCommandStates, 1,
		"bundled demo must showcase a completed command that can be reset in its writable copy")
	require.GreaterOrEqual(t, terminalTransitions, 1,
		"bundled demo must showcase cross-terminal navigation")
}

func TestWailsMigrationRuntimeStatusContractIsFrozen(t *testing.T) {
	t.Parallel()

	descriptor := (&privatev1.RuntimeStatus{}).ProtoReflect().Descriptor()
	wantFields := []string{
		"server_info", "client_count", "hack_state", "startup_error",
		"save_state", "requested_revision", "saved_revision", "coordination_state",
	}
	gotFields := make([]string, 0, descriptor.Fields().Len())
	for index := range descriptor.Fields().Len() {
		gotFields = append(gotFields, string(descriptor.Fields().Get(index).Name()))
	}
	require.Equal(t, wantFields, gotFields)
	require.Nil(t, descriptor.Fields().ByName("phase"))
	require.Zero(t, descriptor.ParentFile().Enums().Len())

	root := assetRepositoryRoot(t)
	wantDigests := map[string]string{
		"proto/fallout/terminal/private/v1/runtime.proto": "6d137c97b08cfe2992bacb1b0f080192fc5051af3c54128920991bedd29f0e54",
		"proto/schema-revision.txt":                       "4dbbf2c119511e08aa10374ef97948518460b95bd687568bedfaf001bd4e8212",
		"proto/compatibility-baseline.binpb":              "b0004a0b4dbfabd1b6cce0c183b7b42a3f104261b1c047fc5d2ebe40932be3a7",
	}
	for relative, want := range wantDigests {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		require.NoError(t, err)
		got := sha256.Sum256(raw)
		require.Equal(t, want, hex.EncodeToString(got[:]), relative)
	}
}

func checkEnumZeroValues(t *testing.T, enums protoreflect.EnumDescriptors) {
	t.Helper()
	for index := 0; index < enums.Len(); index++ {
		enum := enums.Get(index)
		assert.Falsef(t, enum.Values().Len() == 0 || enum.Values().Get(0).Number() != 0 || !strings.HasSuffix(string(enum.Values().Get(0).Name()), "_UNSPECIFIED"),
			"enum %s must define an UNSPECIFIED zero value", enum.FullName())

	}
}

func checkMessageContractShape(t *testing.T, messages protoreflect.MessageDescriptors) {
	t.Helper()
	for index := 0; index < messages.Len(); index++ {
		message := messages.Get(index)
		prototest.Message{}.Test(t, dynamicpb.NewMessageType(message))
		checkEnumZeroValues(t, message.Enums())
		checkMessageContractShape(t, message.Messages())
		fields := message.Fields()
		for fieldIndex := 0; fieldIndex < fields.Len(); fieldIndex++ {
			field := fields.Get(fieldIndex)
			assert.Falsef(t, message.ReservedNames().Has(field.Name()) || message.ReservedRanges().Has(field.Number()),
				"active field %s collides with a reserved identifier", field.FullName())

		}
	}
}

func TestAssetsOverseerVueManifestSupportsCleanCheckoutAndBuiltOutput(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	assertNonEmptyFiles(t, root, []string{
		"frontend/overseer/src/index.html",
		"frontend/overseer/src/main.ts",
		"frontend/overseer/src/mount.ts",
		"frontend/overseer/src/App.vue",
		"frontend/overseer/src/controllers/overseer-controller.ts",
		"frontend/overseer/src/adapters/desktop-api.ts",
		"frontend/overseer/src/overseer.css",
		"frontend/overseer/src/Fixedsys.ttf",
	})

	distRoot := filepath.Join(root, "frontend", "overseer", "dist")
	{
		info, err := os.Stat(filepath.Join(distRoot, ".keep"))
		require.Falsef(t, err != nil || info.IsDir(),
			"frontend/overseer/dist/.keep must preserve the go:embed root on a clean checkout: %v", err)
	}

	builtFiles := make([]string, 0)
	err := filepath.WalkDir(distRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() == ".keep" {
			return nil
		}
		relative, err := filepath.Rel(distRoot, path)
		if err != nil {
			return err
		}
		builtFiles = append(builtFiles, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		require.NoError(t, err)
	}
	if len(builtFiles) == 0 {
		return
	}

	assertNonEmptyFiles(t, distRoot, []string{"index.html"})
	for extension, description := range map[string]string{
		".js":  "JavaScript bundle",
		".css": "stylesheet bundle",
		".ttf": "Fixedsys font bundle",
	} {
		assert.Falsef(t, !containsExtension(builtFiles, extension),
			"built overseer output is missing a %s (%s); files: %v", description, extension, builtFiles)
	}

	index, err := os.ReadFile(filepath.Join(distRoot, "index.html"))
	require.NoError(t, err)
	document := string(index)
	assert.Equal(t, 1, strings.Count(document, `<div id="overseerApp"></div>`))
	assert.Contains(t, document, `type="module"`)
	assert.Contains(t, document, `./assets/`)

	for _, relative := range builtFiles {
		extension := strings.ToLower(filepath.Ext(relative))
		assert.NotContains(t, []string{".ts", ".vue", ".map"}, extension, relative)
		for _, forbidden := range []string{"candidate-main", "test-fixtures", "overseer.js", "desktop-api.js"} {
			assert.NotContains(t, relative, forbidden)
		}
		if extension != ".html" && extension != ".js" && extension != ".css" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(distRoot, filepath.FromSlash(relative)))
		require.NoError(t, err)
		for _, forbidden := range []string{"candidate-main", "test-fixtures", "legacyOverseerRoot", "overseerVueLeaves", "overseer.js", "desktop-api.js"} {
			assert.NotContains(t, string(raw), forbidden, relative)
		}
	}

	sourceFont, err := os.ReadFile(filepath.Join(root, "frontend", "overseer", "src", "Fixedsys.ttf"))
	require.NoError(t, err)
	builtFonts, err := filepath.Glob(filepath.Join(distRoot, "assets", "*.ttf"))
	require.NoError(t, err)
	require.Len(t, builtFonts, 1)
	builtFont, err := os.ReadFile(builtFonts[0])
	require.NoError(t, err)
	assert.Equal(t, sha256.Sum256(sourceFont), sha256.Sum256(builtFont), "built Overseer font differs from the production source asset")
}

func TestPlayerVueSourceAndSoundManifest(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	assertNonEmptyFiles(t, root, []string{
		"frontend/client/index.html",
		"frontend/client/client.css",
		"frontend/client/src/main.ts",
		"frontend/client/src/mount.ts",
		"frontend/client/src/App.vue",
		"frontend/client/src/adapters/player-rpc.ts",
		"frontend/client/src/adapters/sound-manifest.ts",
		"frontend/client/fonts/Fixedsys.ttf",
	})

	requiredCategories := []string{
		"ambient",
		"charscroll",
		"enter",
		"hack-bad",
		"hack-good",
		"menu-focus",
		"multiple",
		"single",
	}
	allowedExtensions := map[string]struct{}{
		".mp3": {}, ".wav": {}, ".ogg": {}, ".m4a": {}, ".webm": {},
	}
	for _, category := range requiredCategories {
		t.Run(category, func(t *testing.T) {
			directory := filepath.Join(root, "frontend", "client", "sounds", category)
			entries, err := os.ReadDir(directory)
			require.Falsef(t, err != nil,
				"read required sound category %q: %v", category, err)

			files := make([]string, 0)
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if _, allowed := allowedExtensions[strings.ToLower(filepath.Ext(entry.Name()))]; !allowed {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					require.NoError(t, err)
				}
				if info.Size() > 0 {
					files = append(files, entry.Name())
				}
			}
			sort.Strings(files)
			require.Falsef(t, len(files) == 0,
				"required sound category %q has no non-empty supported audio asset", category)

		})
	}
}

func TestPackagedCompositionIgnoresExactDevelopmentPublicAccessEnvironment(t *testing.T) {
	t.Parallel()
	root := assetRepositoryRoot(t)
	mainSource, err := os.ReadFile(filepath.Join(root, "main.go"))
	require.NoError(t, err)
	overrideSource, err := os.ReadFile(filepath.Join(root, "internal", "tunnel", "test_override.go"))
	require.NoError(t, err)

	active := string(mainSource) + "\n" + string(overrideSource)
	for _, name := range []string{
		"FALLOUT_NGROK_AUTHTOKEN", "FALLOUT_NGROK_RESERVED_DOMAIN",
		"FALLOUT_PUBLIC_TEST_USERNAME", "FALLOUT_PUBLIC_TEST_PASSWORD",
	} {
		assert.Contains(t, active, name)
	}
	assert.Contains(t, string(mainSource), "publicAccessStoresForProfile(publicSettings, publicSecrets, packaged, os.LookupEnv)")
	assert.Contains(t, string(mainSource), "packaged := isPackagedApplication()")
	assert.Contains(t, string(mainSource), "if packaged")
	assert.NotContains(t, string(overrideSource), "os.Environ")
	assert.NotContains(t, active, `"NGROK_AUTHTOKEN"`)
}

func TestBundledDemoManifestIsValidAndResolvesFromResources(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	demoPath := filepath.Join(root, "sessions", "demo.json")
	raw, err := os.ReadFile(demoPath)
	if err != nil {
		require.NoError(t, err)
	}
	session, err := domain.DecodeSession(raw)
	require.Falsef(t, err != nil,
		"bundled demo is not a valid version-1 session: %v", err)
	require.Falsef(t, session.Version != 1 || len(session.Terminals) == 0,
		"bundled demo = version %d with %d terminals, want version 1 with content", session.Version, len(session.Terminals))
	require.Equal(t, "demo-players.json", session.PlayerConfig)
	playerConfigRaw, err := os.ReadFile(filepath.Join(filepath.Dir(demoPath), session.PlayerConfig))
	require.NoError(t, err)
	playerConfig, err := domain.DecodePlayerConfig(playerConfigRaw)
	require.NoError(t, err)
	require.Equal(t, 1, playerConfig.Version)
	require.Equal(t, []domain.CharacterRosterEntry{
		{ID: "demo_scout", Name: "Пайпер Райт", Intelligence: 7, HackerPerkAvailable: false},
		{ID: "demo_technician", Name: "Ник Валентайн", Intelligence: 8, HackerPerkAvailable: true},
		{ID: "demo_medic", Name: "Кюри", Intelligence: 10, HackerPerkAvailable: true},
		{ID: "demo_guard", Name: "Престон Гарви", Intelligence: 6, HackerPerkAvailable: false},
	}, playerConfig.Roster, "bundled player profiles must preserve authored IDs, order, and private attributes")

	locations, err := NewSessionLocations(t.TempDir(), root)
	if err != nil {
		require.NoError(t, err)
	}
	require.Falsef(t, locations.BundledDemo != demoPath,
		"BundledDemo = %q, want manifest path %q", locations.BundledDemo, demoPath)

}

func TestAssetsProductionEmbedsOverseerAndPlayerAsSeparateFilesystems(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		require.NoError(t, err)
	}
	source := string(raw)
	requiredFragments := []string{
		"//go:embed all:frontend/overseer/dist\nvar overseerSource embed.FS",
		"//go:embed all:frontend/client/dist\nvar clientSource embed.FS",
		`fs.Sub(overseerSource, "frontend/overseer/dist")`,
		`fs.Sub(clientSource, "frontend/client/dist")`,
		"windowActivation := &overseerWindowActivation{}",
		"newWailsApplication(overseerAssets, windowActivation.handleSecondInstanceLaunch)",
		"composeApplication(rootContext, host, clientAssets)",
		"windowActivation.bind(newOverseerWindow(host))",
	}
	for _, fragment := range requiredFragments {
		assert.Falsef(t, !strings.Contains(source, fragment),
			"main.go is missing production asset wiring %q", fragment)

	}
	hostConstruction := strings.Index(source,
		"newWailsApplication(overseerAssets, windowActivation.handleSecondInstanceLaunch)")
	applicationComposition := strings.Index(source, "composeApplication(rootContext, host, clientAssets)")
	require.NotEqual(t, -1, hostConstruction)
	require.NotEqual(t, -1, applicationComposition)
	assert.Less(t, hostConstruction, applicationComposition,
		"single-instance ownership must be acquired before application services are composed")
	assert.True(t, regexp.MustCompile(`Assets:\s+clientAssets`).MatchString(source),
		"main.go is missing production player asset wiring")
	assert.False(t, strings.Contains(source, "//go:embed all:frontend/overseer/dist all:frontend/client/dist") ||
		strings.Contains(source, "//go:embed all:frontend/client/dist all:frontend/overseer/dist"),
		"overseer and remote-player assets share one embed directive; their serving boundaries must remain separate")
	for _, forbidden := range []string{"frontend/overseer/src", "frontend/overseer/test-fixtures", "frontend/client/src"} {
		assert.NotContains(t, source, "//go:embed all:"+forbidden,
			"production embed includes authored or test-only frontend sources")
	}

	hostRaw, err := os.ReadFile(filepath.Join(root, "wails_host.go"))
	require.NoError(t, err)
	hostSource := string(hostRaw)
	for _, fragment := range []string{
		"application.New(wailsApplicationOptions(overseerAssets, onSecondInstanceLaunch))",
		"Handler: application.AssetFileServerFS(overseerAssets)",
		"ApplicationShouldTerminateAfterLastWindowClosed: true",
		"host.Window.NewWithOptions(overseerWindowOptions())",
		"host.RegisterService(application.NewService(newWailsLifecycleService(ctx, core, host.Quit)))",
		"host.RegisterService(application.NewService(newDesktopService(core)))",
	} {
		assert.Contains(t, hostSource, fragment)
	}
	assert.NotContains(t, hostSource, "playerAssets")
	assert.NotContains(t, hostSource, "PlayerService")
	assert.Equal(t, 1, strings.Count(hostSource, "host.Window.NewWithOptions("))

	viteConfig, err := os.ReadFile(filepath.Join(root, "frontend", "overseer", "vite.config.ts"))
	if err != nil {
		require.NoError(t, err)
	}
	assert.Contains(t, string(viteConfig), "preserveGoEmbedMarker",
		"Vite build does not define the go:embed marker preservation plugin")
	assert.Contains(t, string(viteConfig), "writeFileSync(resolve(outputDirectory, '.keep'), '')",
		"Vite build does not restore frontend/overseer/dist/.keep after emptyOutDir")

}

func TestAssetsPackagedPlayerVueBuildIsCompleteAndOffline(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	dist := filepath.Join(root, "frontend", "client", "dist")
	entries, err := os.ReadDir(dist)
	require.Falsef(t, err != nil,
		"frontend/client/dist/.keep must preserve the go:embed root on a clean checkout: %v", err)

	if len(entries) == 1 && entries[0].Name() == ".keep" {
		return
	}
	assertNonEmptyFiles(t, dist, []string{"index.html"})
	index, err := os.ReadFile(filepath.Join(dist, "index.html"))
	require.NoError(t, err)
	document := string(index)
	assert.Equal(t, 1, strings.Count(document, `<div id="playerApp"></div>`))
	assert.Contains(t, document, `type="module"`)
	assert.Contains(t, document, `./assets/`)
	for _, forbidden := range []string{"client.js", "sound.js", "presentation-uplink.js", "candidate", "test-fixtures", "main.ts", "overseerApp"} {
		assert.NotContains(t, document, forbidden)
	}

	var bundleFiles []string
	err = filepath.WalkDir(dist, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(dist, path)
		if err != nil {
			return err
		}
		bundleFiles = append(bundleFiles, filepath.ToSlash(relative))
		extension := strings.ToLower(filepath.Ext(path))
		assert.NotContains(t, []string{".ts", ".tsx", ".vue", ".map"}, extension, relative)
		for _, forbidden := range []string{"client.js", "sound.js", "presentation-uplink.js", "candidate", "test-fixtures", "overseer"} {
			assert.NotContains(t, strings.ToLower(relative), strings.ToLower(forbidden), relative)
		}
		if extension == ".html" || extension == ".js" || extension == ".css" {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{
				"https://cdn.", "http://localhost:", "http://127.0.0.1:5173", "@vite/client",
				"client.js", "sound.js", "presentation-uplink.js", "candidate-main", "test-fixtures",
				"overseerApp", "fallout.terminal.private.v1",
			} {
				assert.Falsef(t, strings.Contains(string(raw), forbidden),
					"packaged player asset %s depends on %q", relative, forbidden)

			}
		}
		return nil
	})
	if err != nil {
		require.NoError(t, err)
	}
	for extension, description := range map[string]string{".js": "generated player bundle", ".css": "player stylesheet", ".ttf": "Fixedsys font"} {
		assert.Falsef(t, !containsExtension(bundleFiles, extension),
			"packaged player is missing %s", description)

	}
	sourceFont, err := os.ReadFile(filepath.Join(root, "frontend", "client", "fonts", "Fixedsys.ttf"))
	require.NoError(t, err)
	builtFonts, err := filepath.Glob(filepath.Join(dist, "assets", "*.ttf"))
	require.NoError(t, err)
	require.Len(t, builtFonts, 1)
	builtFont, err := os.ReadFile(builtFonts[0])
	require.NoError(t, err)
	assert.Equal(t, sha256.Sum256(sourceFont), sha256.Sum256(builtFont), "built Player font differs from the production source asset")
	for _, category := range []string{"ambient", "charscroll", "enter", "hack-bad", "hack-good", "menu-focus", "multiple", "single"} {
		entries, err := os.ReadDir(filepath.Join(dist, "sounds", category))
		assert.Falsef(t, err != nil || len(entries) == 0,
			"packaged player sound category %s is missing: %v", category, err)

	}

	mainSource, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		require.NoError(t, err)
	}
	assert.False(t, !strings.Contains(string(mainSource), "//go:embed all:frontend/client/dist") || !strings.Contains(string(mainSource), `fs.Sub(clientSource, "frontend/client/dist")`),
		"production does not embed only the complete built player application")

}

func TestMacOSPackageVerificationCoversResourcesSignatureAndCanonicalIdentity(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	verifyRaw, err := os.ReadFile(filepath.Join(root, "scripts", "verify-macos-app.sh"))
	require.NoError(t, err)
	verify := string(verifyRaw)
	for _, required := range []string{
		"Contents/MacOS/Fallout Terminal",
		"Contents/Info.plist",
		"Contents/Resources",
		"lipo -archs",
		"LSMinimumSystemVersion",
		"LC_BUILD_VERSION",
		"icon.icns",
		"sessions/demo.json",
		"codesign -d --entitlements",
		"codesign --verify --deep --strict",
		"TestPackagePlanCompletesResourcesBeforeFinalSignature",
		"hash-macos-app.sh",
	} {
		assert.Contains(t, verify, required)
	}
	assert.NotRegexp(t, regexp.MustCompile(`(^|[[:space:];|&])rg([[:space:]]|$)`), verify)

	hashRaw, err := os.ReadFile(filepath.Join(root, "scripts", "hash-macos-app.sh"))
	require.NoError(t, err)
	hashSource := string(hashRaw)
	for _, required := range []string{"LC_ALL=C sort -z", "stat -f '%Lp'", "shasum -a 256", "readlink", "bundle inventory changed while hashing", "--self-test"} {
		assert.Contains(t, hashSource, required)
	}
}

func TestActiveWailsV3DocumentsStaySeparateFromHistoricalEvidence(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	readmeRaw, err := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, err)
	readme := string(readmeRaw)
	assert.Contains(t, readme, "docs/wails-v3-migration-rollback.md")

	rollbackRaw, err := os.ReadFile(filepath.Join(root, "docs", "wails-v3-migration-rollback.md"))
	require.NoError(t, err)
	rollback := string(rollbackRaw)
	assert.Contains(t, rollback, "specs/006-wails-v3-migration/")
	assert.Contains(t, rollback, "specs/001-wails-v2-migration/")
	assert.Contains(t, rollback, "docs/wails-migration-rollback.md")

	scannerRaw, err := os.ReadFile(filepath.Join(root, "scripts", "wails-v3-cutover-check.sh"))
	require.NoError(t, err)
	scanner := string(scannerRaw)
	for _, required := range []string{
		"active Go source contains v2 or dual-runtime code",
		"application module still resolves Wails v2",
		"frontend source/generated/bundle contains a v2 global or dual-runtime fallback",
		"active command/documentation bypasses Task or uses v2, global, or floating Wails resolution",
		"historical Wails v2 spec is missing",
		"historical Electron-to-Wails rollback record is missing",
		"git -C \"${repository_root}\" diff --exit-code -- specs/001-wails-v2-migration docs/wails-migration-rollback.md",
		"go -C \"${repository_root}\" list -m all",
	} {
		assert.Contains(t, scanner, required)
	}
	assert.NotRegexp(t, regexp.MustCompile(`(^|[[:space:];|&])rg([[:space:]]|$)`), scanner)
}

func TestRetainedLegacyPlayerDocumentsAreExplicitlyHistoricalAndLinkCurrentContract(t *testing.T) {
	t.Parallel()
	root := assetRepositoryRoot(t)
	directories := []string{
		"specs/001-wails-v2-migration",
		"specs/003-hacking-game-evolution",
		"specs/004-player-sessions-control",
	}
	legacyDocuments := 0
	for _, relative := range directories {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(relative)), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			lower := strings.ToLower(string(raw))
			if !strings.Contains(lower, "websocket") && !strings.Contains(lower, "ws:") && !strings.Contains(lower, "wss:") && !strings.Contains(lower, "handwritten json player") {
				return nil
			}
			legacyDocuments++
			require.Contains(t, string(raw), "SUPERSEDED LEGACY PLAYER TRANSPORT — HISTORICAL, NON-AUTHORITATIVE", path)
			require.Contains(t, string(raw), "005-connectrpc-protobuf-migration/contracts/public-player.md", path)
			return nil
		})
		require.NoError(t, err)
	}
	require.Greater(t, legacyDocuments, 1, "the scan must cover retained documents beyond feature 001")
}

func assertNonEmptyFiles(t *testing.T, root string, paths []string) {
	t.Helper()
	for _, path := range paths {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			assert.Failf(t, "assertion failed", "required asset %q: %v", path, err)
			continue
		}
		assert.Falsef(t, !info.Mode().IsRegular() || info.Size() == 0,
			"required asset %q is not a non-empty regular file", path)

	}
}

func containsExtension(paths []string, extension string) bool {
	for _, path := range paths {
		if strings.EqualFold(filepath.Ext(path), extension) {
			return true
		}
	}
	return false
}

func assetRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.False(t, !ok,
		"cannot resolve asset-manifest test location")

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
