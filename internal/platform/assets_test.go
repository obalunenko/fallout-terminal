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
		"frontend/client/client.js",
		"frontend/overseer/package.json",
		"frontend/overseer/src/index.html",
		"frontend/overseer/src/overseer.js",
		"frontend/overseer/src/overseer.css",
	})
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

func TestGeneratedProtobufIdentityChangesOnlyGoPackageForV2(t *testing.T) {
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
		"fallout/terminal/player/v1/hacking.proto":            contract("player", "playerv1", "hacking_pb.js"),
		"fallout/terminal/player/v1/navigation.proto":         contract("player", "playerv1", "navigation_pb.js"),
		"fallout/terminal/player/v1/player.proto":             contract("player", "playerv1", "player_pb.js"),
		"fallout/terminal/player/v1/sound.proto":              contract("player", "playerv1", "sound_pb.js"),
		"fallout/terminal/player/v1/terminal.proto":           contract("player", "playerv1", "terminal_pb.js"),
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
	gotBrowserPaths, err := filepath.Glob(filepath.Join(root, "frontend", "client", "gen", "fallout", "terminal", "player", "v1", "*_pb.js"))
	require.NoError(t, err)
	gotBrowserFiles := make([]string, 0, len(gotBrowserPaths))
	for _, path := range gotBrowserPaths {
		gotBrowserFiles = append(gotBrowserFiles, filepath.Base(path))
	}
	require.Equal(t, wantBrowserFiles, gotBrowserFiles, "browser descriptor inventory changed")
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

func TestOverseerPublicAccessControlsAreAccessibleAndNeverExposeStoredSecrets(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(relative string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		require.NoError(t, err)
		return string(raw)
	}
	html := read("frontend/overseer/src/index.html")
	css := read("frontend/overseer/src/overseer.css")
	javascript := read("frontend/overseer/src/overseer.js")

	for _, fragment := range []string{
		`id="publicAccessSection"`, `class="public-access-compact"`, `ПУБЛИЧНЫЙ АДРЕС`,
		`id="btnStartPublicAccess" type="button">ВКЛЮЧИТЬ ДОСТУП</button>`,
		`id="btnStopPublicAccess" type="button" hidden>ОСТАНОВИТЬ ДОСТУП</button>`,
		`id="btnOpenPublicAccessSettings" type="button">НАСТРОЙКИ…</button>`,
		`id="publicAccessSettingsDialog" aria-modal="true"`,
		`id="btnClosePublicAccessSettings" type="button">ЗАКРЫТЬ</button>`,
		`id="publicAccessSetupRequired" role="status" aria-live="polite" hidden`,
		`id="publicAccessConnectionGroup"`, `ПОДКЛЮЧЕНИЕ NGROK`,
		`id="publicAccessProviderConfigured" hidden`,
		`id="btnOpenPublicAccessProviderToken" type="button">ИЗМЕНИТЬ ТОКЕН</button>`,
		`id="publicAccessPlayerLoginGroup"`, `ВХОД ДЛЯ ИГРОКОВ`,
		`for="publicAccessDomain"`, `for="publicAccessUsername"`,
		`for="publicAccessProviderToken"`, `for="publicAccessPlayerPassword"`,
		`id="publicAccessProviderToken" type="password"`, `id="publicAccessPlayerPassword" type="password"`,
		`id="btnGeneratePlayerPassword" type="button">СГЕНЕРИРОВАТЬ</button>`,
		`id="btnCancelPublicAccessSettings" type="button">ОТМЕНА</button>`,
		`id="publicAccessProviderTokenDialog" aria-modal="true"`,
		`id="publicAccessReplacementProviderToken" type="password"`,
		`Сохранённый токен нельзя посмотреть`, `СОХРАНИТЬ ТОКЕН`, `УДАЛИТЬ СОХРАНЁННЫЙ ТОКЕН`,
		`id="publicAccessGuide"`, `КАК НАСТРОИТЬ ЧЕРЕЗ NGROK`,
		`СОХРАНИТЬ НАСТРОЙКИ`, `Basic Auth`,
		`id="publicAccessStatus" role="status" aria-live="polite"`,
		`id="publicAccessError" role="alert" aria-live="assertive"`,
		`id="generatedPasswordDialog"`, `aria-modal="true"`,
	} {
		assert.Contains(t, html, fragment)
	}
	sectionStart := strings.Index(html, `id="publicAccessSection"`)
	dialogStart := strings.Index(html, `id="publicAccessSettingsDialog"`)
	require.NotEqual(t, -1, sectionStart)
	require.NotEqual(t, -1, dialogStart)
	require.Less(t, sectionStart, dialogStart)
	assert.NotContains(t, html[sectionStart:dialogStart], `id="publicAccessForm"`,
		"public-access configuration must not remain inline in the compact section")
	connectionStart := strings.Index(html, `id="publicAccessConnectionGroup"`)
	playerLoginStart := strings.Index(html, `id="publicAccessPlayerLoginGroup"`)
	actionsStart := strings.Index(html, `class="public-access-actions"`)
	require.NotEqual(t, -1, connectionStart)
	require.NotEqual(t, -1, playerLoginStart)
	require.NotEqual(t, -1, actionsStart)
	assert.Less(t, connectionStart, playerLoginStart)
	assert.Less(t, playerLoginStart, actionsStart)
	for _, removed := range []string{
		`id="publicAccessBehaviorGroup"`,
		`id="publicAccessEnabledPreference"`,
		`Включать публичный доступ при запуске приложения`,
	} {
		assert.NotContains(t, html+javascript, removed)
	}
	for _, forbidden := range []string{"RevealSecret", "GetSecret", ">REVEAL<", ">ПОКАЗАТЬ ПАРОЛЬ<"} {
		assert.NotContains(t, html+javascript, forbidden)
	}
	assert.Contains(t, html, `id="publicAccessUsername"`)
	assert.Contains(t, html, `value="players"`)
	assert.Contains(t, css, ".public-access")
	for _, fragment := range []string{
		"generatePlayerPassword",
		"public-access-status",
		"publicAccessDisplayURL",
		"showPublicAccessSettings({ setupRequired: true })",
		"publicAccessSnapshot?.providerTokenPresence === 'present'",
		"publicAccessSnapshot?.playerPasswordPresence === 'present'",
		"publicAccessProviderSetup.hidden = providerTokenConfigured",
		"showPublicAccessProviderTokenDialog",
		"runPublicAccessProviderTokenMutation",
		"deleteProviderToken: true",
		"btnStartPublicAccess.hidden = stopping",
		"btnStopPublicAccess.hidden = !stopping",
		"hidePublicAccessSettings()",
	} {
		assert.Contains(t, javascript, fragment)
	}
	assert.Contains(t, read("frontend/overseer/src/desktop-api.js"), "Clipboard.SetText")
}

func TestApplicationUpdateAssetsAreAccessibleAndKeepProviderDetailsBackendOnly(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(relative string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		require.NoError(t, err)
		return string(raw)
	}

	html := read("frontend/overseer/src/index.html")
	for _, fragment := range []string{
		`id="applicationUpdateStatusPanel" aria-label="Обновление приложения"`,
		`id="applicationUpdateStatus" role="status" aria-live="polite" aria-atomic="true"`,
		`id="applicationUpdateError" role="alert" aria-live="assertive" aria-atomic="true"`,
		`id="applicationUpdateProgress" aria-label="Подготовка обновления приложения"`,
		`id="applicationUpdateDialog" aria-modal="true" aria-labelledby="applicationUpdateDialogTitle" aria-describedby="applicationUpdateDialogDescription applicationUpdateReleaseNotes"`,
		`id="applicationUpdateRestartDialog" aria-modal="true" aria-labelledby="applicationUpdateRestartDialogTitle" aria-describedby="applicationUpdateRestartDialogDescription"`,
		`id="btnAcceptApplicationUpdate" type="button"`,
		`id="btnDeferApplicationUpdate" type="button"`,
		`id="btnRestartApplicationUpdate" type="button"`,
		`id="btnPostponeApplicationUpdate" type="button"`,
	} {
		assert.Contains(t, html, fragment,
			"Overseer update markup is missing accessible contract %q", fragment)
	}
	assert.Equal(t, 1, strings.Count(html, `id="applicationUpdateDialog"`),
		"the update offer must have one dialog")
	assert.Equal(t, 1, strings.Count(html, `id="applicationUpdateRestartDialog"`),
		"restart consent must use one separate dialog")

	overseer := read("frontend/overseer/src/overseer.js")
	for _, fragment := range []string{
		"applicationUpdateDialog.showModal()",
		"applicationUpdateRestartDialog.showModal()",
		"btnDeferApplicationUpdate.focus()",
		"btnPostponeApplicationUpdate.focus()",
		"void resolveApplicationUpdateOffer('defer')",
		"void resolveApplicationUpdateRestart('postpone')",
		"void resolveApplicationUpdateRestart('restart')",
	} {
		assert.Contains(t, overseer, fragment,
			"Overseer update flow is missing keyboard-safe consent behavior %q", fragment)
	}

	facade := read("frontend/overseer/src/desktop-api.js")
	for _, fragment := range []string{
		"getApplicationUpdateStatus: desktopService.GetApplicationUpdateStatus",
		"resolveApplicationUpdateOffer: desktopService.ResolveApplicationUpdateOffer",
		"resolveApplicationUpdateRestart: desktopService.ResolveApplicationUpdateRestart",
		"Events.On('application-update-status'",
	} {
		assert.Contains(t, facade, fragment,
			"Overseer update facade is missing the private desktop boundary %q", fragment)
	}

	backend := read("wails_updater.go") + "\n" + read("internal/update/model.go") + "\n" + read("internal/update/helper.go")
	for _, fragment := range []string{
		"applicationGitHubAPIBaseURL",
		"applicationGitHubProvider",
		"DownloadURL",
		"PreparedApplicationUnit",
		"InstalledUnit",
		"StagedUnit",
		"LaunchRelativePath",
	} {
		assert.Contains(t, backend, fragment,
			"application update backend is missing privileged implementation detail %q", fragment)
	}

	frontend := html + "\n" + read("frontend/overseer/src/overseer.css") + "\n" + overseer + "\n" + facade
	for _, forbidden := range []string{
		"https://api.github.com",
		"api.github.com/repos/",
		"browser_download_url",
		"github.asset.",
		"applicationGitHubProvider",
		"DownloadURL",
		"PreparedApplicationUnit",
		"InstalledUnit",
		"StagedUnit",
		"LaunchRelativePath",
		"application-update-recovery.json",
		"Authorization: Bearer",
		"fetch(",
		"XMLHttpRequest",
	} {
		assert.NotContains(t, frontend, forbidden,
			"frontend update assets expose backend-only provider/helper capability %q", forbidden)
	}

	player := read("frontend/client/index.html") + "\n" + read("frontend/client/client.js")
	for _, forbidden := range []string{
		"application-update-status",
		"GetApplicationUpdateStatus",
		"ResolveApplicationUpdateOffer",
		"ResolveApplicationUpdateRestart",
		"applicationUpdateDialog",
		"applicationUpdateRestartDialog",
	} {
		assert.NotContains(t, player, forbidden,
			"player frontend exposes private application-update capability %q", forbidden)
	}
}

func TestOverseerAssetManifestSupportsCleanCheckoutAndBuiltOutput(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	assertNonEmptyFiles(t, root, []string{
		"frontend/overseer/src/index.html",
		"frontend/overseer/src/overseer.css",
		"frontend/overseer/src/overseer.js",
		"frontend/overseer/src/desktop-api.js",
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
}

func TestRetainedPlayerAssetAndSoundManifest(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	assertNonEmptyFiles(t, root, []string{
		"frontend/client/index.html",
		"frontend/client/client.css",
		"frontend/client/client.js",
		"frontend/client/sound.js",
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

func TestPlayerHackingOutcomeAudioUsesEligibleAuthoritativeTransitions(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	soundScript := read("frontend/client/sound.js")
	for _, required := range []string{
		"let webAudioEligible = false;",
		"const folderLoads = new Map();",
		"const rawLoads    = new Map();",
		"const oneShotFolders = ['single', 'multiple', 'enter', 'hack-good', 'hack-bad', 'menu-focus', 'charscroll'];",
		"function enableWebAudio()",
		"function reportPlayback(url)",
		"if (!webAudioReady || !await webAudioReady || !webAudioEligible) {",
		"await Promise.all(oneShotFolders.map(loadFolder));",
		"const buffer = await context.decodeAudioData(raw.slice(0));",
		"if (!rawBufs.has(url)) await prefetch(url);",
		"await Promise.all(supported.map(prefetch));",
		"enableWebAudio();",
		"reportPlayback(url);",
		"playFromFolder('single', 0.55)",
		"playFromFolder('multiple', 0.55)",
		"playFromFolder('enter', 0.65)",
		"playFirst('menu-focus', 0.5)",
		"playFirst('hack-good', 0.8)",
		"playFirst('hack-bad', 0.7)",
		"playFromFolder('charscroll', 0.4)",
	} {
		assert.Falsef(t, !strings.Contains(soundScript, required),
			"player sound adapter is missing outcome-audio contract %q", required)

	}

	playerScript := read("frontend/client/client.js")
	outcomeStart := strings.Index(playerScript, "function playHackOutcomeTransition(previousHack, nextHack, revision = appliedSharedRevision) {")
	require.False(t, outcomeStart < 0,
		"player script is missing the common authoritative hacking-outcome boundary")

	outcomeEnd := strings.Index(playerScript[outcomeStart:], "function scheduleHackSolvedNavigation() {")
	require.False(t, outcomeEnd < 0,
		"player script is missing the common authoritative hacking-outcome boundary")

	outcomeBoundary := playerScript[outcomeStart : outcomeStart+outcomeEnd]
	for _, required := range []string{
		"nextHack.attemptsLeft < previousHack.attemptsLeft",
		"playHackBad();",
		"nextHack.solved && !previousHack.solved",
		"playHackGood();",
	} {
		assert.Falsef(t, !strings.Contains(outcomeBoundary, required),
			"common authoritative outcome handler is missing transition guard %q", required)

	}
	for _, required := range []string{
		"let terminalLiveBaselinePending = true;",
		"const isContinuousTerminalUpdate = !terminalLiveBaselinePending && hasLive",
		"if (isContinuousTerminalUpdate) playHackOutcomeTransition(previousHack, hack);",
		"terminalLiveBaselinePending = false;",
		"playHackOutcomeTransition(previousHack, hack);",
	} {
		assert.Falsef(t, !strings.Contains(playerScript, required),
			"player script is missing revisioned live/hack outcome guard %q", required)

	}

	actionResultStart := strings.Index(playerScript, "async function applyMutationResult(operation) {")
	require.False(t, actionResultStart < 0,
		"player script is missing typed mutation-result boundary")

	actionResultEnd := strings.Index(playerScript[actionResultStart:], "function actionReasonName(reason) {")
	require.False(t, actionResultEnd < 0,
		"player script is missing typed mutation-result boundary")
	assert.False(t, strings.Contains(playerScript[actionResultStart:actionResultStart+actionResultEnd], "playHack"),
		"ACTION_RESULT must not optimistically play hacking outcome audio")

	beginActionStart := strings.Index(playerScript, "function beginSharedMutation(procedure, invoke) {")
	require.False(t, beginActionStart < 0,
		"player script is missing the shared-action presentation boundary")

	beginActionEnd := strings.Index(playerScript[beginActionStart:], "function beginSharedMutationForAction(")
	require.False(t, beginActionEnd < 0,
		"player script is missing the shared-action presentation boundary")

	beginAction := playerScript[beginActionStart : beginActionStart+beginActionEnd]
	assert.False(t, !strings.Contains(beginAction, "renderPlayerContext();") || strings.Contains(beginAction, "render();"),
		"beginSharedAction must update pending presentation without rebuilding the hacking board")

	for _, required := range []string{
		"playControllerPresentationCue(previousPresentation, controllerPresentation, terminalLiveBaselinePending);",
		"if (baseline || !next || JSON.stringify(previous) === JSON.stringify(next)) return;",
		"setHackHover(hoveredCells.length ? target : null, true);",
	} {
		assert.Falsef(t, !strings.Contains(playerScript, required),
			"player script is missing preview-audio replay guard %q", required)

	}
}

func TestPlayerAmbientAudioUsesExplicitRetryableLifecycleState(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}
	functionBoundary := func(script, start, end string) string {
		t.Helper()
		startIndex := strings.Index(script, start)
		require.Falsef(t, startIndex < 0,
			"missing JavaScript boundary %q", start)

		endIndex := strings.Index(script[startIndex+len(start):], end)
		require.Falsef(t, endIndex < 0,
			"missing JavaScript boundary %q after %q", end, start)

		return script[startIndex : startIndex+len(start)+endIndex]
	}

	soundScript := read("frontend/client/sound.js")
	for _, state := range []string{"ambientRequested", "ambientPlayAttempt", "webAudioEligible", "webAudioReady"} {
		assert.Falsef(t, !strings.Contains(soundScript, state),
			"player sound adapter is missing explicit audio state %q", state)

	}
	activation := functionBoundary(soundScript, "function enableWebAudio()", "async function prefetch")
	for _, boundary := range []string{
		"if (webAudioEligible)",
		"if (webAudioReady)",
		"webAudioReady = attempt",
		"webAudioReady = null",
	} {
		assert.Falsef(t, !strings.Contains(activation, boundary),
			"Web Audio activation is missing retry boundary %q", boundary)

	}
	for _, event := range []string{"'pointerdown'", "'keydown'"} {
		assert.Falsef(t, strings.Count(soundScript, "document.addEventListener("+event+", handleAudioGesture)") != 1,
			"player sound adapter must observe qualifying gesture %s exactly once", event)

	}

	reconcile := functionBoundary(soundScript, "function reconcileAmbient()", "function setAmbientActive(active)")
	for _, boundary := range []string{
		"ambientRequested",
		"ambientReady",
		"ambientAudio.paused",
		"ambientPlayAttempt",
		"ambientAudio.play()",
		".catch(",
		".finally(",
	} {
		assert.Falsef(t, !strings.Contains(reconcile, boundary),
			"ambient reconciliation is missing state boundary %q", boundary)

	}
	ambientState := functionBoundary(soundScript, "function setAmbientActive(active)", "function stopAmbient()")
	for _, boundary := range []string{"Boolean(active)", "ambientRequested = requested", "reconcileAmbient()", "stopAmbient()"} {
		assert.Falsef(t, !strings.Contains(ambientState, boundary),
			"ambient state API is missing transition boundary %q", boundary)

	}

	playerScript := read("frontend/client/client.js")
	assert.False(t, strings.Count(playerScript, "setAmbientActive(true);") != 1,
		"accepted terminal-live dispatch must be the sole ambient activation boundary")
	assert.False(t, strings.Count(playerScript, "setAmbientActive(false);") < 3,
		"player lifecycle must revoke ambient state across clear, reset, and reconnect boundaries")

	for _, obsolete := range []string{"tryStartAmbient", "stopAmbient"} {
		assert.Falsef(t, strings.Contains(playerScript, obsolete),
			"player lifecycle must use the explicit ambient state API, found %q", obsolete)

	}
}

func TestBrowserJavaScriptUsesSpacesInsteadOfTabs(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	paths := []string{
		"frontend/client/client.js",
		"frontend/client/sound.js",
		"frontend/overseer/src/desktop-api.js",
		"frontend/overseer/src/overseer.js",
	}
	for _, relative := range paths {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			require.NoError(t, err)
		}
		assert.Falsef(t, strings.ContainsRune(string(raw), '\t'),
			"%s contains a tab; browser JavaScript uses two-space indentation", relative)

	}
}

func TestOverseerTerminalCommandsCannotBypassCoordinator(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	facade := read("frontend/overseer/src/desktop-api.js")
	app := read("app.go")
	for _, required := range []string{
		"requestTerminalActivation: desktopService.RequestTerminalActivation",
		"requestTerminalClear: desktopService.RequestTerminalClear",
		"updateLiveTerminal: desktopService.UpdateLiveTerminal",
		"forceHackSuccess: desktopService.ForceHackSuccess",
	} {
		assert.Falsef(t, !strings.Contains(facade, required),
			"overseer desktop facade is missing coordinator-owned command %q", required)

	}
	for _, forbidden := range []string{"SetLiveTerminal", "ClearLiveTerminal", "setLiveTerminal", "clearLiveTerminal"} {
		assert.Falsef(t, strings.Contains(facade, forbidden),
			"overseer desktop facade exposes legacy terminal command %q", forbidden)

	}
	for _, required := range []string{
		"func (app *App) RequestTerminalActivation(",
		"func (app *App) RequestTerminalClear(",
		"func (app *App) UpdateLiveTerminal(",
		"func (app *App) ForceHackSuccess(",
	} {
		assert.Falsef(t, !strings.Contains(app, required),
			"Wails App is missing required terminal command %q", required)

	}
	for _, forbidden := range []string{
		"func (app *App) SetLiveTerminal(",
		"func (app *App) ClearLiveTerminal(",
	} {
		assert.Falsef(t, strings.Contains(app, forbidden),
			"Wails App still binds legacy terminal command %q", forbidden)

	}
}

func TestPlayerHiddenStatesStayOutOfInactiveLayout(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	html := read("frontend/client/index.html")
	for _, fragment := range []string{
		`class="term-body" id="termBody"`,
		`class="term-idle" id="termIdle"`,
		`class="hack-board" id="hackBoard" hidden`,
		`class="hack-blocked" id="hackBlocked" hidden`,
	} {
		assert.Falsef(t, !strings.Contains(html, fragment),
			"player markup is missing visibility fixture %q", fragment)

	}

	css := read("frontend/client/client.css")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(css)
	for _, fragment := range []string{
		".term-body{flex:11auto;display:flex;flex-direction:column;min-height:0;overflow:hidden;}",
		".term-idle{height:100%;display:flex;",
		".term-entry{height:100%;min-height:0;display:flex;flex-direction:column;overflow:hidden;",
		".hack-board{height:100%;display:flex;",
		".hack-blocked{height:100%;display:flex;",
	} {
		assert.Falsef(t, !strings.Contains(compactCSS, fragment),
			"player stylesheet no longer exercises the hidden-layout regression fixture %q", fragment)

	}
	assert.False(t, !strings.Contains(compactCSS, "[hidden]{display:none!important;}"),
		"player stylesheet must make the hidden attribute authoritative so inactive state containers occupy no layout space")

}

func TestPlayerSessionSelectionAssetContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	html := read("frontend/client/index.html")
	for _, fragment := range []string{
		`class="player-status-line" id="playerIdentity" hidden`,
		`class="player-status-role" id="roleBadge"`,
		`class="character-select" id="characterSelect" hidden`,
		`class="assigned-waiting" id="assignedWaiting" hidden`,
		`class="player-notice" id="playerNotice"`,
	} {
		assert.Falsef(t, !strings.Contains(html, fragment),
			"player markup is missing session-selection region %q", fragment)

	}
	for _, forbidden := range []string{"browserToken", "ForceHackSuccess", "forceHackSuccess", "HACK_ADMIN"} {
		assert.Falsef(t, strings.Contains(html, forbidden),
			"player markup exposes private/session capability %q", forbidden)

	}

	css := read("frontend/client/client.css")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(css)
	for _, fragment := range []string{
		".character-select{height:100%;min-height:0;display:flex;",
		`.character-option[data-status="claimed"]{`,
		".character-select.pending{",
		".assigned-waiting{height:100%;display:flex;",
		"[hidden]{display:none!important;}",
	} {
		assert.Falsef(t, !strings.Contains(compactCSS, fragment),
			"player stylesheet is missing bounded selection contract %q", fragment)

	}
	for _, forbidden := range []string{"overflow:auto", "overflow-y:auto", "overflow:scroll", "overflow-y:scroll", "scrollbar"} {
		assert.Falsef(t, strings.Contains(compactCSS, forbidden),
			"session selection must remain within the no-scroll player layout; found %q", forbidden)

	}

	js := read("frontend/client/client.js")
	for _, fragment := range []string{
		"const PLAYER_TOKEN_KEY = 'fallout-terminal.player-token'",
		"localStorage.getItem(PLAYER_TOKEN_KEY)",
		"localStorage.setItem(PLAYER_TOKEN_KEY",
		"option.textContent = entry.name",
		"playerRPC.subscribe(request",
		"playerRPC.selectCharacter({",
	} {
		assert.Falsef(t, !strings.Contains(js, fragment),
			"player script is missing safe selection/identity contract %q", fragment)

	}
	for _, forbidden := range []string{
		"searchParams.set('browserToken'",
		`searchParams.set("browserToken"`,
		"innerHTML = entry.name",
		"ForceHackSuccess",
		"forceHackSuccess",
		"HACK_ADMIN",
	} {
		assert.Falsef(t, strings.Contains(js, forbidden),
			"player script exposes unsafe session/privileged path %q", forbidden)

	}
}

func TestPlayerDesktopResponsiveLayoutContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	html := read("frontend/client/index.html")
	assert.False(t, !strings.Contains(html, `content="width=device-width, initial-scale=1.0"`),
		"player viewport must follow the browser width at the default zoom")
	assert.False(t, strings.Contains(html, "maximum-scale"),
		"player viewport must not prevent accessibility zoom")

	for _, fragment := range []string{
		`class="term-header" id="normalHeader"`,
		`class="term-body" id="termBody"`,
		`class="term-footer"`,
		`class="page-nav" id="pageNav"`,
		`class="page-btn" id="pagePrev"`,
		`class="page-indicator" id="pageIndicator"`,
		`class="page-btn" id="pageNext"`,
		`class="term-prompt" id="termPrompt"`,
	} {
		assert.Falsef(t, !strings.Contains(html, fragment),
			"player markup is missing persistent responsive region %q", fragment)

	}

	css := read("frontend/client/client.css")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(css)
	for _, fragment := range []string{
		"--terminal-scale:clamp(8px,min(1.5625vw,2.6667vh),24px);",
		"--font-chrome:calc(var(--terminal-scale)*.875);",
		"--font-body:var(--terminal-scale);",
		"--font-menu:calc(var(--terminal-scale)*1.0625);",
		"--font-title:calc(var(--terminal-scale)*1.1875);",
		"--font-hack:var(--terminal-scale);",
		".screen{position:relative;width:100%;max-width:1500px;height:100%;max-height:920px;min-width:0;min-height:0;overflow:hidden;",
		".hdr-intro{max-height:min(18vh,8rem);overflow:hidden;overflow-wrap:anywhere;}",
		".term-output{flex:0030%;min-height:calc(var(--terminal-scale)*2.8);",
		".page-nav{min-width:0;display:flex;align-items:center;justify-content:flex-end;",
		".term-footer:has(.back-btn[hidden]):has(.page-nav[hidden]){min-height:0;margin-top:0;}",
		"@media(max-height:720px){:root{--screen-pad-y:max(4px,calc(var(--terminal-scale)*.5));",
		".hack-board.hack-compact.hack-stacked{",
		".hack-board.hack-stacked{flex-direction:column;",
	} {
		assert.Falsef(t, !strings.Contains(compactCSS, fragment),
			"player stylesheet is missing responsive layout contract %q", fragment)

	}

	for _, forbidden := range []string{"overflow:auto", "overflow-y:auto", "overflow:scroll", "overflow-y:scroll", "scrollbar"} {
		assert.Falsef(t, strings.Contains(compactCSS, forbidden),
			"player stylesheet must not expose browser or localized scrolling; found %q", forbidden)

	}

	js := read("frontend/client/client.js")
	for _, fragment := range []string{
		"function paginateText(container, text)",
		"function naturalPageBreak(text, start, fittedEnd)",
		"pagedView.index = Math.min(authoritativeIndex, pagedView.pages.length - 1)",
		"pagePrev.hidden = pagedView.index === 0",
		"pageNext.hidden = pagedView.index >= pagedView.pages.length - 1",
		"pageIndicator.value = `${pagedView.index + 1} / ${pagedView.pages.length}`",
		"activatePagination('entry', viewEntryId",
		"function renderCommandRecordSurface({ kind, key, title, text, showBack })",
		"activatePagination(kind, key, text, entryBody, isNewCommand)",
		"e.key === 'ArrowLeft' || e.key === 'PageUp'",
		"e.key === 'ArrowRight' || e.key === 'PageDown'",
		"window.addEventListener('resize', scheduleRepagination)",
		"new ResizeObserver(scheduleRepagination)",
		"document.fonts.ready.then(scheduleRepagination)",
		"function scheduleHackFit()",
		"hackBoard.classList.toggle('hack-stacked'",
		"hackBoard.classList.toggle('hack-compact'",
		"function renderHackScreen() {\n  deactivatePagination();",
		"renderHackInputPreview();\n  renderHackColumns(isNewHack, hackKey);",
	} {
		assert.Falsef(t, !strings.Contains(js, fragment),
			"player script is missing pagination contract %q", fragment)

	}
	assert.False(t, strings.Contains(js, ".scrollTop") || strings.Contains(js, ".scrollTo("),
		"player script must not navigate terminal content through scrolling")

}

func TestPlayerHackingSingleScreenContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	html := read("frontend/client/index.html")
	for _, fragment := range []string{
		`class="term-header" id="hackHeader" hidden`,
		`class="hack-board" id="hackBoard" hidden`,
		`class="hack-columns" id="hackColumns"`,
		`class="hack-log" id="hackLog"`,
		`class="hack-input-line"`,
		`class="page-nav" id="pageNav" aria-label="Навигация по страницам" hidden`,
	} {
		assert.Falsef(t, !strings.Contains(html, fragment),
			"player markup is missing single-screen hacking region %q", fragment)

	}

	css := read("frontend/client/client.css")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(css)
	for _, fragment := range []string{
		"--font-body:var(--terminal-scale);",
		"--font-hack:var(--terminal-scale);",
		".hack-board{height:100%;display:flex;align-items:stretch;",
		".hack-columns{flex:11auto;display:flex;",
		".hack-log-panel{flex:0132%;display:grid;grid-template-rows:1frauto;",
		".hack-board.hack-tight.hack-stacked{gap:0;}",
		".hack-board.hack-tight.hack-stacked.hack-columns{flex-shrink:0;}",
		".hack-board.hack-tight.hack-stacked.hack-log-panel{min-height:0;row-gap:0;padding-top:0;}",
	} {
		assert.Falsef(t, !strings.Contains(compactCSS, fragment),
			"player stylesheet is missing single-screen hacking contract %q", fragment)

	}
	for _, forbidden := range []string{"overflow:auto", "overflow-y:auto", "overflow:scroll", "overflow-y:scroll", "scrollbar"} {
		assert.Falsef(t, strings.Contains(compactCSS, forbidden),
			"hacking layout must not rely on scrolling; found %q", forbidden)

	}

	js := read("frontend/client/client.js")
	start := strings.Index(js, "function renderHackScreen()")
	end := strings.Index(js, "function hackRevealIdentity")
	require.False(t, start < 0 || end <= start,
		"player script is missing the hacking render boundary")

	hackingRender := js[start:end]
	assert.False(t, !strings.Contains(hackingRender, "deactivatePagination();"),
		"hacking render must disable information-view pagination")
	assert.False(t, strings.Contains(hackingRender, "\n  activatePagination(") || strings.Contains(hackingRender, "\n  paginateText("),
		"hacking render must not paginate the board or activity log")

	for _, fragment := range []string{
		"function regionContains(parent, child)",
		"columnsContainer.querySelectorAll('.hack-row')",
		"Array.from(log.children)",
		"regions.some(regionOverflows)",
		"containedRegions.some(([parent, child]) => !regionContains(parent, child))",
		"board.classList.add('hack-tight')",
	} {
		assert.Falsef(t, !strings.Contains(js, fragment),
			"player script is missing rendered hacking geometry contract %q", fragment)

	}
}

func TestPlayerHackingCheatPathsAreRemoved(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	playerScript, err := os.ReadFile(filepath.Join(root, "frontend", "client", "client.js"))
	if err != nil {
		require.NoError(t, err)
	}
	for _, forbidden := range []string{
		"HACK_ADMIN",
		"val === '1'",
		"SUCCESS",
		"URLSearchParams",
		"location.search",
		"forceHackSuccess",
		"ForceHackSuccess",
	} {
		assert.Falsef(t, strings.Contains(string(playerScript), forbidden),
			"bundled player still exposes removed hacking shortcut %q", forbidden)

	}
	assert.False(t, !strings.Contains(string(playerScript), "beginGuess(cell.dataset.target)"),
		"ordinary candidate and filler cells must continue through the typed Guess procedure")

}

func TestPlayerSharedActionPathsAreRoleAndPendingGated(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	playerScript, err := os.ReadFile(filepath.Join(root, "frontend", "client", "client.js"))
	if err != nil {
		require.NoError(t, err)
	}
	js := string(playerScript)

	for _, required := range []string{
		"let pendingSharedAction = null",
		"function canControlSharedTerminal()",
		"playerState.role === 'active'",
		"playerState.phase === 'controlling'",
		"pendingSharedAction === null",
		"function beginSharedMutation(procedure, invoke)",
		"beginNavigation('enter', node.id)",
		"beginNavigation('command', node.id)",
		"beginNavigation('entry', node.id)",
		"beginNavigation('back')",
		"beginPattern(pattern.id)",
		"beginGuess(cell.dataset.target)",
		"activateRow(kids[selIndex])",
		"goBack()",
	} {
		assert.Falsef(t, !strings.Contains(js, required),
			"player script is missing shared-action gate %q", required)

	}
	for _, forbidden := range []string{
		"send({ type: 'NAV_ACTION'",
		"send({ type: 'HACK_GUESS'",
		"send({ type: 'HACK_PATTERN'",
		"ForceHackSuccess",
		"forceHackSuccess",
		"HACK_ADMIN",
	} {
		assert.Falsef(t, strings.Contains(js, forbidden),
			"player script bypasses authority or exposes a privileged path %q", forbidden)

	}

	stylesheet, err := os.ReadFile(filepath.Join(root, "frontend", "client", "client.css"))
	if err != nil {
		require.NoError(t, err)
	}
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(string(stylesheet))
	for _, forbidden := range []string{
		"#screen.observer-read-only:is(.term-row,.back-btn,.hcell){pointer-events:none",
		"#screen.shared-input-pending:is(.term-row,.back-btn,.hcell){pointer-events:none",
		"#screen.observer-read-only.hcell{pointer-events:none",
		"#screen.shared-input-pending.hcell{pointer-events:none",
	} {
		assert.Falsef(t, strings.Contains(compactCSS, forbidden),
			"read-only/pending presentation disables target hit-testing through %q", forbidden)

	}
	for _, required := range []string{
		"cell.className = `hcell ${className}`",
		"cell.dataset.target = String(target)",
		"cell.tabIndex = 0",
		"createHackCell('word', wid, col.text.slice(i, j))",
		"createHackCell('filler', `${colIndex}:${i}`, col.text[i])",
		"cell.dataset.row = String(rowBase + rowIndex)",
		"cell.dataset.offset = String(i - rowStart)",
		"const lines = Array.isArray(hack.log) ? hack.log : []",
	} {
		assert.Falsef(t, !strings.Contains(js, required),
			"active hacking target is not rendered as a focusable hit target %q", required)

	}

	protocol, err := os.ReadFile(filepath.Join(root, "internal", "player", "handler.go"))
	if err != nil {
		require.NoError(t, err)
	}
	for _, forbidden := range []string{"ForceHackSuccess", "forceHackSuccess", "HACK_ADMIN"} {
		assert.Falsef(t, strings.Contains(string(protocol), forbidden),
			"player protocol exposes trusted hacking capability %q", forbidden)

	}
}

func TestPlayerHackingPatternInteractionContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "frontend", "client", "client.js"))
	if err != nil {
		require.NoError(t, err)
	}
	playerScript := string(raw)
	for _, required := range []string{
		"function patternAtCell(cell)",
		"(hack.patterns || [])",
		"pattern.row === row && pattern.start === offset",
		"const pattern = patternAtCell(cell)",
		"const relatedPattern = patternAtCell(related)",
		"offset >= pattern.start && offset <= pattern.end",
		"`[data-row=\"${pattern.row}\"][data-offset]`",
		"candidate.id === presentation.patternId && !candidate.used",
		"setHackPatternHover(pattern || null)",
		"beginPattern(pattern.id)",
		"beginGuess(cell.dataset.target)",
	} {
		assert.Falsef(t, !strings.Contains(playerScript, required),
			"bundled player is missing pattern interaction contract %q", required)

	}
	for _, forbidden := range []string{
		"offset >= pattern.start && offset <= pattern.end\n  )",
		"matches.find(pattern => pattern.start === offset)",
		"pattern.start > nearest.start",
		"pattern.column",
		"pattern.pair",
		"column.text.slice(pattern.start",
		"data-column=\"${pattern.column}",
	} {
		assert.Falsef(t, strings.Contains(playerScript, forbidden),
			"bundled player depends on private pattern metadata %q", forbidden)

	}
}

