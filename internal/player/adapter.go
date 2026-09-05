package player

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/control"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	playerv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/player/v1/playerv1connect"
	"google.golang.org/protobuf/proto"
)

// SelectionMutation is a structurally validated recognition plus typed
// selection request. Logical-session lookup remains coordinator-owned.
type SelectionMutation struct {
	RecognitionHandle domain.RecognitionHandle
	Selection         control.CharacterSelection
}

// RuntimeMutation is a structurally validated recognition plus one typed
// shared-terminal command. Authority remains coordinator-owned.
type RuntimeMutation struct {
	RecognitionHandle domain.RecognitionHandle
	Command           domain.RuntimeCommand
}

// PresentationUplinkBinding is validated process-local routing metadata from
// the generated opening frame. It identifies a tab and generation but grants
// no mutation authority.
type PresentationUplinkBinding struct {
	ClientInstanceID  string
	Generation        uint64
	RecognitionHandle domain.RecognitionHandle
}

// PresentationUplinkOpenFromProto validates and detaches one opening frame.
func PresentationUplinkOpenFromProto(open *playerv1.PresentationUplinkOpen) (PresentationUplinkBinding, error) {
	if open == nil {
		return PresentationUplinkBinding{}, fmt.Errorf("presentation uplink open frame is required")
	}
	if err := ValidateMessageSize(open); err != nil {
		return PresentationUplinkBinding{}, err
	}
	if err := domain.ValidatePublicField(domain.PublicFieldGenerationID, open.GetClientInstanceId()); err != nil {
		return PresentationUplinkBinding{}, err
	}
	if err := domain.ValidatePublicField(domain.PublicFieldRecognitionHandle, open.GetRecognitionHandle()); err != nil {
		return PresentationUplinkBinding{}, err
	}
	if open.GetUplinkGeneration() == 0 {
		return PresentationUplinkBinding{}, fmt.Errorf("presentation uplink generation is required")
	}
	return PresentationUplinkBinding{
		ClientInstanceID: open.GetClientInstanceId(), Generation: open.GetUplinkGeneration(),
		RecognitionHandle: domain.RecognitionHandle(open.GetRecognitionHandle()),
	}, nil
}

// PresentationIntentFromProto validates one generated stream intent through
// the same canonical presentation adapter used by SetPresentation.
func PresentationIntentFromProto(intent *playerv1.PresentationIntent) (RuntimeMutation, error) {
	if intent == nil {
		return RuntimeMutation{}, fmt.Errorf("presentation intent is required")
	}
	if err := ValidateMessageSize(intent); err != nil {
		return RuntimeMutation{}, err
	}
	mutation, err := PresentationFromProto(&playerv1.SetPresentationRequest{
		RecognitionHandle: intent.GetRecognitionHandle(), RequestId: intent.GetRequestId(),
		BroadcastId: intent.GetBroadcastId(), TerminalId: intent.GetTerminalId(),
		ContextKey: intent.GetContextKey(), Presentation: intent.GetPresentation(),
	})
	if err != nil {
		return RuntimeMutation{}, err
	}
	fingerprint, err := deterministicRequestFingerprint(playerv1connect.PlayerServicePresentationUplinkProcedure, intent)
	if err != nil {
		return RuntimeMutation{}, err
	}
	mutation.Command.PayloadFingerprint = fingerprint
	return mutation, nil
}

// SubscribeRecognition distinguishes absent scalar presence from an invalid
// present value. Unknown but well-formed handles are resolved by the
// coordinator and may receive a replacement session.
func SubscribeRecognition(request *playerv1.SubscribeRequest) (*domain.RecognitionHandle, error) {
	if request == nil {
		return nil, fmt.Errorf("subscribe request is required")
	}
	if err := ValidateMessageSize(request); err != nil {
		return nil, err
	}
	if request.RecognitionHandle == nil {
		return nil, nil
	}
	if err := domain.ValidatePublicField(domain.PublicFieldRecognitionHandle, request.GetRecognitionHandle()); err != nil {
		return nil, err
	}
	handle := domain.RecognitionHandle(request.GetRecognitionHandle())
	return &handle, nil
}

// SubscribeClientInstance validates the optional ephemeral tab routing ID.
func SubscribeClientInstance(request *playerv1.SubscribeRequest) (string, error) {
	if request == nil || request.ClientInstanceId == nil {
		return "", nil
	}
	if err := domain.ValidatePublicField(domain.PublicFieldGenerationID, request.GetClientInstanceId()); err != nil {
		return "", err
	}
	return request.GetClientInstanceId(), nil
}

