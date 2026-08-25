package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/internal/domain"
	configv1 "github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/config/v1"
	persistencev1 "github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/persistence/v1"
	privatev1 "github.com/obalunenko/Fallout-Terminal/internal/gen/fallout/terminal/private/v1"
	sessionservice "github.com/obalunenko/Fallout-Terminal/internal/session"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/testing/prototest"
)

func TestPublicAccessDescriptorsAreExactAndSecretSurfacesStayNarrow(t *testing.T) {
	t.Parallel()

	messages := []struct {
		message proto.Message
		fields  []string
	}{
		{&configv1.PublicAccessPreferences{}, []string{"version", "enabled_preference", "reserved_domain", "username", "provider_token_present_hint", "player_password_present_hint", "revision"}},
		{&privatev1.PublicAccessStatus{}, []string{"state", "generation", "settings_revision", "public_url", "error_category", "error_message"}},
		{&privatev1.PublicAccessSnapshot{}, []string{"preferences", "provider_token_presence", "player_password_presence", "status"}},
		{&privatev1.GetPublicAccessRequest{}, []string{}},
		{&privatev1.PublicAccessStatusEvent{}, []string{"snapshot"}},
		{&privatev1.PublicAccessCommandResult{}, []string{"ok", "error", "snapshot"}},
		{&privatev1.SavePublicAccessSettingsRequest{}, []string{"expected_revision", "enabled_preference", "reserved_domain", "username", "replacement_provider_token", "delete_provider_token", "replacement_player_password", "delete_player_password"}},
		{&privatev1.GeneratePlayerPasswordRequest{}, []string{"expected_revision"}},
		{&privatev1.GeneratedPlayerPasswordResult{}, []string{"ok", "error", "generated_password", "settings_revision"}},
		{&privatev1.PublicAccessCommandRequest{}, []string{"expected_revision"}},
	}
	for _, test := range messages {
		descriptor := test.message.ProtoReflect().Descriptor()
		prototest.Message{}.Test(t, test.message.ProtoReflect().Type())
		actual := make([]string, 0, descriptor.Fields().Len())
		for index := range descriptor.Fields().Len() {
			actual = append(actual, string(descriptor.Fields().Get(index).Name()))
		}
		require.Equal(t, test.fields, actual, "descriptor drifted for %s", descriptor.FullName())
	}

	save := (&privatev1.SavePublicAccessSettingsRequest{}).ProtoReflect().Descriptor()
	require.NotNil(t, save.Oneofs().ByName("provider_token_change"))
	require.NotNil(t, save.Oneofs().ByName("player_password_change"))
	nonsyntheticOneofs := 0
	for index := range save.Oneofs().Len() {
		if !save.Oneofs().Get(index).IsSynthetic() {
			nonsyntheticOneofs++
		}
	}
	require.Equal(t, 2, nonsyntheticOneofs)

	assertEnum := func(message proto.Message, field string, names []string) {
		t.Helper()
		descriptor := message.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(field)).Enum()
		actual := make([]string, 0, descriptor.Values().Len())
		for index := range descriptor.Values().Len() {
			value := descriptor.Values().Get(index)
			require.Equal(t, protoreflect.EnumNumber(index), value.Number())
			actual = append(actual, string(value.Name()))
		}
		require.Equal(t, names, actual)
	}
	assertEnum(&privatev1.PublicAccessStatus{}, "state", []string{
		"PUBLIC_ACCESS_LIFECYCLE_STATE_UNSPECIFIED", "PUBLIC_ACCESS_LIFECYCLE_STATE_DISABLED",
		"PUBLIC_ACCESS_LIFECYCLE_STATE_STARTING", "PUBLIC_ACCESS_LIFECYCLE_STATE_READY",
		"PUBLIC_ACCESS_LIFECYCLE_STATE_STOPPING", "PUBLIC_ACCESS_LIFECYCLE_STATE_FAILED",
	})
	assertEnum(&privatev1.PublicAccessSnapshot{}, "provider_token_presence", []string{
		"SECRET_PRESENCE_UNSPECIFIED", "SECRET_PRESENCE_ABSENT", "SECRET_PRESENCE_PRESENT", "SECRET_PRESENCE_UNKNOWN",
	})
	assertEnum(&privatev1.PublicAccessStatus{}, "error_category", []string{
		"PUBLIC_ACCESS_ERROR_CATEGORY_UNSPECIFIED", "PUBLIC_ACCESS_ERROR_CATEGORY_VALIDATION",
		"PUBLIC_ACCESS_ERROR_CATEGORY_SETTINGS_CORRUPT", "PUBLIC_ACCESS_ERROR_CATEGORY_SECRET_STORE_LOCKED",
		"PUBLIC_ACCESS_ERROR_CATEGORY_SECRET_STORE_DENIED", "PUBLIC_ACCESS_ERROR_CATEGORY_SECRET_STORE_UNAVAILABLE",
		"PUBLIC_ACCESS_ERROR_CATEGORY_CREDENTIAL_MISSING", "PUBLIC_ACCESS_ERROR_CATEGORY_PROVIDER_AUTHENTICATION",
		"PUBLIC_ACCESS_ERROR_CATEGORY_DOMAIN_UNAVAILABLE", "PUBLIC_ACCESS_ERROR_CATEGORY_NETWORK_UNAVAILABLE",
		"PUBLIC_ACCESS_ERROR_CATEGORY_TIMEOUT", "PUBLIC_ACCESS_ERROR_CATEGORY_PROVIDER_FAILURE",
		"PUBLIC_ACCESS_ERROR_CATEGORY_SHUTDOWN_TIMEOUT", "PUBLIC_ACCESS_ERROR_CATEGORY_CONFLICT",
	})

	for _, message := range []proto.Message{
		&configv1.PublicAccessPreferences{}, &privatev1.PublicAccessStatus{},
		&privatev1.PublicAccessSnapshot{}, &privatev1.PublicAccessStatusEvent{},
		&privatev1.PublicAccessCommandResult{}, &privatev1.PublicAccessCommandRequest{},
	} {
		descriptor := message.ProtoReflect().Descriptor()
		for index := range descriptor.Fields().Len() {
			name := strings.ToLower(string(descriptor.Fields().Get(index).Name()))
			isOpaquePresence := strings.HasSuffix(name, "_presence") || strings.HasSuffix(name, "_present_hint")
			if !isOpaquePresence {
				require.NotContains(t, name, "password", "%s must be reusable and secret-free", descriptor.FullName())
				require.NotContains(t, name, "token", "%s must be reusable and secret-free", descriptor.FullName())
			}
		}
	}

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(file.Path(), "fallout/terminal/player/v1/") {
			return true
		}
		for index := range file.Imports().Len() {
			path := file.Imports().Get(index).Path()
			require.False(t, strings.Contains(path, "/private/"), "public player schema imports private contract: %s", path)
			require.False(t, strings.Contains(path, "public_access.proto"), "public player schema imports public-access config: %s", path)
		}
		return true
	})
}

