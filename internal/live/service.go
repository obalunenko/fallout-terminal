// Package live owns the process-local, server-authoritative live terminal.
package live

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/hack"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/nav"
)

// Service serializes every canonical live-state transition. Values returned
// across this boundary are detached projections and may be freely mutated by
// callers without changing the canonical aggregate.
type Service struct {
	mu            sync.RWMutex
	live          *domain.LiveState
	random        hack.Random
	words         hack.WordSource
	generationIDs generationIDSource
}

type generationIDSource interface {
	Next() string
}

type cryptoGenerationIDSource struct {
	fallback atomic.Uint64
}

func (source *cryptoGenerationIDSource) Next() string {
	var value [16]byte
	if _, err := cryptorand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), source.fallback.Add(1))
}

// New returns an empty live-state service. Randomness and the word source are
// injectable to keep puzzle generation deterministic in tests.
func New(random hack.Random, words hack.WordSource) *Service {
	return &Service{random: random, words: words, generationIDs: &cryptoGenerationIDSource{}}
}

// Set installs a fresh live terminal, resets navigation, and creates a new
// puzzle when hackLevel is greater than zero.
func (service *Service) Set(terminalID, terminalName string, tree domain.ContentNode, hackLevel int, introText string) *domain.PublicLiveState {
	service.mu.Lock()
	defer service.mu.Unlock()

	state := &domain.LiveState{
		TerminalID:   terminalID,
		TerminalName: terminalName,
		Tree:         domain.CloneContentNode(tree),
		HackLevel:    hackLevel,
		IntroText:    introText,
		Nav:          nav.Default(),
	}
	if hackLevel > 0 {
		state.Hack = hack.GenerateBoard(service.generationIDs.Next(), hackLevel, service.random, service.words)
	}
	service.live = state
	return publicLiveState(state)
}

// Update replaces published content while retaining the current puzzle and
// repairing shared navigation against the new tree. A nil introText preserves
// the current introduction.
func (service *Service) Update(tree domain.ContentNode, introText *string) (*domain.PublicLiveState, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.live == nil {
		return nil, false
	}

	service.live.Tree = domain.CloneContentNode(tree)
	if introText != nil {
		service.live.IntroText = *introText
	}
	service.live.Nav = nav.Revalidate(service.live.Nav, service.live.Tree)
	return publicLiveState(service.live), true
}

// Clear removes the canonical live terminal. It is safe to call repeatedly.
func (service *Service) Clear() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.live = nil
}

// Snapshot returns the current immutable player projection, or nil when no
// terminal is live.
func (service *Service) Snapshot() *domain.PublicLiveState {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return publicLiveState(service.live)
}