// SelectionFromProto validates and detaches one SelectCharacter request.
func SelectionFromProto(request *playerv1.SelectCharacterRequest, connectionID domain.ConnectionID) (SelectionMutation, error) {
	if request == nil {
		return SelectionMutation{}, fmt.Errorf("select character request is required")
	}
	if err := ValidateMessageSize(request); err != nil {
		return SelectionMutation{}, err
	}
	if err := validateCommonMutation(request.GetRecognitionHandle(), request.GetRequestId(), request.GetBroadcastId()); err != nil {
		return SelectionMutation{}, err
	}
	if err := domain.ValidatePublicField(domain.PublicFieldCharacterID, request.GetCharacterId()); err != nil {
		return SelectionMutation{}, err
	}
	fingerprint, err := deterministicRequestFingerprint(playerv1connect.PlayerServiceSelectCharacterProcedure, request)
	if err != nil {
		return SelectionMutation{}, err
	}
	return SelectionMutation{
		RecognitionHandle: domain.RecognitionHandle(request.GetRecognitionHandle()),
		Selection: control.CharacterSelection{
			ConnectionID: connectionID,
			RequestID:    domain.RequestID(request.GetRequestId()),
			BroadcastID:  domain.BroadcastID(request.GetBroadcastId()),
			CharacterID:  domain.CharacterID(request.GetCharacterId()),
			Fingerprint:  fingerprint,
		},
	}, nil
}

// NavigateFromProto validates and detaches one navigation variant.
func NavigateFromProto(request *playerv1.NavigateRequest) (RuntimeMutation, error) {
	if request == nil {
		return RuntimeMutation{}, fmt.Errorf("navigate request is required")
	}
	if err := ValidateMessageSize(request); err != nil {
		return RuntimeMutation{}, err
	}
	command := domain.RuntimeCommand{
		RequestID:   domain.RequestID(request.GetRequestId()),
		BroadcastID: domain.BroadcastID(request.GetBroadcastId()),
		TerminalID:  request.GetTerminalId(),
		Kind:        domain.RuntimeCommandNavigate,
	}
	if err := validateTerminalMutation(request.GetRecognitionHandle(), request.GetRequestId(), request.GetBroadcastId(), request.GetTerminalId()); err != nil {
		return RuntimeMutation{}, err
	}
	switch action := request.Action.(type) {
	case *playerv1.NavigateRequest_Back:
		command.Action = "back"
	case *playerv1.NavigateRequest_Enter:
		command.Action = "enter"
		command.NodeID = action.Enter.GetNodeId()
	case *playerv1.NavigateRequest_Command:
		command.Action = "command"
		command.NodeID = action.Command.GetNodeId()
	case *playerv1.NavigateRequest_Entry:
		command.Action = "entry"
		command.NodeID = action.Entry.GetNodeId()
	default:
		return RuntimeMutation{}, fmt.Errorf("navigate action is required")
	}
	if command.Action != "back" {
		if err := domain.ValidatePublicField(domain.PublicFieldActionTarget, command.NodeID); err != nil {
			return RuntimeMutation{}, err
		}
	}
	fingerprint, err := deterministicRequestFingerprint(playerv1connect.PlayerServiceNavigateProcedure, request)
	if err != nil {
		return RuntimeMutation{}, err
	}
	command.PayloadFingerprint = fingerprint
	return RuntimeMutation{RecognitionHandle: domain.RecognitionHandle(request.GetRecognitionHandle()), Command: command}, nil
}