func TestPlayerHackingCamouflageAndDelimiterContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	playerScript := read("frontend/client/client.js")
	for _, required := range []string{
		"const pattern = patternAtCell(cell)",
		"pattern.row === row && pattern.start === offset",
		"if (pattern && !pattern.used)",
		"if (beginPattern(pattern.id)) playEnter();",
		"if (pattern) return;",
		"if (beginGuess(cell.dataset.target)) playEnter();",
		"createHackCell('word', wid, col.text.slice(i, j))",
		"createHackCell('filler', `${colIndex}:${i}`, col.text[i])",
	} {
		assert.Falsef(t, !strings.Contains(playerScript, required),
			"bundled player is missing camouflage interaction contract %q", required)

	}
	for _, forbidden := range []string{
		"function isDelimiterCell(cell)",
		"HACK_DELIMITERS.includes(cell.textContent)",
		"pattern || isDelimiterCell(cell)",
		"classList.add('pattern')",
		"classList.add('valid-pattern')",
		"classList.add('delimiter-decoy')",
		"classList.add('decoy')",
		"data-pattern-valid",
		"data-delimiter-decoy",
	} {
		assert.Falsef(t, strings.Contains(playerScript, forbidden),
			"bundled player exposes persistent pattern validity through %q", forbidden)

	}

	stylesheet := read("frontend/client/client.css")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(stylesheet)
	for _, required := range []string{
		".hcell{cursor:pointer;}",
		".hcell.filler{opacity:.8;}",
		".hcell.hi{background:#57ff6e;color:#021002;text-shadow:none;}",
	} {
		assert.Falsef(t, !strings.Contains(compactCSS, required),
			"player stylesheet is missing static/transient cell styling contract %q", required)

	}
	for _, forbidden := range []string{".pattern{", ".valid-pattern{", ".delimiter-decoy{", ".decoy{"} {
		assert.Falsef(t, strings.Contains(compactCSS, forbidden),
			"player stylesheet exposes persistent delimiter validity through %q", forbidden)

	}
}