func TestLegacyTunnelConfigurationFieldsAreReserved(t *testing.T) {
	t.Parallel()

	file := configv1.File_fallout_terminal_config_v1_config_proto
	assertReserved := func(messageName string, numbers []protoreflect.FieldNumber, names []protoreflect.Name) {
		t.Helper()
		message := file.Messages().ByName(protoreflect.Name(messageName))
		require.NotNil(t, message)
		for _, number := range numbers {
			require.True(t, message.ReservedRanges().Has(number), "%s field %d must be reserved", messageName, number)
			require.Nil(t, message.Fields().ByNumber(number))
		}
		for _, name := range names {
			require.True(t, message.ReservedNames().Has(name), "%s field %s must be reserved", messageName, name)
			require.Nil(t, message.Fields().ByName(name))
		}
	}
	assertReserved("TunnelCredentials", []protoreflect.FieldNumber{1, 2}, []protoreflect.Name{"username", "password"})
	assertReserved("TunnelConfig", []protoreflect.FieldNumber{1, 2, 3, 4, 5, 6, 7, 8}, []protoreflect.Name{
		"enabled", "binary", "domain", "port", "local_url", "startup_timeout_milliseconds", "policy_parent", "credentials",
	})
	assertReserved("ApplicationConfig", []protoreflect.FieldNumber{1, 6}, []protoreflect.Name{"tunnel_enabled", "tunnel"})
}