// GuessFromProto validates and detaches a word or filler target.
func GuessFromProto(request *playerv1.GuessRequest) (RuntimeMutation, error) {
	if request == nil {
		return RuntimeMutation{}, fmt.Errorf("guess request is required")
	}
	if err := ValidateMessageSize(request); err != nil {
		return RuntimeMutation{}, err
	}
	if err := validateTerminalMutation(request.GetRecognitionHandle(), request.GetRequestId(), request.GetBroadcastId(), request.GetTerminalId()); err != nil {
		return RuntimeMutation{}, err
	}
	command := domain.RuntimeCommand{
		RequestID:   domain.RequestID(request.GetRequestId()),
		BroadcastID: domain.BroadcastID(request.GetBroadcastId()),
		TerminalID:  request.GetTerminalId(),
		Kind:        domain.RuntimeCommandGuess,
	}
	switch target := request.Target.(type) {
	case *playerv1.GuessRequest_WordId:
		command.TargetID = target.WordId
	case *playerv1.GuessRequest_Filler:
		if target.Filler == nil || target.Filler.GetColumn() < 0 || target.Filler.GetColumn() > 1 || target.Filler.GetCharacter() < 0 || target.Filler.GetCharacter() >= 192 {
			return RuntimeMutation{}, fmt.Errorf("filler target is outside the public board")
		}
		command.TargetID = fmt.Sprintf("%d:%d", target.Filler.GetColumn(), target.Filler.GetCharacter())
	default:
		return RuntimeMutation{}, fmt.Errorf("guess target is required")
	}
	if err := domain.ValidatePublicField(domain.PublicFieldActionTarget, command.TargetID); err != nil {
		return RuntimeMutation{}, err
	}
	fingerprint, err := deterministicRequestFingerprint(playerv1connect.PlayerServiceGuessProcedure, request)
	if err != nil {
		return RuntimeMutation{}, err
	}
	command.PayloadFingerprint = fingerprint
	return RuntimeMutation{RecognitionHandle: domain.RecognitionHandle(request.GetRecognitionHandle()), Command: command}, nil
}

// ActivatePatternFromProto validates and detaches an opaque pattern target.
func ActivatePatternFromProto(request *playerv1.ActivatePatternRequest) (RuntimeMutation, error) {
	if request == nil {
		return RuntimeMutation{}, fmt.Errorf("activate pattern request is required")
	}
	if err := ValidateMessageSize(request); err != nil {
		return RuntimeMutation{}, err
	}
	if err := validateTerminalMutation(request.GetRecognitionHandle(), request.GetRequestId(), request.GetBroadcastId(), request.GetTerminalId()); err != nil {
		return RuntimeMutation{}, err
	}
	if err := domain.ValidatePublicField(domain.PublicFieldActionTarget, request.GetPatternId()); err != nil {
		return RuntimeMutation{}, err
	}
	command := domain.RuntimeCommand{
		RequestID:   domain.RequestID(request.GetRequestId()),
		BroadcastID: domain.BroadcastID(request.GetBroadcastId()),
		TerminalID:  request.GetTerminalId(),
		Kind:        domain.RuntimeCommandActivatePattern,
		PatternID:   request.GetPatternId(),
	}
	fingerprint, err := deterministicRequestFingerprint(playerv1connect.PlayerServiceActivatePatternProcedure, request)
	if err != nil {
		return RuntimeMutation{}, err
	}
	command.PayloadFingerprint = fingerprint
	return RuntimeMutation{
		RecognitionHandle: domain.RecognitionHandle(request.GetRecognitionHandle()),
		Command:           command,
	}, nil
}