func TestOverseerRetainsExclusiveHackSolveControl(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(parts ...string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}
	overseerHTML := read("frontend", "overseer", "src", "index.html")
	overseerJS := read("frontend", "overseer", "src", "overseer.js")
	desktopAPI := read("frontend", "overseer", "src", "desktop-api.js")
	playerJS := read("frontend", "client", "client.js")
	playerHTML := read("frontend", "client", "index.html")
	playerProtocol := read("internal", "player", "handler.go")
	appBoundary := read("app.go")
	for _, required := range []string{
		`id="btnHackSuccess"`,
		"desktopAPI.forceHackSuccess()",
		"h.solved || h.failed",
		"forceHackSuccess: desktopService.ForceHackSuccess",
		"func (app *App) ForceHackSuccess() CommandResult",
	} {
		assert.Falsef(t, !strings.Contains(overseerHTML+overseerJS+desktopAPI+appBoundary, required),
			"overseer bundle is missing solve-control contract %q", required)

	}
	playerSurface := playerJS + playerHTML + playerProtocol
	for _, forbidden := range []string{
		"forceHackSuccess",
		"ForceHackSuccess",
		"btnHackSuccess",
		"HACK_ADMIN",
		"URLSearchParams",
		"location.search",
	} {
		assert.Falsef(t, strings.Contains(playerSurface, forbidden),
			"player surface gained overseer solve authority %q", forbidden)

	}
}