// Apply executes one already-authorized shared command against the
// coordinator-owned terminal checkpoint. The caller retains ownership of the
// canonical state; the returned player projection is deeply detached. This
// boundary deliberately performs no authorization and invokes no callbacks,
// allowing the coordinator to reject ineligible commands before live rules or
// their randomness are reached.
func (service *Service) Apply(state *domain.TerminalRuntime, command domain.RuntimeCommand) (*domain.PublicLiveState, bool) {
	if service == nil || state == nil {
		return nil, false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.applyRuntimeLocked(state, command)
}

// CreateRuntime builds a fresh coordinator-owned terminal checkpoint. It does
// not install the checkpoint into the legacy process-global live slot.
func (service *Service) CreateRuntime(target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	if service == nil {
		return nil, nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.createRuntimeLocked(target)
}

func (service *Service) createRuntimeLocked(target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	authored := domain.CloneContentNode(target.Tree)
	state := &domain.TerminalRuntime{
		TerminalID: target.TerminalID, TerminalName: target.TerminalName,
		AuthoredTree: authored, Tree: effectiveTree(authored, target.CommandStates),
		HackLevel: target.HackLevel, IntroText: target.IntroText,
		Nav: nav.Default(), Lifecycle: domain.TerminalLifecycleActive,
		CommandStates: cloneCommandStates(target.CommandStates),
	}
	if target.HackLevel > 0 {
		state.Hack = hack.GenerateBoard(service.generationIDs.Next(), target.HackLevel, service.random, service.words)
	}
	revalidateControllerPresentation(state)
	return state, publicTerminalRuntime(state)
}

// ResetFailedHack creates a fresh checkpoint only when the supplied source is
// the still-current failed puzzle for the same terminal. The caller owns the
// atomic slot replacement; this helper owns generation and projection rules.
func (service *Service) ResetFailedHack(source *domain.TerminalRuntime, target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	if service == nil || source == nil || target.TerminalID == "" || source.TerminalID != target.TerminalID || source.Hack == nil || !source.Hack.Failed || source.Hack.Solved {
		return nil, nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.createRuntimeLocked(target)
}

// UpdateRuntime applies the latest authored content to an existing checkpoint
// while retaining its private puzzle and repairing navigation.
func (service *Service) UpdateRuntime(state *domain.TerminalRuntime, target domain.TerminalTarget) *domain.PublicLiveState {
	if service == nil || state == nil || target.TerminalID == "" || state.TerminalID != target.TerminalID {
		return nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	state.TerminalName = target.TerminalName
	state.AuthoredTree = domain.CloneContentNode(target.Tree)
	state.CommandStates = cloneCommandStates(target.CommandStates)
	state.Tree = effectiveTree(state.AuthoredTree, state.CommandStates)
	state.IntroText = target.IntroText
	if state.Hack == nil {
		state.HackLevel = target.HackLevel
	}
	state.Nav = nav.Revalidate(state.Nav, state.Tree)
	revalidateControllerPresentation(state)
	return publicTerminalRuntime(state)
}

// ProjectFacility refreshes one checkpoint's effective presentation from its
// retained authored tree and the shared facility snapshot.
func (service *Service) ProjectFacility(state *domain.TerminalRuntime, facility *domain.Facility) *domain.PublicLiveState {
	if service == nil || state == nil {
		return nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	authored := state.AuthoredTree
	if authored.ID == "" {
		authored = state.Tree
		state.AuthoredTree = domain.CloneContentNode(authored)
	}
	projection := projectFacility(authored, state.CommandStates, facility, state.TerminalID)
	state.Tree = projection.Tree
	state.Effects = slices.Clone(projection.Effects)
	state.Nav = nav.Revalidate(state.Nav, state.Tree)
	revalidateControllerPresentation(state)
	return publicTerminalRuntime(state)
}

// ProjectRuntime returns a detached, secret-free checkpoint projection.
func (service *Service) ProjectRuntime(state *domain.TerminalRuntime) *domain.PublicLiveState {
	if service == nil || state == nil {
		return nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	revalidateControllerPresentation(state)
	return publicTerminalRuntime(state)
}

// SuspendRuntime changes only checkpoint eligibility. The private navigation
// and hacking aggregates remain in place without passing through a projection.
func (service *Service) SuspendRuntime(state *domain.TerminalRuntime) {
	if service == nil || state == nil {
		return
	}
	service.mu.Lock()
	state.Lifecycle = domain.TerminalLifecycleSuspended
	service.mu.Unlock()
}

// ReactivateRuntime reapplies current authored metadata and content while
// preserving the exact private puzzle and revalidating navigation against the
// refreshed tree.
func (service *Service) ReactivateRuntime(state *domain.TerminalRuntime, target domain.TerminalTarget) *domain.PublicLiveState {
	if service == nil || state == nil || target.TerminalID == "" || state.TerminalID != target.TerminalID {
		return nil
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	state.TerminalName = target.TerminalName
	state.AuthoredTree = domain.CloneContentNode(target.Tree)
	state.CommandStates = cloneCommandStates(target.CommandStates)
	state.Tree = effectiveTree(state.AuthoredTree, state.CommandStates)
	state.IntroText = target.IntroText
	if state.Hack == nil {
		state.HackLevel = target.HackLevel
	}
	state.CommandExecution = nil
	state.Nav = nav.Revalidate(state.Nav, state.Tree)
	state.Lifecycle = domain.TerminalLifecycleActive
	revalidateControllerPresentation(state)
	return publicTerminalRuntime(state)
}

// DiscardRuntime creates a wholly fresh checkpoint from the latest authored
// payload. The caller replaces the prior slot atomically in coordinator state.
func (service *Service) DiscardRuntime(target domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState) {
	return service.CreateRuntime(target)
}

// ApplyNav applies a player navigation request. The boolean reports whether a
// live terminal existed; valid no-op requests remain observable for protocol
// compatibility.
func (service *Service) ApplyNav(action, nodeID string) (*domain.NavState, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.live == nil {
		return nil, false
	}

	runtime := terminalRuntime(service.live)
	projection, ok := service.applyRuntimeLocked(runtime, domain.RuntimeCommand{
		Kind: domain.RuntimeCommandNavigate, Action: action, NodeID: nodeID,
	})
	if !ok {
		return nil, false
	}
	service.live.Nav = runtime.Nav
	return &projection.Nav, true
}

// ApplyHackGuess applies a candidate or filler guess to an active puzzle.
func (service *Service) ApplyHackGuess(targetID string) (*domain.PublicHackState, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if !activePuzzle(service.live) {
		return nil, false
	}

	runtime := terminalRuntime(service.live)
	projection, ok := service.applyRuntimeLocked(runtime, domain.RuntimeCommand{
		Kind: domain.RuntimeCommandGuess, TargetID: targetID,
	})
	if !ok {
		return nil, false
	}
	service.live.Hack = runtime.Hack
	return projection.Hack, true
}

// ApplyHackPattern atomically validates and consumes one current pattern. The
// publication callback runs while the live-service mutex is held and must not
// call back into the service or perform blocking transport work.
func (service *Service) ApplyHackPattern(patternID string, publish func(*domain.PublicHackState)) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	if !activePuzzle(service.live) {
		return false
	}

	runtime := terminalRuntime(service.live)
	projection, ok := service.applyRuntimeLocked(runtime, domain.RuntimeCommand{
		Kind: domain.RuntimeCommandActivatePattern, PatternID: patternID,
	})
	if !ok {
		return false
	}
	service.live.Hack = runtime.Hack
	publicHack := projection.Hack

	if publish != nil {
		publish(publicHack)
	}
	return true
}

// ForceHackSuccess completes an active puzzle without spending an attempt.
func (service *Service) ForceHackSuccess() (*domain.PublicHackState, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if !activePuzzle(service.live) {
		return nil, false
	}

	hack.ForceSuccess(service.live.Hack)
	return hack.PublicState(service.live.Hack), true
}

// ForceRuntimeHackSuccess completes one coordinator-owned active runtime
// without touching the legacy live slot or spending an attempt.
func (service *Service) ForceRuntimeHackSuccess(state *domain.TerminalRuntime) (*domain.PublicLiveState, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if state == nil || state.Lifecycle != domain.TerminalLifecycleActive || !activeRuntimePuzzle(state) {
		return nil, false
	}

	hack.ForceSuccess(state.Hack)
	revalidateControllerPresentation(state)
	return publicTerminalRuntime(state), true
}

func (service *Service) applyRuntimeLocked(state *domain.TerminalRuntime, command domain.RuntimeCommand) (*domain.PublicLiveState, bool) {
	if state == nil {
		return nil, false
	}
	if state.CommandExecution != nil {
		switch state.CommandExecution.Phase {
		case domain.CommandExecutionPhasePending:
			return nil, false
		case domain.CommandExecutionPhaseRejected:
			if command.Kind != domain.RuntimeCommandNavigate || command.Action != "back" {
				return nil, false
			}
			state.CommandExecution = nil
			state.Nav = nav.ApplyAction(state.Nav, state.Tree, command.Action, command.NodeID)
			revalidateControllerPresentation(state)
			return publicTerminalRuntime(state), true
		}
	}

	switch command.Kind {
	case domain.RuntimeCommandNavigate:
		state.Nav = nav.ApplyAction(state.Nav, state.Tree, command.Action, command.NodeID)
	case domain.RuntimeCommandGuess:
		if !activeRuntimePuzzle(state) {
			return nil, false
		}
		attemptsBefore := state.Hack.AttemptsLeft
		solvedBefore := state.Hack.Solved
		failedBefore := state.Hack.Failed
		logLengthBefore := len(state.Hack.Log)
		hack.ApplyGuess(state.Hack, command.TargetID)
		if state.Hack.AttemptsLeft == attemptsBefore && state.Hack.Solved == solvedBefore && state.Hack.Failed == failedBefore && len(state.Hack.Log) == logLengthBefore {
			return nil, false
		}
	case domain.RuntimeCommandActivatePattern:
		if !activeRuntimePuzzle(state) || !hack.ApplyPattern(state.Hack, command.PatternID, service.random) {
			return nil, false
		}
	case domain.RuntimeCommandPresentation:
		if !validControllerPresentation(state, command.Presentation) || state.Presentation == command.Presentation {
			return nil, false
		}
		state.Presentation = command.Presentation
	default:
		return nil, false
	}
	revalidateControllerPresentation(state)
	return publicTerminalRuntime(state), true
}

func terminalRuntime(state *domain.LiveState) *domain.TerminalRuntime {
	if state == nil {
		return nil
	}
	return &domain.TerminalRuntime{
		TerminalID: state.TerminalID, TerminalName: state.TerminalName,
		AuthoredTree: domain.CloneContentNode(state.Tree), Tree: domain.CloneContentNode(state.Tree),
		HackLevel: state.HackLevel, IntroText: state.IntroText,
		Nav: state.Nav, Hack: state.Hack, Lifecycle: domain.TerminalLifecycleActive,
	}
}

func publicTerminalRuntime(state *domain.TerminalRuntime) *domain.PublicLiveState {
	if state == nil {
		return nil
	}
	return &domain.PublicLiveState{
		TerminalID: state.TerminalID, TerminalName: state.TerminalName,
		Tree: domain.CloneContentNode(state.Tree), HackLevel: state.HackLevel, IntroText: state.IntroText,
		Nav: cloneNav(state.Nav), Hack: hack.PublicState(state.Hack),
		CommandExecution: cloneCommandExecution(state.CommandExecution),
		Presentation:     state.Presentation,
		Effects:          slices.Clone(state.Effects),
	}
}

func activePuzzle(state *domain.LiveState) bool {
	return state != nil && state.Hack != nil && !state.Hack.Solved && !state.Hack.Failed
}

func activeRuntimePuzzle(state *domain.TerminalRuntime) bool {
	return state != nil && state.Hack != nil && !state.Hack.Solved && !state.Hack.Failed
}

func revalidateControllerPresentation(state *domain.TerminalRuntime) {
	if state == nil {
		return
	}
	contextKey, kind := controllerPresentationContext(state)
	current := state.Presentation
	if current.ContextKey == contextKey && validControllerPresentation(state, current) {
		return
	}

	switch kind {
	case domain.ControllerTerminalPresentationHacking:
		state.Presentation = domain.ControllerTerminalPresentation{
			Kind: domain.ControllerTerminalPresentationNone, ContextKey: contextKey,
		}
	case domain.ControllerTerminalPresentationPage:
		state.Presentation = domain.ControllerTerminalPresentation{
			Kind: domain.ControllerTerminalPresentationPage, ContextKey: contextKey,
		}
	case domain.ControllerTerminalPresentationMenu:
		folder := currentPresentationFolder(state)
		if folder != nil && len(folder.Children) != 0 {
			state.Presentation = domain.ControllerTerminalPresentation{
				Kind: domain.ControllerTerminalPresentationMenu, ContextKey: contextKey, TargetID: folder.Children[0].ID,
			}
			return
		}
		state.Presentation = domain.ControllerTerminalPresentation{
			Kind: domain.ControllerTerminalPresentationNone, ContextKey: contextKey,
		}
	default:
		state.Presentation = domain.ControllerTerminalPresentation{
			Kind: domain.ControllerTerminalPresentationNone, ContextKey: contextKey,
		}
	}
}

func validControllerPresentation(state *domain.TerminalRuntime, presentation domain.ControllerTerminalPresentation) bool {
	if state == nil {
		return false
	}
	contextKey, contextKind := controllerPresentationContext(state)
	if presentation.ContextKey == "" || presentation.ContextKey != contextKey {
		return false
	}
	switch presentation.Kind {
	case domain.ControllerTerminalPresentationNone:
		return presentation.TargetID == "" && presentation.PatternID == "" && presentation.PageIndex == 0
	case domain.ControllerTerminalPresentationMenu:
		if contextKind != domain.ControllerTerminalPresentationMenu || presentation.TargetID == "" || presentation.PatternID != "" || presentation.PageIndex != 0 {
			return false
		}
		folder := currentPresentationFolder(state)
		if folder == nil {
			return false
		}
		for index := range folder.Children {
			if folder.Children[index].ID == presentation.TargetID {
				return true
			}
		}
		return false
	case domain.ControllerTerminalPresentationPage:
		return contextKind == domain.ControllerTerminalPresentationPage && presentation.TargetID == "" && presentation.PatternID == "" && presentation.PageIndex <= domain.MaxPresentationPageIndex
	case domain.ControllerTerminalPresentationHacking:
		if contextKind != domain.ControllerTerminalPresentationHacking || presentation.PageIndex != 0 {
			return false
		}
		if presentation.TargetID != "" && presentation.PatternID == "" {
			return validHackPreviewTarget(state.Hack, presentation.TargetID)
		}
		if presentation.PatternID != "" && presentation.TargetID == "" {
			for _, pattern := range hack.PublicState(state.Hack).Patterns {
				if pattern.ID == presentation.PatternID && !pattern.Used {
					return true
				}
			}
		}
	}
	return false
}

func controllerPresentationContext(state *domain.TerminalRuntime) (string, domain.ControllerTerminalPresentationKind) {
	if activeRuntimePuzzle(state) {
		digest := sha256.Sum256([]byte(state.Hack.GenerationID))
		return fmt.Sprintf("hack:%x", digest[:8]), domain.ControllerTerminalPresentationHacking
	}
	if state.Nav.ViewEntryID != nil {
		return "entry:" + *state.Nav.ViewEntryID, domain.ControllerTerminalPresentationPage
	}
	if state.Nav.CommandNodeID != nil {
		return "command:" + *state.Nav.CommandNodeID, domain.ControllerTerminalPresentationPage
	}
	return "menu:" + strings.Join(state.Nav.Path, "/"), domain.ControllerTerminalPresentationMenu
}

func currentPresentationFolder(state *domain.TerminalRuntime) *domain.ContentNode {
	if state == nil || len(state.Nav.Path) == 0 {
		return nil
	}
	return findPresentationNode(&state.Tree, state.Nav.Path[len(state.Nav.Path)-1])
}

func findPresentationNode(node *domain.ContentNode, nodeID string) *domain.ContentNode {
	if node == nil {
		return nil
	}
	if node.ID == nodeID {
		return node
	}
	for index := range node.Children {
		if found := findPresentationNode(&node.Children[index], nodeID); found != nil {
			return found
		}
	}
	return nil
}

func validHackPreviewTarget(state *domain.HackState, targetID string) bool {
	if !activeHackState(state) {
		return false
	}
	if _, ok := state.WordsByID[targetID]; ok {
		return true
	}
	parts := strings.Split(targetID, ":")
	if len(parts) != 2 {
		return false
	}
	columnIndex, columnErr := strconv.Atoi(parts[0])
	characterIndex, characterErr := strconv.Atoi(parts[1])
	if columnErr != nil || characterErr != nil || columnIndex < 0 || characterIndex < 0 || columnIndex >= len(state.Columns) {
		return false
	}
	column := state.Columns[columnIndex]
	if characterIndex >= len(column.Text) {
		return false
	}
	for _, word := range column.Words {
		if characterIndex >= word.Start && characterIndex < word.Start+word.Length {
			return false
		}
	}
	return true
}

func activeHackState(state *domain.HackState) bool {
	return state != nil && !state.Solved && !state.Failed
}

func publicLiveState(state *domain.LiveState) *domain.PublicLiveState {
	if state == nil {
		return nil
	}
	return &domain.PublicLiveState{
		TerminalID:   state.TerminalID,
		TerminalName: state.TerminalName,
		Tree:         domain.CloneContentNode(state.Tree),
		HackLevel:    state.HackLevel,
		IntroText:    state.IntroText,
		Nav:          cloneNav(state.Nav),
		Hack:         hack.PublicState(state.Hack),
	}
}

func cloneNav(state domain.NavState) domain.NavState {
	clone := state
	clone.Path = append([]string(nil), state.Path...)
	if state.ViewEntryID != nil {
		value := *state.ViewEntryID
		clone.ViewEntryID = &value
	}
	if state.CommandNodeID != nil {
		value := *state.CommandNodeID
		clone.CommandNodeID = &value
	}
	return clone
}

func effectiveTree(node domain.ContentNode, states map[string]domain.CommandExecutionState) domain.ContentNode {
	clone := domain.CloneContentNode(node)
	completedBlockText := make(map[string]string, len(states))
	for _, state := range states {
		if state.EntryContentChange != nil {
			completedBlockText[state.EntryContentChange.BlockID] = state.EntryContentChange.CompletedText
		}
	}
	applyEffectiveContentStates(&clone, states, completedBlockText)
	return clone
}

func applyEffectiveContentStates(
	node *domain.ContentNode,
	states map[string]domain.CommandExecutionState,
	completedBlockText map[string]string,
) {
	if node == nil {
		return
	}
	if state, ok := states[node.ID]; ok && node.Type == domain.NodeCommand {
		node.Name = state.CompletedName
		node.Text = state.ResultText
	}
	if node.Type == domain.NodeEntry && len(node.Blocks) != 0 {
		parts := make([]string, len(node.Blocks))
		for index, block := range node.Blocks {
			parts[index] = block.InitialText
			if completedText, ok := completedBlockText[block.ID]; ok {
				parts[index] = completedText
			}
		}
		node.Description = strings.Join(parts, "\n\n")
	}
	for index := range node.Children {
		applyEffectiveContentStates(&node.Children[index], states, completedBlockText)
	}
}

func cloneCommandStates(states map[string]domain.CommandExecutionState) map[string]domain.CommandExecutionState {
	return domain.CloneCommandExecutionStates(states)
}

func cloneCommandExecution(presentation *domain.CommandExecutionPresentation) *domain.CommandExecutionPresentation {
	if presentation == nil {
		return nil
	}
	clone := *presentation
	return &clone
}