// PresentationFromProto validates and detaches one exclusive semantic view
// mutation. The repeated context key is an explicit stale-context precondition
// and must agree with the complete projected presentation value.
func PresentationFromProto(request *playerv1.SetPresentationRequest) (RuntimeMutation, error) {
	if request == nil || request.GetPresentation() == nil {
		return RuntimeMutation{}, fmt.Errorf("presentation request is required")
	}
	if err := ValidateMessageSize(request); err != nil {
		return RuntimeMutation{}, err
	}
	if err := validateTerminalMutation(request.GetRecognitionHandle(), request.GetRequestId(), request.GetBroadcastId(), request.GetTerminalId()); err != nil {
		return RuntimeMutation{}, err
	}
	if err := domain.ValidatePublicField(domain.PublicFieldActionTarget, request.GetContextKey()); err != nil {
		return RuntimeMutation{}, err
	}
	generated := request.GetPresentation()
	if generated.GetContextKey() != request.GetContextKey() {
		return RuntimeMutation{}, fmt.Errorf("presentation context precondition does not match")
	}
	presentation := domain.ControllerTerminalPresentation{ContextKey: request.GetContextKey()}
	switch value := generated.Presentation.(type) {
	case *playerv1.ControllerTerminalPresentation_None:
		if value.None == nil {
			return RuntimeMutation{}, fmt.Errorf("none presentation is required")
		}
		presentation.Kind = domain.ControllerTerminalPresentationNone
	case *playerv1.ControllerTerminalPresentation_Menu:
		if value.Menu == nil || domain.ValidatePublicField(domain.PublicFieldActionTarget, value.Menu.GetTargetId()) != nil {
			return RuntimeMutation{}, fmt.Errorf("menu presentation target is invalid")
		}
		presentation.Kind = domain.ControllerTerminalPresentationMenu
		presentation.TargetID = value.Menu.GetTargetId()
	case *playerv1.ControllerTerminalPresentation_Page:
		if value.Page == nil || value.Page.GetPageIndex() > domain.MaxPresentationPageIndex {
			return RuntimeMutation{}, fmt.Errorf("page presentation is invalid")
		}
		presentation.Kind = domain.ControllerTerminalPresentationPage
		presentation.PageIndex = value.Page.GetPageIndex()
	case *playerv1.ControllerTerminalPresentation_Hacking:
		if value.Hacking == nil {
			return RuntimeMutation{}, fmt.Errorf("hacking presentation target is required")
		}
		presentation.Kind = domain.ControllerTerminalPresentationHacking
		switch target := value.Hacking.Target.(type) {
		case *playerv1.HackingPreview_TargetId:
			if domain.ValidatePublicField(domain.PublicFieldActionTarget, target.TargetId) != nil {
				return RuntimeMutation{}, fmt.Errorf("hacking preview target is invalid")
			}
			presentation.TargetID = target.TargetId
		case *playerv1.HackingPreview_PatternId:
			if domain.ValidatePublicField(domain.PublicFieldActionTarget, target.PatternId) != nil {
				return RuntimeMutation{}, fmt.Errorf("hacking preview pattern is invalid")
			}
			presentation.PatternID = target.PatternId
		default:
			return RuntimeMutation{}, fmt.Errorf("hacking preview target is required")
		}
	default:
		return RuntimeMutation{}, fmt.Errorf("presentation variant is required")
	}

	fingerprint, err := deterministicRequestFingerprint(playerv1connect.PlayerServiceSetPresentationProcedure, request)
	if err != nil {
		return RuntimeMutation{}, err
	}
	return RuntimeMutation{
		RecognitionHandle: domain.RecognitionHandle(request.GetRecognitionHandle()),
		Command: domain.RuntimeCommand{
			RequestID: domain.RequestID(request.GetRequestId()), BroadcastID: domain.BroadcastID(request.GetBroadcastId()),
			TerminalID: request.GetTerminalId(), Kind: domain.RuntimeCommandPresentation,
			Presentation: presentation, PayloadFingerprint: fingerprint,
		},
	}, nil
}

// deterministicRequestFingerprint binds replay identity to the generated
// procedure and the exact structurally valid protobuf payload, including
// retained unknown fields, without retaining request bytes in canonical state.
func deterministicRequestFingerprint(procedure string, message proto.Message) (string, error) {
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("fingerprint generated request: %w", err)
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(procedure))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(payload)
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func validateCommonMutation(handle, requestID, broadcastID string) error {
	for _, field := range []struct {
		kind  domain.PublicField
		value string
	}{
		{domain.PublicFieldRecognitionHandle, handle},
		{domain.PublicFieldRequestID, requestID},
		{domain.PublicFieldBroadcastID, broadcastID},
	} {
		if err := domain.ValidatePublicField(field.kind, field.value); err != nil {
			return err
		}
	}
	return nil
}

func validateTerminalMutation(handle, requestID, broadcastID, terminalID string) error {
	if err := validateCommonMutation(handle, requestID, broadcastID); err != nil {
		return err
	}
	return domain.ValidatePublicField(domain.PublicFieldTerminalID, terminalID)
}

// SnapshotToProto maps a detached complete snapshot to generated values.
func SnapshotToProto(snapshot *domain.PersonalizedSnapshot) (*playerv1.PersonalizedSnapshot, error) {
	if snapshot == nil || snapshot.PlayerState == nil {
		return nil, fmt.Errorf("complete snapshot is required")
	}
	presentation, err := TerminalPresentationToProto(snapshot.Terminal)
	if err != nil {
		return nil, err
	}
	return &playerv1.PersonalizedSnapshot{
		RecognitionHandle:    string(snapshot.RecognitionHandle),
		Revision:             snapshot.Revision,
		PlayerState:          PlayerStateToProto(snapshot.PlayerState),
		TerminalPresentation: presentation,
	}, nil
}

// CompoundUpdateToProto maps complete changed components; nil means unchanged.
func CompoundUpdateToProto(update *domain.CompoundUpdate) (*playerv1.CompoundUpdate, error) {
	if update == nil || update.Revision == 0 {
		return nil, fmt.Errorf("compound update revision is required")
	}
	result := &playerv1.CompoundUpdate{Revision: update.Revision}
	result.PlayerState = PlayerStateToProto(update.Player)
	if update.Terminal != nil {
		presentation, err := TerminalPresentationToProto(*update.Terminal)
		if err != nil {
			return nil, err
		}
		result.TerminalPresentation = presentation
	}
	result.Navigation = NavigationToProto(update.Nav)
	result.Hacking = HackToProto(update.Hack)
	return result, nil
}

