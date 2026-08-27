package player

import (
	"reflect"
	"strings"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestGeneratedMutationFingerprintsAreDeterministicProcedureQualifiedAndUnknownAware(t *testing.T) {
	navigate := &playerv1.NavigateRequest{
		RecognitionHandle: "recognition-1", RequestId: "request-1", BroadcastId: "broadcast-1", TerminalId: "terminal-1",
		Action: &playerv1.NavigateRequest_Enter{Enter: &playerv1.NavigateEnter{NodeId: "docs"}},
	}
	first, err := NavigateFromProto(navigate)
	require.NoError(t, err)
	second, err := NavigateFromProto(proto.Clone(navigate).(*playerv1.NavigateRequest))
	require.NoError(t, err)
	require.Equal(t, first.Command.PayloadFingerprint, second.Command.PayloadFingerprint)
	require.Len(t, first.Command.PayloadFingerprint, 64)
	require.NotContains(t, first.Command.PayloadFingerprint, "recognition-1")

	changed := proto.Clone(navigate).(*playerv1.NavigateRequest)
	changed.Action = &playerv1.NavigateRequest_Entry{Entry: &playerv1.NavigateEntry{NodeId: "docs"}}
	changedMutation, err := NavigateFromProto(changed)
	require.NoError(t, err)
	require.NotEqual(t, first.Command.PayloadFingerprint, changedMutation.Command.PayloadFingerprint)

	withUnknown := proto.Clone(navigate).(*playerv1.NavigateRequest)
	withUnknown.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	unknownMutation, err := NavigateFromProto(withUnknown)
	require.NoError(t, err)
	require.NotEqual(t, first.Command.PayloadFingerprint, unknownMutation.Command.PayloadFingerprint)

	guess, err := GuessFromProto(&playerv1.GuessRequest{
		RecognitionHandle: "recognition-1", RequestId: "request-1", BroadcastId: "broadcast-1", TerminalId: "terminal-1",
		Target: &playerv1.GuessRequest_WordId{WordId: "docs"},
	})
	require.NoError(t, err)
	require.NotEqual(t, first.Command.PayloadFingerprint, guess.Command.PayloadFingerprint)
	require.False(t, strings.Contains(guess.Command.PayloadFingerprint, "Guess"))
}

func TestGeneratedControllerPresentationContractIsTypedAndProjected(t *testing.T) {
	service := playerv1.File_fallout_terminal_player_v1_player_proto.Services().ByName("PlayerService")
	require.NotNil(t, service)
	method := service.Methods().ByName("SetPresentation")
	require.NotNil(t, method, "PlayerService must expose the typed presentation mutation")
	request := method.Input()
	for _, fieldName := range []protoreflect.Name{
		"recognition_handle", "request_id", "broadcast_id", "terminal_id", "context_key", "presentation",
	} {
		require.NotNilf(t, request.Fields().ByName(fieldName), "SetPresentationRequest.%s is required", fieldName)
	}

	live := playerv1.File_fallout_terminal_player_v1_terminal_proto.Messages().ByName("LiveTerminal")
	require.NotNil(t, live)
	require.NotNil(t, live.Fields().ByName("controller_presentation"),
		"complete snapshots and updates must carry controller presentation")
}

func TestPresentationAdapterValidatesContextAndDetachesExclusiveVariant(t *testing.T) {
	request := &playerv1.SetPresentationRequest{
		RecognitionHandle: "recognition-1", RequestId: "presentation-1",
		BroadcastId: "broadcast-1", TerminalId: "terminal-1", ContextKey: "menu:root",
		Presentation: &playerv1.ControllerTerminalPresentation{
			ContextKey:   "menu:root",
			Presentation: &playerv1.ControllerTerminalPresentation_Menu{Menu: &playerv1.MenuSelection{TargetId: "docs"}},
		},
	}
	mutation, err := PresentationFromProto(request)
	require.NoError(t, err)
	require.Equal(t, domain.RuntimeCommandPresentation, mutation.Command.Kind)
	require.Equal(t, domain.ControllerTerminalPresentation{
		Kind: domain.ControllerTerminalPresentationMenu, ContextKey: "menu:root", TargetID: "docs",
	}, mutation.Command.Presentation)
	require.Len(t, mutation.Command.PayloadFingerprint, 64)

	request.Presentation.ContextKey = "menu:stale"
	_, err = PresentationFromProto(request)
	require.Error(t, err)
}

func TestPresentationUplinkContractAndAdapters(t *testing.T) {
	service := playerv1.File_fallout_terminal_player_v1_player_proto.Services().ByName("PlayerService")
	require.NotNil(t, service)
	method := service.Methods().ByName("PresentationUplink")
	require.NotNil(t, method)
	require.True(t, method.IsStreamingClient())
	require.False(t, method.IsStreamingServer())

	binding, err := PresentationUplinkOpenFromProto(&playerv1.PresentationUplinkOpen{
		ClientInstanceId: "tab-1", UplinkGeneration: 2, RecognitionHandle: "recognition-1",
	})
	require.NoError(t, err)
	require.Equal(t, "tab-1", binding.ClientInstanceID)
	require.Equal(t, uint64(2), binding.Generation)
	require.Equal(t, domain.RecognitionHandle("recognition-1"), binding.RecognitionHandle)

	intent := &playerv1.PresentationIntent{
		RecognitionHandle: "recognition-1", RequestId: "presentation-stream-1",
		BroadcastId: "broadcast-1", TerminalId: "terminal-1", ContextKey: "menu:root",
		Presentation: &playerv1.ControllerTerminalPresentation{
			ContextKey:   "menu:root",
			Presentation: &playerv1.ControllerTerminalPresentation_Menu{Menu: &playerv1.MenuSelection{TargetId: "docs"}},
		},
	}
	mutation, err := PresentationIntentFromProto(intent)
	require.NoError(t, err)
	require.Equal(t, domain.RuntimeCommandPresentation, mutation.Command.Kind)
	require.Equal(t, domain.RequestID("presentation-stream-1"), mutation.Command.RequestID)
	require.Len(t, mutation.Command.PayloadFingerprint, 64)

	intent.Presentation.ContextKey = "menu:stale"
	_, err = PresentationIntentFromProto(intent)
	require.Error(t, err)
	_, err = PresentationUplinkOpenFromProto(&playerv1.PresentationUplinkOpen{})
	require.Error(t, err)
}

func TestLiveToProtoCarriesCompleteControllerPresentation(t *testing.T) {
	state := &domain.PublicLiveState{
		TerminalID: "terminal-1", TerminalName: "Overseer",
		Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT"},
		Nav:  domain.NavState{Path: []string{"root"}, Mode: "list"},
		Presentation: domain.ControllerTerminalPresentation{
			Kind: domain.ControllerTerminalPresentationHacking, ContextKey: "hack:opaque", PatternID: "pattern-1",
		},
	}
	generated := LiveToProto(state).GetControllerPresentation()
	require.NotNil(t, generated)
	require.Equal(t, "hack:opaque", generated.GetContextKey())
	require.Equal(t, "pattern-1", generated.GetHacking().GetPatternId())
}

func TestLiveToProtoMapsPendingAndRejectedCommandExecutionPresentation(t *testing.T) {
	tests := []struct {
		name      string
		domain    string
		generated playerv1.CommandExecutionPhase
	}{
		{name: "pending", domain: "pending", generated: playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_PENDING},
		{name: "rejected", domain: "rejected", generated: playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_REJECTED},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &domain.PublicLiveState{
				TerminalID: "terminal-1", TerminalName: "Overseer",
				Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT"},
				Nav:  domain.NavState{Path: []string{"root"}, Mode: "list"},
			}
			setDomainCommandExecution(t, state, test.domain, "doors")

			got := LiveToProto(state)
			require.NotNil(t, got)
			require.NotNil(t, got.GetCommandExecution())
			require.Equal(t, test.generated, got.GetCommandExecution().GetPhase())
			require.Equal(t, "doors", got.GetCommandExecution().GetCommandNodeId())
		})
	}
}