func TestPrivateDescriptorFieldsAndEnumsHaveExplicitAdapterCoverage(t *testing.T) {
	coverage := []struct {
		message proto.Message
		fields  []string
	}{
		{&privatev1.CommandResult{}, []string{"ok", "error"}},
		{&privatev1.SessionOperationResult{}, []string{"ok", "canceled", "error", "file_path", "session"}},
		{&privatev1.SaveSessionResult{}, []string{"ok", "error", "requested_revision", "saved_revision"}},
		{&privatev1.PlayerConfigOperationResult{}, []string{"ok", "canceled", "error", "player_config", "session", "state", "player_config_metadata"}},
		{&privatev1.CoordinationResult{}, []string{"ok", "error", "state"}},
		{&privatev1.TerminalSwitchResult{}, []string{"ok", "error", "status", "switch_id", "state"}},
		{&privatev1.TerminalActivationRequest{}, []string{"terminal_id", "terminal_name", "tree", "hack_level", "intro_text"}},
		{&privatev1.LiveTerminalUpdateRequest{}, []string{"tree", "intro_text"}},
		{&privatev1.TerminalSwitchDecisionRequest{}, []string{"switch_id", "choice"}},
		{&privatev1.ResolveCommandExecutionRequest{}, []string{"request_id", "decision"}},
		{&privatev1.ResolveCommandExecutionResult{}, []string{"ok", "error", "state"}},
		{&privatev1.ResolveTerminalNavigationRequest{}, []string{"request_id", "decision"}},
		{&privatev1.ResolveTerminalNavigationResult{}, []string{"ok", "error", "state"}},
		{&privatev1.ResetCommandStateRequest{}, []string{"terminal_id", "command_id"}},
		{&privatev1.ResetTerminalCommandStatesRequest{}, []string{"terminal_id"}},
		{&privatev1.SessionStateResult{}, []string{"ok", "error", "revision", "session"}},
		{&privatev1.ReplaceTerminalGroupsRequest{}, []string{"terminal_groups", "expected_session_revision", "expected_coordination_revision"}},
		{&privatev1.ReplaceTerminalGroupsResult{}, []string{"ok", "error", "session_revision", "session", "coordination_state"}},
		{&privatev1.ResetFailedHackRequest{}, []string{"terminal_id", "terminal_name", "tree", "hack_level", "intro_text"}},
		{&privatev1.AddCharacterRequest{}, []string{"display_name", "intelligence", "hacker_perk_available", "expected_revision"}},
		{&privatev1.RenameCharacterRequest{}, []string{"character_id", "display_name", "intelligence", "hacker_perk_available", "expected_revision"}},
		{&privatev1.DeleteCharacterRequest{}, []string{"character_id", "expected_revision"}},
		{&privatev1.RenameLogicalSessionRequest{}, []string{"logical_session_id", "fallback_name"}},
		{&privatev1.AssignCharacterRequest{}, []string{"logical_session_id", "character_id"}},
		{&privatev1.ReleaseCharacterRequest{}, []string{"logical_session_id"}},
		{&privatev1.MoveCharacterRequest{}, []string{"character_id", "destination_session_id"}},
		{&privatev1.SetActiveControllerRequest{}, []string{"logical_session_id"}},
		{&privatev1.OpenUrlRequest{}, []string{"url"}},
		{&privatev1.ServerInformation{}, []string{"local_url", "public_url", "tunnel_enabled", "ip", "port", "tunnel_error", "url"}},
		{&privatev1.RuntimeStatus{}, []string{"server_info", "client_count", "hack_state", "startup_error", "save_state", "requested_revision", "saved_revision", "coordination_state"}},
		{&privatev1.ServerInformationEvent{}, []string{"server_info"}},
		{&privatev1.ClientCountEvent{}, []string{"client_count"}},
		{&privatev1.HackStateEvent{}, []string{"hack_state"}},
		{&privatev1.CoordinationStateEvent{}, []string{"coordination_state"}},
		{&privatev1.SessionStateEvent{}, []string{"revision", "session"}},
		{&privatev1.CharacterState{}, []string{"character_id", "display_name", "logical_session_id", "intelligence", "hacker_perk_available"}},
		{&privatev1.LogicalSessionState{}, []string{"logical_session_id", "fallback_name", "connected", "active_streams", "character_id", "role"}},
		{&privatev1.BroadcastState{}, []string{"broadcast_id", "active_controller_session_id", "active_terminal_id", "revision"}},
		{&privatev1.PendingTerminalSwitch{}, []string{"switch_id", "terminal_id", "terminal_name", "requested_terminal", "broadcast_id", "source_terminal_id", "target_terminal_id"}},
		{&privatev1.PendingCommandExecution{}, []string{"request_id", "broadcast_id", "terminal_id", "command_id", "command_name", "confirmation_text", "command_mode"}},
		{&privatev1.PendingTerminalNavigation{}, []string{"request_id", "broadcast_id", "direction", "source_terminal_id", "source_terminal_name", "command_id", "command_name", "target_terminal_id", "target_terminal_name", "route_depth"}},
		{&privatev1.TerminalNavigationNotice{}, []string{"reason", "source_terminal_id", "command_id", "target_terminal_id"}},
		{&privatev1.PlayerConfigMetadata{}, []string{"status", "file_path", "version", "name"}},
		{&privatev1.CoordinationState{}, []string{"roster", "logical_sessions", "broadcast", "pending_terminal_switch", "revision", "player_config", "pending_command_execution", "pending_terminal_navigation", "terminal_navigation_notice"}},
	}
	for _, test := range coverage {
		descriptor := test.message.ProtoReflect().Descriptor()
		prototest.Message{}.Test(t, test.message.ProtoReflect().Type())
		actual := make([]string, 0, descriptor.Fields().Len())
		for index := 0; index < descriptor.Fields().Len(); index++ {
			actual = append(actual, string(descriptor.Fields().Get(index).Name()))
		}
		require.ElementsMatch(t, test.fields, actual, "adapter coverage drifted for %s", descriptor.FullName())
	}
	require.Equal(t, privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_UNSPECIFIED, terminalSwitchStatusToPrivate("unknown"))
	require.Empty(t, terminalSwitchStatusFromPrivate(privatev1.TerminalSwitchStatus_TERMINAL_SWITCH_STATUS_UNSPECIFIED))
	require.Empty(t, terminalSwitchChoiceFromPrivate(privatev1.TerminalSwitchChoice_TERMINAL_SWITCH_CHOICE_UNSPECIFIED))

	decision := (&privatev1.ResolveCommandExecutionRequest{}).ProtoReflect().Descriptor().Fields().ByName("decision").Enum()
	require.Equal(t, []string{
		"COMMAND_EXECUTION_DECISION_UNSPECIFIED",
		"COMMAND_EXECUTION_DECISION_APPROVE",
		"COMMAND_EXECUTION_DECISION_REJECT",
	}, descriptorEnumNames(decision))
	navigationDecision := (&privatev1.ResolveTerminalNavigationRequest{}).ProtoReflect().Descriptor().Fields().ByName("decision").Enum()
	require.Equal(t, []string{
		"TERMINAL_NAVIGATION_DECISION_UNSPECIFIED",
		"TERMINAL_NAVIGATION_DECISION_APPROVE",
		"TERMINAL_NAVIGATION_DECISION_REJECT",
	}, descriptorEnumNames(navigationDecision))
}