// PlayerStateToProto maps one complete personalized player projection.
func PlayerStateToProto(state *domain.PlayerState) *playerv1.PlayerState {
	if state == nil {
		return nil
	}
	result := &playerv1.PlayerState{
		LogicalSessionId: string(state.SessionID),
		FallbackName:     state.FallbackName,
		Role:             roleToProto(state.Role),
		Phase:            phaseToProto(state.Phase),
		Roster:           make([]*playerv1.RosterEntry, 0, len(state.Roster)),
	}
	if state.Character != nil {
		result.AssignedCharacter = &playerv1.AssignedCharacter{
			CharacterId: string(state.Character.ID),
			DisplayName: state.Character.Name,
		}
	}
	if state.BroadcastID != "" {
		value := string(state.BroadcastID)
		result.BroadcastId = &value
	}
	if state.ActiveTerminalID != "" {
		value := state.ActiveTerminalID
		result.ActiveTerminalId = &value
	}
	for _, entry := range state.Roster {
		result.Roster = append(result.Roster, &playerv1.RosterEntry{
			CharacterId:  string(entry.ID),
			DisplayName:  entry.Name,
			Availability: rosterToProto(entry.Status),
		})
	}
	if state.Role == domain.PlayerRoleActive {
		result.Notice = playerNoticeToProto(state.Notice)
	}
	return result
}

// TerminalPresentationToProto enforces exactly one terminal variant.
func TerminalPresentationToProto(presentation domain.TerminalPresentation) (*playerv1.TerminalPresentation, error) {
	switch {
	case presentation.Live != nil && !presentation.NoLiveTerminal:
		return &playerv1.TerminalPresentation{Presentation: &playerv1.TerminalPresentation_LiveTerminal{LiveTerminal: LiveToProto(presentation.Live)}}, nil
	case presentation.Live == nil && presentation.NoLiveTerminal:
		return &playerv1.TerminalPresentation{Presentation: &playerv1.TerminalPresentation_NoLiveTerminal{NoLiveTerminal: &playerv1.NoLiveTerminal{}}}, nil
	default:
		return nil, fmt.Errorf("terminal presentation must set exactly one variant")
	}
}

// LiveToProto maps a complete player-safe live projection.
func LiveToProto(state *domain.PublicLiveState) *playerv1.LiveTerminal {
	if state == nil {
		return nil
	}
	result := &playerv1.LiveTerminal{
		TerminalId:             state.TerminalID,
		TerminalName:           state.TerminalName,
		Tree:                   ContentNodeToProto(state.Tree),
		HackLevel:              int32(state.HackLevel),
		IntroText:              state.IntroText,
		Navigation:             NavigationToProto(&state.Nav),
		Hacking:                HackToProto(state.Hack),
		CommandExecution:       commandExecutionToProto(state.CommandExecution),
		TerminalNavigation:     terminalNavigationToProto(state.TerminalNavigation),
		ControllerPresentation: controllerPresentationToProto(state.Presentation),
	}
	for _, effect := range state.Effects {
		if mapped, ok := terminalPresentationEffectToProto(effect); ok {
			result.Effects = append(result.Effects, mapped)
		}
	}
	return result
}

func terminalPresentationEffectToProto(effect domain.TerminalPresentationEffect) (playerv1.TerminalPresentationEffect, bool) {
	switch effect {
	case domain.TerminalPresentationEffectDisplayUnstable:
		return playerv1.TerminalPresentationEffect_TERMINAL_PRESENTATION_EFFECT_DISPLAY_UNSTABLE, true
	case domain.TerminalPresentationEffectUnspecified:
		return playerv1.TerminalPresentationEffect_TERMINAL_PRESENTATION_EFFECT_UNSPECIFIED, false
	default:
		return playerv1.TerminalPresentationEffect_TERMINAL_PRESENTATION_EFFECT_UNSPECIFIED, false
	}
}