func TestPlayerStateToProtoMapsSafePersistenceNoticeAndClearsItWithoutStickyDetails(t *testing.T) {
	state := &domain.PlayerState{
		SessionID: "controller-session", FallbackName: "Игрок 1",
		Role: domain.PlayerRoleActive, Phase: domain.PlayerPhaseControlling,
	}
	setDomainPlayerNotice(t, state, "command-persistence-failed")

	failed := PlayerStateToProto(state)
	require.NotNil(t, failed.GetNotice())
	require.Equal(t,
		playerv1.PlayerNoticeKind_PLAYER_NOTICE_KIND_COMMAND_PERSISTENCE_FAILED,
		failed.GetNotice().GetKind(),
	)
	require.Equal(t, 1, failed.GetNotice().ProtoReflect().Descriptor().Fields().Len(),
		"the player notice must remain an enum-only safe projection")

	clearDomainOptionalField(t, state, "Notice")
	cleared := PlayerStateToProto(state)
	require.Nil(t, cleared.GetNotice(), "a later authoritative state without a notice must clear it")
}

func TestPlayerStateProjectionExcludesMasterOnlyPlayerProfileAttributes(t *testing.T) {
	t.Parallel()

	state := &domain.PlayerState{
		SessionID: "session-1", FallbackName: "PLAYER 1",
		Character: &domain.PlayerCharacter{ID: "character-mara", Name: "Mara"},
		Roster: []domain.PlayerRosterEntry{
			{ID: "character-mara", Name: "Mara", Status: domain.RosterStatusClaimed},
			{ID: "character-boone", Name: "Boone", Status: domain.RosterStatusAvailable},
		},
	}

	got := PlayerStateToProto(state)
	require.NotNil(t, got)
	require.Equal(t, "Mara", got.GetAssignedCharacter().GetDisplayName())
	require.Equal(t, "Boone", got.GetRoster()[1].GetDisplayName())

	assertExactPlayerSafeFields := func(message proto.Message, expected []string) {
		t.Helper()
		fields := message.ProtoReflect().Descriptor().Fields()
		actual := make([]string, 0, fields.Len())
		for index := range fields.Len() {
			name := string(fields.Get(index).Name())
			actual = append(actual, name)
			lowerName := strings.ToLower(name)
			require.NotContains(t, lowerName, "intelligence")
			require.NotContains(t, lowerName, "hacker")
			require.NotContains(t, lowerName, "digest")
		}
		require.Equal(t, expected, actual)
	}
	assertExactPlayerSafeFields(got.GetAssignedCharacter(), []string{"character_id", "display_name"})
	assertExactPlayerSafeFields(got.GetRoster()[0], []string{"character_id", "display_name", "availability"})

	state.Character.Name = "mutated source"
	state.Roster[0].Name = "mutated source"
	require.Equal(t, "Mara", got.GetAssignedCharacter().GetDisplayName())
	require.Equal(t, "Mara", got.GetRoster()[0].GetDisplayName(), "public projection must remain detached")
}