func TestOverseerRetainsExclusiveFailedHackResetControl(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}
	overseerHTML := read("frontend/overseer/src/index.html")
	overseerCSS := read("frontend/overseer/src/overseer.css")
	overseerJS := read("frontend/overseer/src/overseer.js")
	desktopAPI := read("frontend/overseer/src/desktop-api.js")
	appBoundary := read("app.go")
	contract := read("specs/004-player-sessions-control/contracts/desktop-coordination.md")
	for _, required := range []string{
		`id="btnResetFailedHack"`,
		`ПОВТОРИТЬ ВЗЛОМ`,
		`desktopAPI.resetFailedHack(`,
		`resetFailedHack: desktopService.ResetFailedHack`,
		`func (app *App) ResetFailedHack(`,
		`ResetFailedHack`,
	} {
		assert.Falsef(t, !strings.Contains(overseerHTML+overseerCSS+overseerJS+desktopAPI+appBoundary+contract, required),
			"overseer bundle is missing failed-hack reset contract %q", required)

	}
	playerSurface := strings.Join([]string{
		read("frontend/client/index.html"), read("frontend/client/client.css"), read("frontend/client/client.js"),
		read("internal/player/handler.go"), read("internal/player/server.go"),
	}, "\n")
	for _, forbidden := range []string{"ResetFailedHack", "resetFailedHack", "btnResetFailedHack", "HACK_RESET", "URLSearchParams", "location.search"} {
		assert.Falsef(t, strings.Contains(playerSurface, forbidden),
			"player surface gained failed-hack reset authority %q", forbidden)

	}
}