func controllerPresentationToProto(presentation domain.ControllerTerminalPresentation) *playerv1.ControllerTerminalPresentation {
	result := &playerv1.ControllerTerminalPresentation{ContextKey: presentation.ContextKey}
	switch presentation.Kind {
	case domain.ControllerTerminalPresentationMenu:
		result.Presentation = &playerv1.ControllerTerminalPresentation_Menu{Menu: &playerv1.MenuSelection{TargetId: presentation.TargetID}}
	case domain.ControllerTerminalPresentationPage:
		result.Presentation = &playerv1.ControllerTerminalPresentation_Page{Page: &playerv1.InformationPagePosition{PageIndex: presentation.PageIndex}}
	case domain.ControllerTerminalPresentationHacking:
		hacking := &playerv1.HackingPreview{}
		if presentation.PatternID != "" {
			hacking.Target = &playerv1.HackingPreview_PatternId{PatternId: presentation.PatternID}
		} else {
			hacking.Target = &playerv1.HackingPreview_TargetId{TargetId: presentation.TargetID}
		}
		result.Presentation = &playerv1.ControllerTerminalPresentation_Hacking{Hacking: hacking}
	default:
		result.Presentation = &playerv1.ControllerTerminalPresentation_None{None: &playerv1.NoControllerTerminalPresentation{}}
	}
	return result
}

func terminalNavigationToProto(presentation *domain.TerminalNavigationPresentation) *playerv1.TerminalNavigationPresentation {
	if presentation == nil {
		return nil
	}
	result := &playerv1.TerminalNavigationPresentation{RouteDepth: presentation.RouteDepth}
	if presentation.ReturnTarget != nil {
		result.ReturnTarget = &playerv1.TerminalReturnTarget{
			TerminalId: presentation.ReturnTarget.TerminalID, TerminalName: presentation.ReturnTarget.TerminalName,
		}
	}
	if presentation.Pending != nil {
		result.Pending = &playerv1.PendingTerminalNavigationPresentation{
			Direction:          terminalNavigationDirectionToProto(presentation.Pending.Direction),
			TargetTerminalId:   presentation.Pending.TargetTerminalID,
			TargetTerminalName: presentation.Pending.TargetTerminalName,
		}
	}
	return result
}

func terminalNavigationDirectionToProto(direction domain.TerminalNavigationDirection) playerv1.TerminalNavigationDirection {
	switch direction {
	case domain.TerminalNavigationForward:
		return playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_FORWARD
	case domain.TerminalNavigationReturn:
		return playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_RETURN
	default:
		return playerv1.TerminalNavigationDirection_TERMINAL_NAVIGATION_DIRECTION_UNSPECIFIED
	}
}

// commandExecutionToProto maps only the shared phase and command identity.
// The master request identity and authored confirmation text have no public
// domain field and therefore cannot cross this adapter.
func commandExecutionToProto(presentation *domain.CommandExecutionPresentation) *playerv1.CommandExecutionPresentation {
	if presentation == nil {
		return nil
	}
	result := &playerv1.CommandExecutionPresentation{CommandNodeId: presentation.CommandID}
	switch presentation.Phase {
	case domain.CommandExecutionPhasePending:
		result.Phase = playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_PENDING
	case domain.CommandExecutionPhaseRejected:
		result.Phase = playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_REJECTED
	default:
		result.Phase = playerv1.CommandExecutionPhase_COMMAND_EXECUTION_PHASE_UNSPECIFIED
	}
	return result
}

// playerNoticeToProto emits only the stable enum. Diagnostic causes, paths,
// and master-facing error text are deliberately not representable publicly.
func playerNoticeToProto(notice *domain.PlayerNotice) *playerv1.PlayerNotice {
	if notice == nil {
		return nil
	}
	result := &playerv1.PlayerNotice{}
	switch notice.Kind {
	case domain.PlayerNoticeCommandPersistenceFailed:
		result.Kind = playerv1.PlayerNoticeKind_PLAYER_NOTICE_KIND_COMMAND_PERSISTENCE_FAILED
	default:
		result.Kind = playerv1.PlayerNoticeKind_PLAYER_NOTICE_KIND_UNSPECIFIED
	}
	return result
}

// ContentNodeToProto maps the established tagged domain tree to an exclusive
// generated variant.
func ContentNodeToProto(node domain.ContentNode) *playerv1.ContentNode {
	result := &playerv1.ContentNode{Id: node.ID, Name: node.Name}
	switch node.Type {
	case domain.NodeFolder:
		children := make([]*playerv1.ContentNode, len(node.Children))
		for index := range node.Children {
			children[index] = ContentNodeToProto(node.Children[index])
		}
		result.Content = &playerv1.ContentNode_Folder{Folder: &playerv1.ContentFolder{Children: children}}
	case domain.NodeCommand:
		command := &playerv1.ContentCommand{Text: node.Text}
		if node.Available != nil {
			command.Available = new(*node.Available)
		}
		result.Content = &playerv1.ContentNode_Command{Command: command}
	case domain.NodeEntry:
		result.Content = &playerv1.ContentNode_Entry{Entry: &playerv1.ContentEntry{Description: node.Description}}
	}
	return result
}