func TestTerminalGroupMutationContractIsPrivateAndRoundTripsCompleteCandidate(t *testing.T) {
	t.Parallel()

	request := &privatev1.ReplaceTerminalGroupsRequest{
		TerminalGroups: []*persistencev1.TerminalGroup{
			{Id: "group-alpha", Name: "Alpha", TerminalIds: []string{"terminal-a", "terminal-b"}},
			{Id: "group-charlie", Name: "Charlie", TerminalIds: []string{"terminal-c"}},
		},
		ExpectedSessionRevision:      17,
		ExpectedCoordinationRevision: 29,
	}
	cloned := proto.Clone(request).(*privatev1.ReplaceTerminalGroupsRequest)
	require.True(t, proto.Equal(request, cloned))
	require.Equal(t, []string{"terminal-a", "terminal-b"}, cloned.GetTerminalGroups()[0].GetTerminalIds())
	require.Equal(t, uint64(17), cloned.GetExpectedSessionRevision())
	require.Equal(t, uint64(29), cloned.GetExpectedCoordinationRevision())

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(file.Path(), "fallout/terminal/player/v1/") {
			return true
		}
		for index := range file.Messages().Len() {
			name := strings.ToLower(string(file.Messages().Get(index).Name()))
			require.NotContains(t, name, "terminalgroup")
			require.NotContains(t, name, "replacegroup")
		}
		return true
	})
}

func TestPrivatePlayerProfileDescriptorsPreserveFieldNumbersAndHackerPresence(t *testing.T) {
	t.Parallel()

	assertFields := func(message proto.Message, expected map[protoreflect.Name]protoreflect.FieldNumber) {
		t.Helper()
		descriptor := message.ProtoReflect().Descriptor()
		require.Equal(t, len(expected), descriptor.Fields().Len())
		for name, number := range expected {
			field := descriptor.Fields().ByName(name)
			require.NotNil(t, field, "%s must expose %s", descriptor.FullName(), name)
			require.Equal(t, number, field.Number(), "%s.%s field number drifted", descriptor.FullName(), name)
		}
	}

	assertFields(&privatev1.CharacterState{}, map[protoreflect.Name]protoreflect.FieldNumber{
		"character_id": 1, "display_name": 2, "logical_session_id": 3,
		"intelligence": 4, "hacker_perk_available": 5,
	})
	assertFields(&privatev1.AddCharacterRequest{}, map[protoreflect.Name]protoreflect.FieldNumber{
		"display_name": 1, "intelligence": 2, "hacker_perk_available": 3, "expected_revision": 4,
	})
	assertFields(&privatev1.RenameCharacterRequest{}, map[protoreflect.Name]protoreflect.FieldNumber{
		"character_id": 1, "display_name": 2, "intelligence": 3,
		"hacker_perk_available": 4, "expected_revision": 5,
	})
	assertFields(&privatev1.DeleteCharacterRequest{}, map[protoreflect.Name]protoreflect.FieldNumber{
		"character_id": 1, "expected_revision": 2,
	})

	hackerUnavailable := false
	add := &privatev1.AddCharacterRequest{
		DisplayName: "Mara", Intelligence: 8,
		HackerPerkAvailable: &hackerUnavailable, ExpectedRevision: 42,
	}
	cloned := proto.Clone(add).(*privatev1.AddCharacterRequest)
	require.NotNil(t, cloned.HackerPerkAvailable, "explicit false must retain presence")
	require.False(t, cloned.GetHackerPerkAvailable())
	require.Equal(t, int32(8), cloned.GetIntelligence())
	require.Equal(t, uint64(42), cloned.GetExpectedRevision())
}