func TestOverseerTerminalSwitchDecisionDialogIsAccessibleAndPrivate(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}
	openingTag := func(document, id string) string {
		t.Helper()
		marker := `id="` + id + `"`
		markerAt := strings.Index(document, marker)
		if markerAt < 0 {
			return ""
		}
		start := strings.LastIndex(document[:markerAt], "<")
		endOffset := strings.Index(document[markerAt:], ">")
		if start < 0 || endOffset < 0 {
			return ""
		}
		return document[start : markerAt+endOffset+1]
	}

	overseerHTML := read("frontend/overseer/src/index.html")
	dialogTag := openingTag(overseerHTML, "terminalSwitchDialog")
	for _, fragment := range []string{
		"<dialog",
		`id="terminalSwitchDialog"`,
		`aria-modal="true"`,
		`aria-labelledby="terminalSwitchDialogTitle"`,
		`aria-describedby="terminalSwitchDialogDescription terminalSwitchStatus terminalSwitchError"`,
		"hidden",
	} {
		assert.Falsef(t, !strings.Contains(dialogTag, fragment),
			"terminal-switch dialog opening tag is missing %q; got %q", fragment, dialogTag)

	}
	for _, fragment := range []string{
		`id="terminalSwitchDialogTitle"`,
		`id="terminalSwitchDialogDescription"`,
		`id="terminalSwitchStatus" role="status" aria-live="polite"`,
		`id="terminalSwitchError" role="alert" aria-live="assertive"`,
	} {
		assert.Falsef(t, !strings.Contains(overseerHTML, fragment),
			"terminal-switch dialog is missing accessible feedback contract %q", fragment)

	}
	{
		errorTag := openingTag(overseerHTML, "terminalSwitchError")
		assert.Falsef(t, !strings.Contains(errorTag, "hidden"),
			"terminal-switch error must be hidden until populated; got %q", errorTag)
	}

	for id, decision := range map[string]string{
		"btnPreserveTerminalSwitch": "preserve",
		"btnDiscardTerminalSwitch":  "discard",
		"btnCancelTerminalSwitch":   "cancel",
	} {
		tag := openingTag(overseerHTML, id)
		for _, fragment := range []string{
			"<button",
			`type="button"`,
			`data-switch-decision="` + decision + `"`,
		} {
			assert.Falsef(t, !strings.Contains(tag, fragment),
				"terminal-switch %s control is missing %q; got %q", decision, fragment, tag)

		}
	}
	discardTag := openingTag(overseerHTML, "btnDiscardTerminalSwitch")
	for _, className := range []string{"btn-danger", "terminal-switch-discard"} {
		assert.Falsef(t, !strings.Contains(discardTag, className),
			"discard must carry destructive emphasis class %q; got %q", className, discardTag)

	}

	overseerCSS := read("frontend/overseer/src/overseer.css")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(overseerCSS)
	for _, fragment := range []string{
		".terminal-switch-dialog{",
		".terminal-switch-dialog[hidden]{display:none;}",
		".terminal-switch-discard{",
	} {
		assert.Falsef(t, !strings.Contains(compactCSS, fragment),
			"overseer stylesheet is missing terminal-switch visibility/destructive contract %q", fragment)

	}

	playerSurface := strings.Join([]string{
		read("frontend/client/index.html"),
		read("frontend/client/client.css"),
		read("frontend/client/client.js"),
		read("internal/player/handler.go"),
	}, "\n")
	for _, forbidden := range []string{
		"terminalSwitchDialog",
		"btnPreserveTerminalSwitch",
		"btnDiscardTerminalSwitch",
		"btnCancelTerminalSwitch",
		"data-switch-decision",
		"resolveTerminalSwitch",
		"ResolveTerminalSwitch",
		"switchId",
		"ForceHackSuccess",
		"forceHackSuccess",
	} {
		assert.Falsef(t, strings.Contains(playerSurface, forbidden),
			"player surface exposes overseer switch/private-puzzle capability %q", forbidden)

	}
}

func TestOverseerEndBroadcastDialogIsAccessibleAndAvoidsNativeConfirm(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}
	openingTag := func(document, id string) string {
		t.Helper()
		marker := `id="` + id + `"`
		markerAt := strings.Index(document, marker)
		if markerAt < 0 {
			return ""
		}
		start := strings.LastIndex(document[:markerAt], "<")
		endOffset := strings.Index(document[markerAt:], ">")
		if start < 0 || endOffset < 0 {
			return ""
		}
		return document[start : markerAt+endOffset+1]
	}

	overseerHTML := read("frontend/overseer/src/index.html")
	overseerJS := read("frontend/overseer/src/overseer.js")
	overseerCSS := read("frontend/overseer/src/overseer.css")
	dialogTag := openingTag(overseerHTML, "endBroadcastDialog")
	for _, fragment := range []string{
		"<dialog",
		`id="endBroadcastDialog"`,
		`aria-modal="true"`,
		`aria-labelledby="endBroadcastDialogTitle"`,
		`aria-describedby="endBroadcastDialogDescription"`,
		"hidden",
	} {
		assert.Falsef(t, !strings.Contains(dialogTag, fragment),
			"end-broadcast dialog opening tag is missing %q; got %q", fragment, dialogTag)

	}
	for _, id := range []string{"endBroadcastDialogTitle", "endBroadcastDialogDescription", "btnCancelEndBroadcast", "btnConfirmEndBroadcast"} {
		assert.Falsef(t, !strings.Contains(overseerHTML, `id="`+id+`"`),
			"end-broadcast dialog is missing %q", id)

	}
	for _, id := range []string{"btnCancelEndBroadcast", "btnConfirmEndBroadcast"} {
		{
			tag := openingTag(overseerHTML, id)
			assert.Falsef(t, !strings.Contains(tag, `type="button"`),
				"end-broadcast control %q must be an explicit button; got %q", id, tag)
		}

	}
	for _, fragment := range []string{
		"showEndBroadcastConfirmation()",
		"desktopAPI.endBroadcast()",
		"!result.state || result.state.broadcast",
		"btnConfirmEndBroadcast.disabled = true",
	} {
		assert.Falsef(t, !strings.Contains(overseerJS, fragment),
			"overseer script is missing end-broadcast contract %q", fragment)

	}
	assert.False(t, strings.Contains(overseerJS, "window.confirm('Завершить текущую трансляцию"),
		"end-broadcast action still depends on the native window.confirm gate")

	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(overseerCSS)
	assert.False(t, !strings.Contains(compactCSS, ".end-broadcast-actions{"),
		"overseer stylesheet is missing end-broadcast confirmation layout")

}