func TestOrdinaryCommandProjectionRemainsUnchangedAndHasNoExecutionPresentation(t *testing.T) {
	state := &domain.PublicLiveState{
		TerminalID: "terminal-1", TerminalName: "Overseer",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: "diagnostic", Type: domain.NodeCommand,
				Name: "RUN DIAGNOSTIC", Text: "SYSTEM NOMINAL",
			}},
		},
		Nav: domain.NavState{Path: []string{"root"}, Mode: "list"},
	}

	got := LiveToProto(state)
	require.Nil(t, got.GetCommandExecution())
	require.Equal(t, "RUN DIAGNOSTIC", got.GetTree().GetFolder().GetChildren()[0].GetName())
	require.Equal(t, "SYSTEM NOMINAL", got.GetTree().GetFolder().GetChildren()[0].GetCommand().GetText())
}

func TestTerminalNavigationProjectionContainsOnlyPlayerSafeRouteMetadata(t *testing.T) {
	state := &domain.PublicLiveState{
		TerminalID: "terminal-b", TerminalName: "Terminal B",
		Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT"},
		Nav:  domain.NavState{Path: []string{"root"}, Mode: "list"},
		TerminalNavigation: &domain.TerminalNavigationPresentation{
			RouteDepth:   2,
			ReturnTarget: &domain.TerminalReturnTarget{TerminalID: "terminal-a", TerminalName: "Terminal A"},
			Pending: &domain.PendingTerminalNavigationPresentation{
				Direction: domain.TerminalNavigationReturn, TargetTerminalID: "terminal-a", TargetTerminalName: "Terminal A",
			},
		},
	}
	got := LiveToProto(state).GetTerminalNavigation()
	require.NotNil(t, got)
	require.Equal(t, uint32(2), got.GetRouteDepth())
	require.Equal(t, "terminal-a", got.GetReturnTarget().GetTerminalId())
	require.Equal(t, playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_RETURN, got.GetPending().GetDirection())

	fields := got.ProtoReflect().Descriptor().Fields()
	names := make([]string, 0, fields.Len())
	for index := range fields.Len() {
		names = append(names, string(fields.Get(index).Name()))
	}
	require.Equal(t, []string{"route_depth", "return_target", "pending"}, names)
	for _, forbidden := range []protoreflect.Name{"request_id", "controller_session_id", "command_id", "return_point", "notice"} {
		require.Nil(t, fields.ByName(forbidden))
	}
	encoded, err := proto.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-request-identity")
}