func TestAddCharacterRequestAdapterPreservesCompletePayloadAndExplicitFalsePresence(t *testing.T) {
	t.Parallel()

	hackerUnavailable := false
	input := CharacterCreatePayload{
		Name:                "  Mara  ",
		Intelligence:        8,
		HackerPerkAvailable: &hackerUnavailable,
		ExpectedRevision:    42,
	}
	routed := routeAddCharacterRequest(input)
	require.Equal(t, input.Name, routed.Name)
	require.Equal(t, input.Intelligence, routed.Intelligence)
	require.Equal(t, input.ExpectedRevision, routed.ExpectedRevision)
	require.NotNil(t, routed.HackerPerkAvailable, "explicit false must retain protobuf presence")
	require.False(t, *routed.HackerPerkAvailable)
	require.NotSame(t, input.HackerPerkAvailable, routed.HackerPerkAvailable, "adapter result must detach optional scalar storage")

	missing := routeAddCharacterRequest(CharacterCreatePayload{
		Name: "Boone", Intelligence: 4, ExpectedRevision: 42,
	})
	require.Nil(t, missing.HackerPerkAvailable, "omission must remain distinguishable from explicit false")
}

func TestUpdateAndDeleteCharacterRequestAdaptersPreserveCompletePayloads(t *testing.T) {
	t.Parallel()

	hackerUnavailable := false
	updateInput := CharacterUpdatePayload{
		CharacterID:         "character-1",
		Name:                "  Mara Voss  ",
		Intelligence:        10,
		HackerPerkAvailable: &hackerUnavailable,
		ExpectedRevision:    42,
	}
	updated := routeUpdateCharacterRequest(updateInput)
	require.Equal(t, updateInput.CharacterID, updated.CharacterID)
	require.Equal(t, updateInput.Name, updated.Name)
	require.Equal(t, updateInput.Intelligence, updated.Intelligence)
	require.Equal(t, updateInput.ExpectedRevision, updated.ExpectedRevision)
	require.NotNil(t, updated.HackerPerkAvailable, "explicit false must retain protobuf presence")
	require.False(t, *updated.HackerPerkAvailable)
	require.NotSame(t, updateInput.HackerPerkAvailable, updated.HackerPerkAvailable)

	missing := routeUpdateCharacterRequest(CharacterUpdatePayload{
		CharacterID: "character-2", Name: "Boone", Intelligence: 4, ExpectedRevision: 42,
	})
	require.Nil(t, missing.HackerPerkAvailable, "omitted Hacker availability must remain distinguishable from explicit false")

	deleteInput := CharacterDeletePayload{CharacterID: "character-2", ExpectedRevision: 43}
	require.Equal(t, deleteInput, routeDeleteCharacterRequest(deleteInput))
}

func TestPrivatePlayerProfileProjectionPreservesValuesAndDetachesBothDirections(t *testing.T) {
	t.Parallel()

	controller := domain.LogicalSessionID("session-1")
	state := &domain.MasterCoordinationState{Revision: 17, Roster: []domain.MasterRosterEntry{
		{ID: "character-mara", Name: "Mara", ClaimedBySessionID: &controller},
		{ID: "character-boone", Name: "Boone"},
	}}
	setMasterRosterProfile(t, &state.Roster[0], 8, true)
	setMasterRosterProfile(t, &state.Roster[1], 4, false)

	semantic := coordinationStateToPrivate(state)
	require.Len(t, semantic.GetRoster(), 2)
	require.Equal(t, int32(8), semantic.GetRoster()[0].GetIntelligence())
	require.True(t, semantic.GetRoster()[0].GetHackerPerkAvailable())
	require.Equal(t, int32(4), semantic.GetRoster()[1].GetIntelligence())
	require.False(t, semantic.GetRoster()[1].GetHackerPerkAvailable(), "canonical false must survive private projection")

	state.Roster[0].Name = "mutated source"
	setMasterRosterProfile(t, &state.Roster[0], 1, false)
	require.Equal(t, "Mara", semantic.GetRoster()[0].GetDisplayName())
	require.Equal(t, int32(8), semantic.GetRoster()[0].GetIntelligence())
	require.True(t, semantic.GetRoster()[0].GetHackerPerkAvailable())

	routed := coordinationStateFromPrivate(semantic)
	require.Len(t, routed.Roster, 2)
	require.Equal(t, 8, masterRosterIntelligence(t, routed.Roster[0]))
	require.True(t, masterRosterHackerAvailable(t, routed.Roster[0]))
	require.Equal(t, 4, masterRosterIntelligence(t, routed.Roster[1]))
	require.False(t, masterRosterHackerAvailable(t, routed.Roster[1]))

	semantic.Roster[0].Intelligence = 2
	semantic.Roster[0].HackerPerkAvailable = false
	require.Equal(t, 8, masterRosterIntelligence(t, routed.Roster[0]))
	require.True(t, masterRosterHackerAvailable(t, routed.Roster[0]))
}

func TestPublicPlayerDescriptorsExcludePrivatePlayerProfileCapabilities(t *testing.T) {
	t.Parallel()

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(file.Path(), "fallout/terminal/player/v1/") {
			return true
		}
		for messageIndex := range file.Messages().Len() {
			message := file.Messages().Get(messageIndex)
			for fieldIndex := range message.Fields().Len() {
				name := strings.ToLower(string(message.Fields().Get(fieldIndex).Name()))
				for _, forbidden := range []string{"intelligence", "hacker", "player_config", "digest"} {
					require.NotContains(t, name, forbidden, "%s must remain player-safe", message.FullName())
				}
			}
		}
		return true
	})

	for _, fullName := range []protoreflect.FullName{
		"fallout.terminal.player.v1.AddCharacterRequest",
		"fallout.terminal.player.v1.RenameCharacterRequest",
		"fallout.terminal.player.v1.DeleteCharacterRequest",
	} {
		_, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
		require.Error(t, err, "%s must remain private", fullName)
	}
}