func TestOverseerTerminalActionsExposeCanonicalContextAndAccessibleDialogs(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err)
		return string(raw)
	}
	openingTag := func(document, id string) string {
		t.Helper()
		marker := `id="` + id + `"`
		markerAt := strings.Index(document, marker)
		if markerAt < 0 {
			return ""
		}
		start := strings.LastIndex(document[:markerAt], "<")
		endOffset := strings.Index(document[markerAt:], ">")
		if start < 0 || endOffset < 0 {
			return ""
		}
		return document[start : markerAt+endOffset+1]
	}

	overseerHTML := read("frontend/overseer/src/index.html")
	overseerJS := read("frontend/overseer/src/overseer.js")
	overseerCSS := read("frontend/overseer/src/overseer.css")
	overseerSurface := strings.Join([]string{overseerHTML, overseerJS, overseerCSS}, "\n")

	for _, required := range []string{
		"+ СОЗДАТЬ ТЕРМИНАЛ",
		"СДЕЛАТЬ АКТИВНЫМ",
		"ОПУБЛИКОВАТЬ ИЗМЕНЕНИЯ",
		"СНЯТЬ С ЭФИРА",
		"ПЕРЕПРИМЕНИТЬ НАСТРОЙКИ",
		"В ЭФИРЕ",
		`id="terminalSettingsMenu"`,
		`id="btnReapplySettings"`,
		`id="createTerminalDialog"`,
		`id="takeOffAirDialog"`,
		"Игроки перестанут видеть активный терминал.",
		"Трансляция, подключения, роли, назначения и сохранённый терминал останутся без изменений.",
	} {
		assert.Contains(t, overseerSurface, required,
			"Overseer terminal-action surface is missing %q", required)
	}
	for _, superseded := range []string{
		"+ НОВЫЙ ТЕРМИНАЛ",
		"ОБНОВИТЬ АКТИВНЫЙ",
		"ОБНОВИТЬ У ИГРОКОВ",
		"УБРАТЬ АКТИВНЫЙ ТЕРМИНАЛ",
	} {
		assert.NotContains(t, overseerSurface, superseded,
			"Overseer terminal-action surface retains superseded label %q", superseded)
	}

	for id, labelledBy := range map[string]string{
		"createTerminalDialog": "createTerminalDialogTitle",
		"takeOffAirDialog":     "takeOffAirDialogTitle",
	} {
		tag := openingTag(overseerHTML, id)
		assert.Contains(t, tag, "<dialog")
		assert.Contains(t, tag, `aria-modal="true"`)
		assert.Contains(t, tag, `aria-labelledby="`+labelledBy+`"`)
		assert.Contains(t, tag, "hidden")
	}
	for _, id := range []string{
		"createTerminalName",
		"createTerminalError",
		"btnCancelCreateTerminal",
		"btnConfirmCreateTerminal",
		"takeOffAirError",
		"btnCancelTakeOffAir",
		"btnConfirmTakeOffAir",
	} {
		assert.Contains(t, overseerHTML, `id="`+id+`"`,
			"Overseer terminal-action dialog is missing %q", id)
	}
	for _, id := range []string{"createTerminalError", "takeOffAirError"} {
		tag := openingTag(overseerHTML, id)
		assert.Contains(t, tag, `role="alert"`)
		assert.Contains(t, tag, `aria-live="assertive"`)
		assert.Contains(t, tag, "hidden")
	}

	playerSurface := strings.Join([]string{
		read("frontend/client/index.html"),
		read("frontend/client/client.css"),
		read("frontend/client/client.js"),
	}, "\n")
	for _, privateControl := range []string{
		"btnReapplySettings",
		"createTerminalDialog",
		"takeOffAirDialog",
		"RequestTerminalActivation",
		"RequestTerminalClear",
		"UpdateLiveTerminal",
	} {
		assert.NotContains(t, playerSurface, privateControl,
			"player surface exposes private Overseer action %q", privateControl)
	}
}

func TestPlayerHackingColumnFontFitContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	css := read("frontend/client/client.css")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(css)
	for _, fragment := range []string{
		"--font-hack:var(--terminal-scale);",
		"--hack-row-font:var(--font-hack);",
		".hack-row{display:flex;gap:clamp(4px,.8vw,12px);font-size:var(--hack-row-font);",
	} {
		assert.Falsef(t, !strings.Contains(compactCSS, fragment),
			"player stylesheet is missing shared hacking-row fit contract %q", fragment)

	}
	assert.False(t, !strings.Contains(css, "'Fixedsys', 'Consolas', monospace"),
		"hacking-row fit must retain the production fallback font stack for metric remeasurement")

	js := read("frontend/client/client.js")
	for _, fragment := range []string{
		"function hackRowsFitColumns(board = hackBoard)",
		"const tolerance = 0.5",
		"finalBounds.right <= columnBounds.right + tolerance",
		"rowBounds.bottom <= columnBounds.bottom + tolerance",
		"function fitHackRowFont(board = hackBoard)",
		"board.style.removeProperty('--hack-row-font')",
		"let low = baseSize",
		"Math.min(...columns.map(column => column.getBoundingClientRect().width))",
		"hackRowsFitColumns(board) && !hackContentOverflows(board)",
		"while (high - low > 0.25)",
		"board.style.setProperty('--hack-row-font', `${size}px`)",
		"const fontSize = fitHackRowFont(board)",
		"window.addEventListener('resize', scheduleHackFit)",
		"hackFitObserver.observe(termBody)",
		"document.fonts.ready.then(scheduleHackFit)",
		"if (hackFitFrame !== null) cancelAnimationFrame(hackFitFrame)",
	} {
		assert.Falsef(t, !strings.Contains(js, fragment),
			"player script is missing column-aware hacking-row fit contract %q", fragment)

	}
	assert.False(t, strings.Contains(js, "hackFitObserver.observe(hackBoard)") || strings.Contains(js, "hackFitObserver.observe(hackColumns)"),
		"hacking-row fit must not observe its own font-sized descendants and create a resize feedback loop")

}

func TestPlayerCRTVisualShellAssetContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err)
		return string(raw)
	}

	html := read("frontend/client/index.html")
	css := read("frontend/client/client.css")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(css)

	for _, fragment := range []string{
		`class="crt"`,
		`class="screen" id="screen"`,
		`class="scanlines" aria-hidden="true"`,
		`class="vignette" aria-hidden="true"`,
		`class="conn-overlay" id="connOverlay"`,
		`default-src 'self'`,
		`object-src 'none'`,
		`<link rel="icon" href="data:,">`,
	} {
		assert.Contains(t, html, fragment)
	}
	assert.NotContains(t, html, `frame-ancestors`,
		"player HTML meta CSP must omit directives that require HTTP-header delivery")

	for _, fragment := range []string{
		".screen{position:relative;",
		"background:#020a02;",
		"border:2pxsolid#0c2e0c;",
		"color:#57ff6e;",
		".scanlines{position:absolute;inset:0;border-radius:inherit;pointer-events:none;",
		".vignette{position:absolute;inset:0;border-radius:inherit;pointer-events:none;",
		".term-row.sel{background:#57ff6e;color:#021002;text-shadow:none;}",
		".hcell.hi{background:#57ff6e;color:#021002;text-shadow:none;}",
		".character-option:hover,.character-option:focus-visible{border-color:#d8ffb8;background:rgba(87,255,110,.16);outline:none;}",
		".back-btn:focus-visible,.page-btn:focus-visible{outline:2pxsolid#d8ffb8;",
	} {
		assert.Contains(t, compactCSS, fragment)
	}

	for _, forbidden := range []string{"@wailsio/runtime", "window.desktopAPI", "electronAPI", "genericDispatch"} {
		assert.NotContains(t, html+css, forbidden)
	}
}

func TestPlayerCRTMotionAndRevealAssetContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err)
		return string(raw)
	}

	css := read("frontend/client/client.css")
	js := read("frontend/client/client.js")
	compactCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(css)

	for _, fragment := range []string{
		"animation:flicker6sinfinite;",
		"@keyframesflicker{0%,96%,100%{opacity:1;}97%{opacity:.92;}98%{opacity:1;}99%{opacity:.96;}}",
		".blink{animation:blink1ssteps(1)infinite;}",
		"@keyframesblink{0%,49%{opacity:1;}50%,100%{opacity:0;}}",
		"animation:selection-pending1ssteps(2)infinite;",
		"@keyframesselection-pending{0%,49%{border-color:#57ff6e;}50%,100%{border-color:#144d18;}}",
	} {
		assert.Contains(t, compactCSS, fragment)
	}
	assert.NotContains(t, strings.ToLower(css), "prefers-reduced-motion")

	for _, fragment := range []string{
		"const REVEAL_DELAY_MS = 40",
		"const activeRevealControllers = new Set()",
		"container._revealGeneration",
		"controller.complete",
		"controller.cancel",
		"container.replaceChildren()",
		"lastRenderedFolderKey",
		"lastRenderedEntryId",
		"lastRenderedCommandKey",
	} {
		assert.Contains(t, js, fragment)
	}
	assert.NotContains(t, strings.ToLower(js), "prefers-reduced-motion")
}

func TestPlayerCRTHackingRevealAssetContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "frontend", "client", "client.js"))
	require.NoError(t, err)
	js := string(raw)

	for _, fragment := range []string{
		"let lastRenderedHackKey",
		"function hackRevealIdentity(hackState)",
		"function createHackCell(className, target, text)",
		"cell.textContent = text",
		"function buildHackColumn(col, colIndex, rowBase)",
		"address.textContent = col.addresses[rowIndex] || ''",
		"function revealInto(container, elements, animate, contentIdentity = '', options = {})",
		"const appendElement = options.appendElement ||",
		"revealInto(hackColumns, built.rows, animate, hackKey, {",
		"appendElement: descriptor => descriptor.parent.appendChild(descriptor.row)",
		"cancelReveal(hackColumns)",
	} {
		assert.Contains(t, js, fragment, "player script is missing hacking reveal contract %q", fragment)
	}

	assert.NotContains(t, js, "hackColumns.innerHTML")
	assert.NotContains(t, js, "function buildColumnHtml")
}