// NavigationToProto maps a complete shared navigation projection.
func NavigationToProto(state *domain.NavState) *playerv1.NavigationState {
	if state == nil {
		return nil
	}
	result := &playerv1.NavigationState{Path: append([]string(nil), state.Path...)}
	if state.Mode == "entry" {
		result.Mode = playerv1.NavigationMode_NAVIGATION_MODE_ENTRY
	} else {
		result.Mode = playerv1.NavigationMode_NAVIGATION_MODE_LIST
	}
	if state.ViewEntryID != nil {
		value := *state.ViewEntryID
		result.ViewEntryId = &value
	}
	if state.CommandNodeID != nil {
		value := *state.CommandNodeID
		result.CommandNodeId = &value
	}
	return result
}

// HackToProto maps only the existing player-safe hacking projection.
func HackToProto(state *domain.PublicHackState) *playerv1.PublicHackState {
	if state == nil {
		return nil
	}
	result := &playerv1.PublicHackState{
		Level:        int32(state.Level),
		WordLength:   int32(state.WordLength),
		AttemptsMax:  int32(state.AttemptsMax),
		AttemptsLeft: int32(state.AttemptsLeft),
		Solved:       state.Solved,
		Failed:       state.Failed,
		Log:          append([]string(nil), state.Log...),
	}
	for _, column := range state.Columns {
		output := &playerv1.PublicHackColumn{Addresses: append([]string(nil), column.Addresses...), Text: column.Text}
		for _, word := range column.Words {
			output.Words = append(output.Words, &playerv1.PublicHackWord{Id: word.ID, Start: int32(word.Start), Length: int32(word.Length)})
		}
		result.Columns = append(result.Columns, output)
	}
	for _, pattern := range state.Patterns {
		result.Patterns = append(result.Patterns, &playerv1.PublicHackPattern{
			PatternId: pattern.ID,
			Row:       int32(pattern.Row),
			Start:     int32(pattern.Start),
			End:       int32(pattern.End),
			Used:      pattern.Used,
		})
	}
	return result
}

// ActionResultToProto maps only the stable public reason vocabulary.
func ActionResultToProto(result domain.ActionResult) *playerv1.ActionResult {
	return &playerv1.ActionResult{
		RequestId: result.RequestID,
		Accepted:  result.Accepted,
		Reason:    actionReasonToProto(result.Reason),
		Revision:  result.Revision,
	}
}

// SoundManifestToProto maps sorted safe relative paths without origins or
// filesystem capabilities.
func SoundManifestToProto(manifest domain.SoundManifest) *playerv1.SoundManifestResponse {
	return &playerv1.SoundManifestResponse{Category: soundCategoryToProto(manifest.Category), Assets: append([]string(nil), manifest.Assets...)}
}

// SoundCategoryFromProto rejects absent and unknown enum values before any
// asset filesystem capability is consulted.
func SoundCategoryFromProto(category playerv1.SoundCategory) (domain.SoundCategory, error) {
	var value string
	switch category {
	case playerv1.SoundCategory_SOUND_CATEGORY_AMBIENT:
		value = string(domain.SoundCategoryAmbient)
	case playerv1.SoundCategory_SOUND_CATEGORY_HACK_GOOD:
		value = string(domain.SoundCategoryHackGood)
	case playerv1.SoundCategory_SOUND_CATEGORY_HACK_BAD:
		value = string(domain.SoundCategoryHackBad)
	case playerv1.SoundCategory_SOUND_CATEGORY_MENU_FOCUS:
		value = string(domain.SoundCategoryMenuFocus)
	case playerv1.SoundCategory_SOUND_CATEGORY_SINGLE:
		value = string(domain.SoundCategorySingle)
	case playerv1.SoundCategory_SOUND_CATEGORY_MULTIPLE:
		value = string(domain.SoundCategoryMultiple)
	case playerv1.SoundCategory_SOUND_CATEGORY_ENTER:
		value = string(domain.SoundCategoryEnter)
	case playerv1.SoundCategory_SOUND_CATEGORY_CHARSCROLL:
		value = string(domain.SoundCategoryCharscroll)
	default:
		return "", fmt.Errorf("sound category is invalid")
	}
	return domain.ValidateSoundCategory(value)
}