// The runtime projection types are introduced by the implementation wave
// after these RED tests. Reflection keeps the package buildable while still
// pinning their required field names, pointer presence, and string enum values.
func setDomainCommandExecution(t *testing.T, state *domain.PublicLiveState, phase, commandNodeID string) {
	t.Helper()
	field := requireDomainOptionalStructField(t, state, "CommandExecution")
	presentation := reflect.New(field.Type().Elem())
	setDomainStringField(t, presentation.Elem(), "Phase", phase)
	setDomainStringField(t, presentation.Elem(), "CommandID", commandNodeID)
	field.Set(presentation)
}

func setDomainPlayerNotice(t *testing.T, state *domain.PlayerState, kind string) {
	t.Helper()
	field := requireDomainOptionalStructField(t, state, "Notice")
	notice := reflect.New(field.Type().Elem())
	setDomainStringField(t, notice.Elem(), "Kind", kind)
	field.Set(notice)
}

func clearDomainOptionalField(t *testing.T, target any, fieldName string) {
	t.Helper()
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	require.Truef(t, field.IsValid(), "domain %T must expose %s", target, fieldName)
	require.Equal(t, reflect.Pointer, field.Kind())
	field.SetZero()
}

func requireDomainOptionalStructField(t *testing.T, target any, fieldName string) reflect.Value {
	t.Helper()
	value := reflect.ValueOf(target)
	require.Equal(t, reflect.Pointer, value.Kind())
	field := value.Elem().FieldByName(fieldName)
	require.Truef(t, field.IsValid(), "domain %T must expose %s", target, fieldName)
	require.Truef(t, field.CanSet(), "domain %T field %s must be settable", target, fieldName)
	require.Equal(t, reflect.Pointer, field.Kind())
	require.Equal(t, reflect.Struct, field.Type().Elem().Kind())
	return field
}

func setDomainStringField(t *testing.T, value reflect.Value, fieldName, content string) {
	t.Helper()
	field := value.FieldByName(fieldName)
	require.Truef(t, field.IsValid(), "%s must expose %s", value.Type(), fieldName)
	require.Truef(t, field.CanSet(), "%s.%s must be settable", value.Type(), fieldName)
	require.Equal(t, reflect.String, field.Kind())
	field.SetString(content)
}