func setMasterRosterProfile(t *testing.T, entry *domain.MasterRosterEntry, intelligence int, hackerAvailable bool) {
	t.Helper()
	value := reflect.ValueOf(entry).Elem()
	intelligenceField := value.FieldByName("Intelligence")
	require.True(t, intelligenceField.IsValid(), "MasterRosterEntry must expose Intelligence")
	require.True(t, intelligenceField.CanSet())
	require.Equal(t, reflect.Int, intelligenceField.Kind())
	intelligenceField.SetInt(int64(intelligence))
	hackerField := value.FieldByName("HackerPerkAvailable")
	require.True(t, hackerField.IsValid(), "MasterRosterEntry must expose HackerPerkAvailable")
	require.True(t, hackerField.CanSet())
	require.Equal(t, reflect.Bool, hackerField.Kind())
	hackerField.SetBool(hackerAvailable)
}

func masterRosterIntelligence(t *testing.T, entry domain.MasterRosterEntry) int {
	t.Helper()
	field := reflect.ValueOf(entry).FieldByName("Intelligence")
	require.True(t, field.IsValid(), "MasterRosterEntry must expose Intelligence")
	require.Equal(t, reflect.Int, field.Kind())
	return int(field.Int())
}

func masterRosterHackerAvailable(t *testing.T, entry domain.MasterRosterEntry) bool {
	t.Helper()
	field := reflect.ValueOf(entry).FieldByName("HackerPerkAvailable")
	require.True(t, field.IsValid(), "MasterRosterEntry must expose HackerPerkAvailable")
	require.Equal(t, reflect.Bool, field.Kind())
	return field.Bool()
}

func TestPublicPlayerDescriptorsExcludePrivateTerminalNavigationCapabilities(t *testing.T) {
	t.Parallel()
	for _, fullName := range []protoreflect.FullName{
		"fallout.terminal.player.v1.PendingTerminalNavigation",
		"fallout.terminal.player.v1.TerminalNavigationDecision",
		"fallout.terminal.player.v1.ResolveTerminalNavigationRequest",
		"fallout.terminal.player.v1.TerminalNavigationNotice",
	} {
		_, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
		require.Error(t, err, "%s must remain private", fullName)
	}
}

func descriptorEnumNames(descriptor protoreflect.EnumDescriptor) []string {
	result := make([]string, 0, descriptor.Values().Len())
	for index := range descriptor.Values().Len() {
		value := descriptor.Values().Get(index)
		if value.Number() != protoreflect.EnumNumber(index) {
			return nil
		}
		result = append(result, string(value.Name()))
	}
	return result
}

func TestCommandStateResetPrivateAdaptersPreserveStableIDsDocumentAndRevision(t *testing.T) {
	t.Parallel()
	decision := CommandExecutionDecisionPayload{RequestID: "request-stable-1", Decision: domain.CommandExecutionApprove}
	routedDecision, err := routeCommandExecutionDecisionRequest(decision)
	require.NoError(t, err)
	require.Equal(t, decision, routedDecision)

	one := ResetCommandStatePayload{TerminalID: "terminal-stable-1", CommandID: "command-stable-1"}
	require.Equal(t, one, routeResetCommandStateRequest(one))

	all := ResetTerminalCommandStatesPayload{TerminalID: "terminal-stable-1"}
	require.Equal(t, all, routeResetTerminalCommandStatesRequest(all))

	session := commandStateResetSessionFixture()
	result := SessionStateResult{OK: true, Revision: 41, Session: &session}
	require.Equal(t, result, routeSessionStateResult(result))

	event := SessionStateEvent{Revision: 41, Session: &session}
	require.Equal(t, event, routeSessionStateEvent(event))
}

func TestRuntimeStatusDescriptorRemainsFeature005Compatible(t *testing.T) {
	t.Parallel()

	descriptor := (&privatev1.RuntimeStatus{}).ProtoReflect().Descriptor()
	require.Equal(t, "fallout.terminal.private.v1.RuntimeStatus", string(descriptor.FullName()))

	wantFields := []string{
		"server_info", "client_count", "hack_state", "startup_error",
		"save_state", "requested_revision", "saved_revision", "coordination_state",
	}
	gotFields := make([]string, 0, descriptor.Fields().Len())
	for index := range descriptor.Fields().Len() {
		field := descriptor.Fields().Get(index)
		gotFields = append(gotFields, string(field.Name()))
		require.Equal(t, protoreflect.FieldNumber(index+1), field.Number())
	}
	require.Equal(t, wantFields, gotFields)
	require.Nil(t, descriptor.Fields().ByName("phase"))
	require.Nil(t, descriptor.Fields().ByJSONName("phase"))
	require.Zero(t, descriptor.ParentFile().Enums().Len())
}