func roleToProto(role domain.PlayerRole) playerv1.PlayerRole {
	switch role {
	case domain.PlayerRoleUnassigned:
		return playerv1.PlayerRole_PLAYER_ROLE_UNASSIGNED
	case domain.PlayerRoleActive:
		return playerv1.PlayerRole_PLAYER_ROLE_ACTIVE
	case domain.PlayerRoleObserver:
		return playerv1.PlayerRole_PLAYER_ROLE_OBSERVER
	default:
		return playerv1.PlayerRole_PLAYER_ROLE_UNSPECIFIED
	}
}

func phaseToProto(phase domain.PlayerPhase) playerv1.PlayerPhase {
	switch phase {
	case domain.PlayerPhaseNoBroadcast:
		return playerv1.PlayerPhase_PLAYER_PHASE_NO_BROADCAST
	case domain.PlayerPhaseSelecting:
		return playerv1.PlayerPhase_PLAYER_PHASE_SELECTING
	case domain.PlayerPhaseWaiting:
		return playerv1.PlayerPhase_PLAYER_PHASE_WAITING
	case domain.PlayerPhaseControlling:
		return playerv1.PlayerPhase_PLAYER_PHASE_CONTROLLING
	case domain.PlayerPhaseObserving:
		return playerv1.PlayerPhase_PLAYER_PHASE_OBSERVING
	default:
		return playerv1.PlayerPhase_PLAYER_PHASE_UNSPECIFIED
	}
}

func rosterToProto(status domain.RosterStatus) playerv1.RosterAvailability {
	if status == domain.RosterStatusAvailable {
		return playerv1.RosterAvailability_ROSTER_AVAILABILITY_AVAILABLE
	}
	if status == domain.RosterStatusClaimed {
		return playerv1.RosterAvailability_ROSTER_AVAILABILITY_CLAIMED
	}
	return playerv1.RosterAvailability_ROSTER_AVAILABILITY_UNSPECIFIED
}

func actionReasonToProto(reason domain.ActionReason) playerv1.ActionReason {
	switch reason {
	case domain.ActionReasonAccepted:
		return playerv1.ActionReason_ACTION_REASON_ACCEPTED
	case domain.ActionReasonInvalidSession:
		return playerv1.ActionReason_ACTION_REASON_INVALID_SESSION
	case domain.ActionReasonStaleBroadcast:
		return playerv1.ActionReason_ACTION_REASON_STALE_BROADCAST
	case domain.ActionReasonUnassigned:
		return playerv1.ActionReason_ACTION_REASON_UNASSIGNED
	case domain.ActionReasonNotController:
		return playerv1.ActionReason_ACTION_REASON_NOT_CONTROLLER
	case domain.ActionReasonControllerDisconnected:
		return playerv1.ActionReason_ACTION_REASON_CONTROLLER_DISCONNECTED
	case domain.ActionReasonStaleTerminal:
		return playerv1.ActionReason_ACTION_REASON_STALE_TERMINAL
	case domain.ActionReasonInvalidAction:
		return playerv1.ActionReason_ACTION_REASON_INVALID_ACTION
	case domain.ActionReasonConflict:
		return playerv1.ActionReason_ACTION_REASON_CONFLICT
	case domain.ActionReasonDuplicate:
		return playerv1.ActionReason_ACTION_REASON_DUPLICATE
	default:
		return playerv1.ActionReason_ACTION_REASON_UNSPECIFIED
	}
}

func soundCategoryToProto(category domain.SoundCategory) playerv1.SoundCategory {
	switch category {
	case domain.SoundCategoryAmbient:
		return playerv1.SoundCategory_SOUND_CATEGORY_AMBIENT
	case domain.SoundCategoryHackGood:
		return playerv1.SoundCategory_SOUND_CATEGORY_HACK_GOOD
	case domain.SoundCategoryHackBad:
		return playerv1.SoundCategory_SOUND_CATEGORY_HACK_BAD
	case domain.SoundCategoryMenuFocus:
		return playerv1.SoundCategory_SOUND_CATEGORY_MENU_FOCUS
	case domain.SoundCategorySingle:
		return playerv1.SoundCategory_SOUND_CATEGORY_SINGLE
	case domain.SoundCategoryMultiple:
		return playerv1.SoundCategory_SOUND_CATEGORY_MULTIPLE
	case domain.SoundCategoryEnter:
		return playerv1.SoundCategory_SOUND_CATEGORY_ENTER
	case domain.SoundCategoryCharscroll:
		return playerv1.SoundCategory_SOUND_CATEGORY_CHARSCROLL
	default:
		return playerv1.SoundCategory_SOUND_CATEGORY_UNSPECIFIED
	}
}