func TestPlayerCRTHackingRevealFontStabilityAssetContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "frontend", "client", "client.js"))
	require.NoError(t, err)
	js := string(raw)

	for _, fragment := range []string{
		"let hackBoardFit = null",
		"function createHackFitProbe()",
		"probe.inert = true",
		"probe.setAttribute('aria-hidden', 'true')",
		"descriptor.row.cloneNode(true)",
		"function fitCompleteHackBoard()",
		"const fit = applyHackLayout(probe)",
		"hackBoardFit = fit",
		"applyHackFit(hackBoardFit)",
		"window.addEventListener('resize', scheduleHackFit)",
		"document.fonts.ready.then(scheduleHackFit)",
	} {
		assert.Contains(t, js, fragment,
			"player script is missing stable complete-board hacking fit contract %q", fragment)
	}

	assert.NotContains(t, js, "afterAppend: scheduleHackFit",
		"progressive row insertion must not refit against the partial interaction DOM")
}

func TestPlayerCRTDudReconciliationAssetContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "frontend", "client", "client.js"))
	require.NoError(t, err)
	js := string(raw)

	for _, fragment := range []string{
		"let lastRenderedHackRows = new Map()",
		"function hackBoardSnapshot(hackState)",
		"function reconcileHackRow(current, replacement)",
		"function reconcileHackColumns(hackState)",
		"if (current.row.isConnected)",
		"current.row.replaceChildren(...replacement.row.childNodes)",
		"current.row = replacement.row",
		"if (!animate && lastRenderedHackRows.size !== 0)",
	} {
		assert.Contains(t, js, fragment, "player script is missing dud reconciliation contract %q", fragment)
	}

	identityStart := strings.Index(js, "function hackRevealIdentity(hackState)")
	identityEnd := strings.Index(js, "function createHackCell(className, target, text)")
	require.NotEqual(t, -1, identityStart)
	require.Greater(t, identityEnd, identityStart)
	identity := js[identityStart:identityEnd]
	assert.NotContains(t, identity, "hackState.columns")
	assert.NotContains(t, identity, "boardText")
}

func TestPlayerCRTRevealSkipAssetContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "frontend", "client", "client.js"))
	require.NoError(t, err)
	js := string(raw)

	for _, fragment := range []string{
		"let consumedRevealKey = null",
		"function completeVisibleReveals()",
		"function consumeRevealKeydown(event)",
		"function releaseConsumedRevealKey(event)",
		"event.preventDefault()",
		"event.stopImmediatePropagation()",
		"event.repeat && key === consumedRevealKey",
		"document.addEventListener('keydown', consumeRevealKeydown, { capture: true })",
		"document.addEventListener('keyup', releaseConsumedRevealKey, { capture: true })",
		"controller.complete()",
		"d.textContent = text",
		"row.textContent = '> ' + node.name",
	} {
		assert.Contains(t, js, fragment)
	}

	for _, forbidden := range []string{
		"localStorage.setItem('crt",
		"localStorage.setItem(\"crt",
		"playerRPC.reveal",
		"innerHTML = node.name",
		"innerHTML = node.description",
		"innerHTML = commandOutput",
	} {
		assert.NotContains(t, js, forbidden)
	}
}

func TestActiveFrontendUsesRuntimeNeutralDesktopFacade(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	adapter, err := os.ReadFile(filepath.Join(root, "frontend", "overseer", "src", "desktop-api.js"))
	if err != nil {
		require.NoError(t, err)
	}
	overseer, err := os.ReadFile(filepath.Join(root, "frontend", "overseer", "src", "overseer.js"))
	if err != nil {
		require.NoError(t, err)
	}

	activeSource := string(adapter) + "\n" + string(overseer)
	assert.False(t, strings.Contains(activeSource, "window.electronAPI") || strings.Contains(activeSource, "'electronAPI'") || strings.Contains(activeSource, `"electronAPI"`),
		"active production frontend still defines or consumes the transitional Electron-specific bridge global")

	for _, required := range []string{
		"window.desktopAPI",
		"Object.defineProperty(window, 'desktopAPI'",
	} {
		assert.Falsef(t, !strings.Contains(activeSource, required),
			"active production frontend is missing runtime-neutral facade contract %q", required)

	}
	adapterSource := string(adapter)
	overseerSource := string(overseer)
	for _, forbidden := range []string{"window.go", "window.runtime", "frontend/wailsjs", "../wailsjs", "CopyDemo", "copyDemo"} {
		assert.NotContains(t, activeSource, forbidden,
			"active production frontend exposes legacy/global or unauthored capability %q", forbidden)
	}
	assert.Contains(t, adapterSource, "import * as desktopService from '../bindings/")
	assert.Contains(t, adapterSource, "import { Clipboard, Events } from '@wailsio/runtime'")
	assert.NotContains(t, overseerSource, "@wailsio/runtime")
	assert.NotContains(t, overseerSource, "desktopService.")
	assert.Contains(t, overseerSource, "const desktopAPI = window.desktopAPI")
	for _, presentation := range []string{"ready-local", "ready-public", "warning", "failed", "startupError", "tunnelError"} {
		assert.Contains(t, overseerSource, presentation,
			"overseer startup presentation is missing existing-status projection %q", presentation)
	}
	assert.NotContains(t, overseerSource, "status.phase")
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

func TestProductionEmbedsOverseerAndPlayerAsSeparateFilesystems(t *testing.T) {
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

	viteConfig, err := os.ReadFile(filepath.Join(root, "frontend", "overseer", "vite.config.js"))
	if err != nil {
		require.NoError(t, err)
	}
	assert.False(t, !strings.Contains(string(viteConfig), `./dist/.keep`),
		"Vite build does not restore frontend/overseer/dist/.keep after emptyOutDir")

}

func TestPackagedPlayerBuildIsCompleteAndOffline(t *testing.T) {
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
		if extension := strings.ToLower(filepath.Ext(path)); extension == ".html" || extension == ".js" || extension == ".css" {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{"https://cdn.", "http://localhost:", "http://127.0.0.1:5173", "@vite/client"} {
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
	assert.Contains(t, readme, "specs/006-wails-v3-migration/quickstart.md")
	assert.Contains(t, readme, "docs/wails-v3-migration-rollback.md")
	assert.Contains(t, readme, "неизменяемые исторические evidence")
	assert.Contains(t, readme, "specs/001-wails-v2-migration/")
	assert.Contains(t, readme, "docs/wails-migration-rollback.md")

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

func TestPlayerSessionsControlCrossCuttingAssetContract(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			require.NoError(t, err)
		}
		return string(raw)
	}

	overseerHTML := read("frontend/overseer/src/index.html")
	overseerJS := read("frontend/overseer/src/overseer.js")
	overseerCSS := read("frontend/overseer/src/overseer.css")
	playerHTML := read("frontend/client/index.html")
	playerJS := read("frontend/client/client.js")
	playerCSS := read("frontend/client/client.css")
	playerHTTP := read("internal/player/http.go")
	playerProtocol := read("internal/player/handler.go")

	for _, directive := range []string{
		`default-src 'self'`,
		`script-src 'self'`,
		`object-src 'none'`,
		`base-uri 'none'`,
		`form-action 'none'`,
	} {
		assert.Falsef(t, !strings.Contains(overseerHTML, directive),
			"overseer CSP is missing restrictive directive %q", directive)

	}
	for _, fragment := range []string{
		`response.Header().Set("Content-Security-Policy", playerContentSecurityPolicy)`,
		`default-src 'self'`,
		`script-src 'self'`,
		`object-src 'none'`,
		`base-uri 'none'`,
		`frame-ancestors 'none'`,
	} {
		assert.Falsef(t, !strings.Contains(playerHTTP, fragment),
			"player HTTP boundary is missing CSP contract %q", fragment)

	}

	for _, fragment := range []string{
		`nameInput.value = character.name || ''`,
		`row.querySelector('.session-primary-name').textContent = assigned`,
		`row.querySelector('.session-character-name').textContent = assigned`,
		`row.querySelector('.session-fallback-label').textContent = `,
	} {
		assert.Falsef(t, !strings.Contains(overseerJS, fragment),
			"overseer asset is missing text-only name rendering %q", fragment)

	}
	for _, fragment := range []string{
		`option.textContent = entry.name`,
		`playerCharacterName.textContent = characterName`,
		`playerFallbackName.textContent = compactPlayerInputLabel(playerState.fallbackName || '')`,
	} {
		assert.Falsef(t, !strings.Contains(playerJS, fragment),
			"player asset is missing text-only name rendering %q", fragment)

	}
	for _, forbidden := range []string{
		"innerHTML = character.name",
		"innerHTML = session.fallbackName",
		"innerHTML = entry.name",
		"innerHTML = playerState.fallbackName",
	} {
		assert.Falsef(t, strings.Contains(overseerJS, forbidden) || strings.Contains(playerJS, forbidden),
			"coordination assets interpolate an unescaped name through %q", forbidden)

	}

	overseerSurface := overseerHTML + "\n" + overseerJS + "\n" + overseerCSS
	for _, forbidden := range []string{"browserToken", "PLAYER_TOKEN_KEY", "fallout-terminal.player-token"} {
		assert.Falsef(t, strings.Contains(overseerSurface, forbidden),
			"overseer assets expose player resume-token detail %q", forbidden)

	}
	for _, fragment := range []string{
		"const PLAYER_TOKEN_KEY = 'fallout-terminal.player-token'",
		"localStorage.getItem(PLAYER_TOKEN_KEY)",
		"localStorage.setItem(PLAYER_TOKEN_KEY",
		"playerRPC.subscribe(request",
	} {
		assert.Falsef(t, !strings.Contains(playerJS, fragment),
			"player token is not confined to the private handshake/storage path %q", fragment)

	}
	for _, forbidden := range []string{"browserToken", "PLAYER_TOKEN_KEY", "?token", "?session"} {
		assert.Falsef(t, strings.Contains(playerHTML+"\n"+playerCSS, forbidden),
			"player document/style surface leaks token detail %q", forbidden)

	}
	for _, forbidden := range []string{"URLSearchParams", "location.search", "searchParams.set('browserToken'", `searchParams.set("browserToken"`} {
		assert.Falsef(t, strings.Contains(playerJS, forbidden),
			"player script exposes resume tokens through the URL via %q", forbidden)

	}

	for _, fragment := range []string{
		"const observerReadOnly = hasState && playerState.role === 'observer'",
		"const blockingSharedInputPending = pendingSharedAction !== null || commandRequestPending || terminalNavigationPending",
		"screen.classList.toggle('observer-read-only', observerReadOnly)",
		"screen.classList.toggle('shared-input-pending', blockingSharedInputPending)",
		"screen.setAttribute('aria-readonly', String(observerReadOnly))",
		"function canControlSharedTerminal()",
		"playerState.role === 'active'",
		"pendingSharedAction === null",
	} {
		assert.Falsef(t, !strings.Contains(playerJS, fragment),
			"player asset is missing observer/local-only action gate %q", fragment)

	}
	assert.NotContains(t, playerJS,
		"pendingSharedAction !== null || pendingPresentationAction !== null || commandRequestPending || terminalNavigationPending",
		"presentation-only correlation must not activate blocking shared-input styling")
	for _, fragment := range []string{
		"#screen.observer-read-only :is(.term-row, .back-btn, .page-btn, .hcell)",
		"#screen.shared-input-pending :is(.term-row, .back-btn, .page-btn, .hcell)",
	} {
		assert.Falsef(t, !strings.Contains(playerCSS, fragment),
			"player stylesheet is missing read-only/pending presentation %q", fragment)

	}

	for _, fragment := range []string{
		"pendingSharedAction.acceptedRevision = Number(result.revision) || 0",
		"if (appliedSharedRevision < pendingSharedAction.acceptedRevision) return",
		"pendingPresentationAction.acceptedRevision = Number(result.revision) || 0",
		"if (appliedSharedRevision < pendingPresentationAction.acceptedRevision) return",
		"let desiredPresentationAction = null",
		"if (pendingPresentationAction || !desiredPresentationAction) return",
		"desiredPresentationAction = presentation",
		"scheduleDesiredPresentationDispatch()",
		"presentation.contextKey !== controllerPresentation.contextKey",
		"playerState.revision >= pendingSelection.acceptedRevision",
	} {
		assert.Falsef(t, !strings.Contains(playerJS, fragment),
			"player asset clears pending input without authoritative revision evidence %q", fragment)

	}
	for _, fragment := range []string{
		"pendingTerminalSwitch = result?.switchId || null",
		"desktopAPI.resolveTerminalSwitch({ switchId: pendingTerminalSwitch, decision })",
		"if (!pendingTerminalSwitch || coordinationCommandPending) return",
	} {
		assert.Falsef(t, !strings.Contains(overseerJS, fragment),
			"overseer asset is missing terminal-switch resolution contract %q", fragment)

	}
	startBroadcast := strings.Index(overseerJS, "btnStartBroadcast.addEventListener")
	require.NotEqual(t, -1, startBroadcast, "overseer asset is missing the start-broadcast handler")
	endBroadcast := strings.Index(overseerJS[startBroadcast:], "btnEndBroadcast.addEventListener")
	require.NotEqual(t, -1, endBroadcast, "overseer asset is missing the end-broadcast handler")
	startBroadcastHandler := overseerJS[startBroadcast : startBroadcast+endBroadcast]
	assert.Contains(t, startBroadcastHandler, "renderTreeHeader()",
		"start-broadcast success must refresh terminal controls after authoritative broadcast state changes")
	for _, fragment := range []string{
		`id="playerConfigStatus"`,
		`id="btnOpenPlayerConfig"`,
		`id="btnNewPlayerConfig"`,
		`id="playerConfigError"`,
	} {
		assert.Falsef(t, !strings.Contains(overseerHTML, fragment),
			"overseer asset is missing player-config recovery control %q", fragment)

	}
	for _, fragment := range []string{
		`.player-management-dialog[aria-readonly="true"] .player-management-mode`,
		`.player-config-error[hidden]`,
	} {
		assert.Falsef(t, !strings.Contains(overseerCSS, fragment),
			"overseer stylesheet is missing player-config gating contract %q", fragment)

	}

	compactPlayerCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(playerCSS)
	compactOverseerCSS := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(overseerCSS)
	for _, fragment := range []string{
		"--terminal-scale:clamp(",
		".screen{position:relative;width:100%;max-width:1500px;height:100%;",
		"@media(max-width:760px){",
		"@media(max-height:720px){",
	} {
		assert.Falsef(t, !strings.Contains(compactPlayerCSS, fragment),
			"player stylesheet is missing responsive layout boundary %q", fragment)

	}
	for _, fragment := range []string{
		"@media(max-width:1050px){",
		"@media(max-width:820px){",
		"@media(max-width:620px),(max-height:560px){",
		".terminal-switch-dialog{width:calc(100vw-20px);max-height:calc(100dvh-20px);",
	} {
		assert.Falsef(t, !strings.Contains(compactOverseerCSS, fragment),
			"overseer stylesheet is missing responsive layout boundary %q", fragment)

	}

	playerSurface := strings.Join([]string{playerHTML, playerCSS, playerJS, playerProtocol}, "\n")
	for _, forbidden := range []string{
		"ForceHackSuccess",
		"forceHackSuccess",
		"HACK_ADMIN",
		"btnHackSuccess",
		"resolveTerminalSwitch",
		"ResolveTerminalSwitch",
	} {
		assert.Falsef(t, strings.Contains(playerSurface, forbidden),
			"player surface exposes private overseer capability %q", forbidden)

	}
}

func TestPlayerBundleImportsOnlyPublicGeneratedContractsAndNoGenericPrivateCarriers(t *testing.T) {
	t.Parallel()
	root := assetRepositoryRoot(t)
	for _, relative := range []string{"frontend/client/client.js", "frontend/client/sound.js", "frontend/client/index.html"} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			require.NoError(t, err)
		}
		source := strings.ToLower(string(raw))
		for _, forbidden := range []string{
			"fallout/terminal/private", "fallout/terminal/persistence", "protojson", "base64",
			"genericdispatch", "generic-dispatch", "forcehacksuccess", "resetfailedhack",
			"@wailsio/runtime", "wailsjs", "window.desktopapi", "window.runtime", "websocket(",
		} {
			assert.Falsef(t, strings.Contains(source, forbidden),
				"%s imports or carries private desktop semantic %q", relative, forbidden)

		}
	}
}