func TestPrivateStatusResultAndEventAdaptersRoundTripEveryNativeSemantic(t *testing.T) {
	controller := domain.LogicalSessionID("session-1")
	terminal := "terminal-1"
	target := "terminal-2"
	state := &domain.MasterCoordinationState{
		Revision:      17,
		PlayerConfig:  &domain.PlayerConfigMetadata{Status: "loaded", FilePath: "/private/players.json", Version: 1, Name: "Vault 33"},
		Roster:        []domain.MasterRosterEntry{{ID: "character-1", Name: "Lucy", ClaimedBySessionID: &controller}},
		Sessions:      []domain.MasterSessionEntry{{ID: controller, FallbackName: "PLAYER 1", Connected: true, Character: &domain.PlayerCharacter{ID: "character-1", Name: "Lucy"}, Role: domain.PlayerRoleActive}},
		Broadcast:     &domain.MasterBroadcastState{ID: "broadcast-1", ControllerSessionID: &controller, ActiveTerminalID: &terminal},
		PendingSwitch: &domain.MasterPendingSwitch{SwitchID: "switch-1", BroadcastID: "broadcast-1", SourceTerminalID: terminal, TargetTerminalID: &target},
		PendingCommandExecution: &domain.MasterPendingCommandExecution{
			RequestID: "request-1", BroadcastID: "broadcast-1", TerminalID: terminal,
			CommandID: "command-1", CommandName: "Open doors", Mode: domain.CommandApprovalModeStateChange,
			ConfirmationText: "Open the doors?",
		},
	}
	status := RuntimeStatus{
		ServerInfo:  &domain.ServerInfo{URL: "https://fallout.example", LocalURL: "http://127.0.0.1:3690", Tunnel: true},
		ClientCount: 2, StartupError: "startup", SaveState: "saved", RequestedRevision: 19, SavedRevision: 18,
		CoordinationState: state,
	}
	routed := routeRuntimeStatus(status)
	require.Equal(t, status, routed)
	require.Equal(t, state, routeCoordinationEvent(state))
	require.Equal(t, domain.ServerInfo{URL: "https://fallout.example", LocalURL: "http://127.0.0.1:3690", Tunnel: true}, routeServerInfoEvent(*status.ServerInfo))
	require.Equal(t, 2, routeClientCountEvent(2))
	require.Equal(t, CommandResult{OK: true}, routeCommandResult(CommandResult{OK: true}))
	require.Equal(t, CoordinationCommandResult{OK: true, State: state}, routeCoordinationResult(CoordinationCommandResult{OK: true, State: state}))
	require.Equal(t, ResolveCommandExecutionResult{OK: true, State: state}, routeResolveCommandExecutionResult(ResolveCommandExecutionResult{OK: true, State: state}))
}

func TestDesktopServiceInventoryAndNativeEventsAreExactlyAllowlisted(t *testing.T) {
	requiredMethods := []string{
		"GetRuntimeStatus", "NewSession", "OpenSession", "CopyDemo", "SaveSession", "ReplaceTerminalGroups", "LoadReferencedPlayerConfig", "NewPlayerConfig", "OpenPlayerConfig",
		"RequestTerminalActivation", "UpdateLiveTerminal", "RequestTerminalClear", "ResolveTerminalSwitch", "ResolveCommandExecution", "ResolveTerminalNavigation", "ForceHackSuccess", "ResetFailedHack", "ResetCommandState", "ResetTerminalCommandStates",
		"AddCharacter", "UpdateCharacter", "DeleteCharacter", "RenameLogicalSession", "AssignCharacter", "ReleaseCharacter", "MoveCharacter", "SetActiveController",
		"StartBroadcast", "EndBroadcast", "OpenURL", "GetPublicAccess", "SavePublicAccessSettings", "GeneratePlayerPassword", "StartPublicAccess", "StopPublicAccess",
	}
	serviceType := reflect.TypeFor[*desktopService]()
	actualMethods := make([]string, 0, serviceType.NumMethod())
	for method := range serviceType.Methods() {
		actualMethods = append(actualMethods, method.Name)
	}
	require.Len(t, actualMethods, 35)
	require.ElementsMatch(t, requiredMethods, actualMethods)

	for _, forbidden := range []string{
		"Start", "Shutdown", "ServiceStartup", "ServiceShutdown",
		"Dispatch", "Call", "Capabilities", "Reflect",
		"ReadFile", "WriteFile", "Exec", "Environment", "OpenDialog", "Browser",
		"PlayerService", "Subscribe", "SelectCharacter", "Navigate", "Guess", "ActivatePattern", "SoundManifest",
	} {
		require.NotContains(t, actualMethods, forbidden)
	}

	require.Equal(t, []string{"server-info", "client-count", "hack-state", "coordination-state", "session-state", "public-access-status"}, []string{serverInfoEvent, clientCountEvent, hackStateEvent, coordinationStateEvent, sessionStateEvent, publicAccessStatusEvent})
}

func TestDesktopServiceMethodsAreTransparentCoreForwards(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "desktop_service.go", nil, 0)
	require.NoError(t, err)

	forwarded := make(map[string]string)
	for _, declaration := range file.Decls {
		method, ok := declaration.(*ast.FuncDecl)
		if !ok || method.Recv == nil || method.Name.Name == "newDesktopService" {
			continue
		}
		require.Len(t, method.Body.List, 1, "%s must remain a transparent forward", method.Name.Name)
		returned, ok := method.Body.List[0].(*ast.ReturnStmt)
		require.True(t, ok, "%s must return the core result directly", method.Name.Name)
		require.Len(t, returned.Results, 1, "%s must return exactly one core call", method.Name.Name)
		call, ok := returned.Results[0].(*ast.CallExpr)
		require.True(t, ok, "%s must return a core call", method.Name.Name)
		selector, ok := call.Fun.(*ast.SelectorExpr)
		require.True(t, ok, "%s must call an explicit core method", method.Name.Name)
		core, ok := selector.X.(*ast.SelectorExpr)
		require.True(t, ok, "%s must call through service.core", method.Name.Name)
		service, ok := core.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "service", service.Name)
		require.Equal(t, "core", core.Sel.Name)
		forwarded[method.Name.Name] = selector.Sel.Name
	}

	require.Len(t, forwarded, 35)
	for exposed, core := range forwarded {
		require.Equal(t, exposed, core, "%s must not translate into an authored capability", exposed)
	}
}

func TestDetachedDesktopResultShapesPreserveCancellationErrorsAndStatusFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		keys  []string
	}{
		{"runtime status", RuntimeStatus{}, []string{"serverInfo", "clientCount", "hackState", "saveState", "requestedRevision", "savedRevision", "coordinationState"}},
		{"command", CommandResult{Error: "safe"}, []string{"ok", "error"}},
		{"session cancellation", sessionservice.SessionResult{Canceled: true}, []string{"ok", "canceled"}},
		{"save", sessionservice.SaveResult{Error: "safe"}, []string{"ok", "error", "requestedRevision"}},
		{"player config cancellation", PlayerConfigCommandResult{Canceled: true}, []string{"ok", "canceled", "state"}},
		{"coordination", CoordinationCommandResult{Error: "safe"}, []string{"ok", "error", "state"}},
		{"command execution", ResolveCommandExecutionResult{Error: "safe"}, []string{"ok", "error", "state"}},
		{"terminal switch", TerminalSwitchCommandResult{Error: "safe"}, []string{"ok", "error", "state"}},
		{"session state", SessionStateResult{Error: "safe", Revision: 7}, []string{"ok", "error", "revision"}},
		{"terminal group replacement", TerminalGroupReplacementResult{Error: "safe", SessionRevision: 7}, []string{"ok", "error", "sessionRevision", "coordinationState"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(test.value)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &fields))
			actual := make([]string, 0, len(fields))
			for key := range fields {
				actual = append(actual, key)
			}
			require.ElementsMatch(t, test.keys, actual)
			require.NotContains(t, fields, "phase")
		})
	}
}

func TestTerminalGroupReplacementRequestPreservesLegacyRepairCandidate(t *testing.T) {
	t.Parallel()

	payload := TerminalGroupReplacementPayload{
		TerminalGroups: []domain.TerminalGroup{{
			ID: "singleton-source", Name: "Source", TerminalIDs: []string{"terminal-a", "terminal-b"},
		}},
		ExpectedSessionRevision: 7, ExpectedCoordinationRevision: 11,
	}

	routed := routeTerminalGroupReplacementRequest(payload)

	require.Equal(t, payload, routed)
	routed.TerminalGroups[0].TerminalIDs[0] = "mutated"
	require.Equal(t, "terminal-a", payload.TerminalGroups[0].TerminalIDs[0])
}

func TestTerminalGroupReplacementRequestPreservesMultiLinkLegacyCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		groups []domain.TerminalGroup
	}{
		{
			name: "partial",
			groups: []domain.TerminalGroup{
				{ID: "singleton-service", Name: "Service", TerminalIDs: []string{"t-krel-service", "t-krel-admin"}},
				{ID: "singleton-emergency", Name: "Emergency", TerminalIDs: []string{"t-krel-emergency"}},
			},
		},
		{
			name: "complete",
			groups: []domain.TerminalGroup{{
				ID: "singleton-service", Name: "Service",
				TerminalIDs: []string{"t-krel-service", "t-krel-admin", "t-krel-emergency"},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			payload := TerminalGroupReplacementPayload{
				TerminalGroups: test.groups, ExpectedSessionRevision: 7, ExpectedCoordinationRevision: 11,
			}

			routed := routeTerminalGroupReplacementRequest(payload)

			require.Equal(t, payload, routed)
			routed.TerminalGroups[0].TerminalIDs[0] = "mutated"
			require.Equal(t, "t-krel-service", payload.TerminalGroups[0].TerminalIDs[0])
		})
	}
}