func TestOneProtocolCutoverHasNoActiveLegacyPlayerSurface(t *testing.T) {
	t.Parallel()

	root := assetRepositoryRoot(t)
	paths := []string{
		"frontend/client/client.js", "frontend/client/sound.js", "frontend/client/index.html",
		"internal/player", "internal/testutil/testdata", "tests/browser/fixture-server",
		"README.md", "docs",
	}
	legacyIdentifiers := []string{
		"SESSION_HELLO", "CHARACTER_SELECT", "NAV_ACTION", "HACK_GUESS", "HACK_PATTERN",
		"SESSION_WELCOME", "PLAYER_STATE", "ACTION_RESULT", "TERMINAL_LIVE", "TERMINAL_UPDATE",
		"TERMINAL_CLEAR", "NAV_STATE", "HACK_STATE", "HACK_ADMIN",
	}
	for _, relative := range paths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil {
			require.NoError(t, err)
		}
		var files []string
		if info.IsDir() {
			err = filepath.WalkDir(path, func(candidate string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !entry.IsDir() && !strings.Contains(filepath.ToSlash(candidate), "/gen/") && !strings.Contains(filepath.ToSlash(candidate), "/dist/") && !strings.Contains(filepath.ToSlash(candidate), "/node_modules/") {
					files = append(files, candidate)
				}
				return nil
			})
			if err != nil {
				require.NoError(t, err)
			}
		} else {
			files = []string{path}
		}
		for _, candidate := range files {
			raw, err := os.ReadFile(candidate)
			if err != nil {
				require.NoError(t, err)
			}
			content := string(raw)
			lower := strings.ToLower(content)
			for _, forbidden := range []string{"github.com/coder/websocket", "new websocket", "fakewebsocket", "/api/sounds/", "connect-src 'self' ws:", "connect-src 'self' wss:"} {
				assert.Falsef(t, strings.Contains(lower, forbidden),
					"active cutover surface %s contains %q", candidate, forbidden)

			}
			for _, forbidden := range legacyIdentifiers {
				assert.Falsef(t, strings.Contains(content, forbidden),
					"active cutover surface %s contains legacy identifier %q", candidate, forbidden)

			}
			assert.Falsef(t, strings.Contains(content, "dispatch(msg") || strings.Contains(content, "send({ type:"),
				"active player surface %s retains a generic message dispatcher", candidate)

		}
	}

	moduleRaw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	module := string(moduleRaw)
	assert.False(t, regexp.MustCompile(`(?m)^\s*github\.com/coder/websocket\s+v\S+\s*$`).MatchString(module),
		"the application must not directly depend on the removed public WebSocket transport")
	if strings.Contains(module, "github.com/coder/websocket") {
		assert.Contains(t, module, "github.com/coder/websocket v1.8.14 // indirect",
			"only Wails v3's pinned private runtime transitive dependency may retain coder/websocket")
	}
	{

		_, err := os.Stat(filepath.Join(root, "internal", "player", "protocol.go"))
		assert.Falsef(t, !errors.Is(err, os.ErrNotExist),
			"legacy handwritten protocol implementation still exists: %v", err)
	}
	{

		_, err := os.Stat(filepath.Join(root, "internal", "testutil", "testdata", "protocol"))
		assert.Falsef(t, !errors.Is(err, os.ErrNotExist),
			"legacy JSON protocol fixture directory still exists: %v", err)
	}

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
