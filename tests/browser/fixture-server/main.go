package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/control"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/live"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/player"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
)

const (
	fixtureAddress           = "127.0.0.1:34119"
	fixtureEdgeUsername      = "players"
	fixtureEdgePassword      = "password-long-enough"
	fixtureRandomSeed        = uint64(0x435254)
	fixtureApprovalRequestID = "approval-request-1"
	fixturePlayerRevision    = uint64(41)
)

type ids struct{ next atomic.Uint64 }

type fixtureRandom struct {
	mu     sync.Mutex
	state  uint64
	forced []int
}

type fixtureCommandStateStore struct {
	mu            sync.Mutex
	states        map[string]domain.CommandExecutionState
	revision      uint64
	executeWrites int
	failNext      bool
}

var fixtureCommandResult = "Доступ в сектор разрешён.\n" +
	"ПРОТОКОЛ ДОСТУПА:\n" +
	strings.Repeat("Состояние гермозатвора подтверждено. Контрольные цепи переведены в штатный режим. "+
		"Маршрут эвакуации свободен. Аварийные блокировки сняты. Диагностический журнал сохранён.\n", 80) +
	"Конец отчёта."

type fixtureAuthoringStore struct {
	mu       sync.Mutex
	session  domain.Session
	revision uint64
}

type fixtureTerminalGroupingStore struct {
	mu                   sync.Mutex
	scenario             string
	persisted            domain.Session
	active               domain.Session
	revision             uint64
	coordinationRevision uint64
	activationHistory    []string
	lastActiveTerminalID string
}

type fixturePlayerManagementStore struct {
	mu        sync.Mutex
	revision  uint64
	nextID    uint64
	roster    []fixturePlayerProfile
	broadcast bool
	failNext  string
}

type fixturePlayerProfile struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Intelligence        int    `json:"intelligence"`
	HackerPerkAvailable bool   `json:"hackerPerkAvailable"`
}

type fixtureAddPlayerRequest struct {
	Name                string `json:"name"`
	Intelligence        int    `json:"intelligence"`
	HackerPerkAvailable *bool  `json:"hackerPerkAvailable"`
	ExpectedRevision    uint64 `json:"expectedRevision"`
}

type fixtureUpdatePlayerRequest struct {
	CharacterID         string `json:"characterId"`
	Name                string `json:"name"`
	Intelligence        int    `json:"intelligence"`
	HackerPerkAvailable *bool  `json:"hackerPerkAvailable"`
	ExpectedRevision    uint64 `json:"expectedRevision"`
}

type fixtureDeletePlayerRequest struct {
	CharacterID      string `json:"characterId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

type fixturePlayerManagementState struct {
	Revision     uint64                 `json:"revision"`
	PlayerConfig map[string]any         `json:"playerConfig"`
	Roster       []fixturePlayerProfile `json:"roster"`
	Sessions     []map[string]any       `json:"sessions"`
	Broadcast    map[string]any         `json:"broadcast"`
}

type fixturePlayerManagementResult struct {
	OK    bool                         `json:"ok"`
	Error string                       `json:"error,omitempty"`
	State fixturePlayerManagementState `json:"state"`
}

func (store *fixturePlayerManagementStore) reset() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.revision = fixturePlayerRevision
	store.nextID = 1
	store.roster = nil
	store.broadcast = false
	store.failNext = ""
}

func (store *fixturePlayerManagementStore) snapshot() fixturePlayerManagementState {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.snapshotLocked()
}

func (store *fixturePlayerManagementStore) failNextSave(message string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failNext = strings.TrimSpace(message)
	if store.failNext == "" {
		store.failNext = "fixture player configuration save failed"
	}
}

func (store *fixturePlayerManagementStore) advanceRevision() fixturePlayerManagementState {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.revision++
	return store.snapshotLocked()
}

func (store *fixturePlayerManagementStore) setBroadcast(active bool) fixturePlayerManagementState {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.broadcast != active {
		store.broadcast = active
		store.revision++
	}
	return store.snapshotLocked()
}

func validateFixturePlayerProfile(name string, intelligence int, hackerPerkAvailable *bool) (string, string) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "", "character name must not be blank"
	}
	if len([]rune(trimmedName)) > 80 {
		return "", "character name must be at most 80 characters"
	}
	if intelligence < 1 || intelligence > 10 {
		return "", "Intelligence must be an integer from 1 to 10"
	}
	if hackerPerkAvailable == nil {
		return "", "Hacker perk availability must be selected"
	}
	return trimmedName, ""
}

func (store *fixturePlayerManagementStore) add(request fixtureAddPlayerRequest) fixturePlayerManagementResult {
	store.mu.Lock()
	defer store.mu.Unlock()

	failure := func(message string) fixturePlayerManagementResult {
		return fixturePlayerManagementResult{Error: message, State: store.snapshotLocked()}
	}
	if store.broadcast {
		return failure("player profiles cannot be changed while a broadcast is active")
	}
	name, validationError := validateFixturePlayerProfile(request.Name, request.Intelligence, request.HackerPerkAvailable)
	if validationError != "" {
		return failure(validationError)
	}
	if request.ExpectedRevision != store.revision {
		return failure("coordination state changed; review the latest player list")
	}
	if store.failNext != "" {
		message := store.failNext
		store.failNext = ""
		return failure(message)
	}

	profile := fixturePlayerProfile{
		ID:                  fmt.Sprintf("fixture-player-%d", store.nextID),
		Name:                name,
		Intelligence:        request.Intelligence,
		HackerPerkAvailable: *request.HackerPerkAvailable,
	}
	store.nextID++
	store.roster = append(store.roster, profile)
	store.revision++
	return fixturePlayerManagementResult{OK: true, State: store.snapshotLocked()}
}

func (store *fixturePlayerManagementStore) update(request fixtureUpdatePlayerRequest) fixturePlayerManagementResult {
	store.mu.Lock()
	defer store.mu.Unlock()

	failure := func(message string) fixturePlayerManagementResult {
		return fixturePlayerManagementResult{Error: message, State: store.snapshotLocked()}
	}
	if store.broadcast {
		return failure("player profiles cannot be changed while a broadcast is active")
	}
	name, validationError := validateFixturePlayerProfile(request.Name, request.Intelligence, request.HackerPerkAvailable)
	if validationError != "" {
		return failure(validationError)
	}
	if request.ExpectedRevision != store.revision {
		return failure("coordination state changed; review the latest player list")
	}
	characterID := strings.TrimSpace(request.CharacterID)
	if characterID == "" {
		return failure("character ID must not be blank")
	}
	index := -1
	for candidateIndex := range store.roster {
		if store.roster[candidateIndex].ID == characterID {
			index = candidateIndex
			break
		}
	}
	if index < 0 {
		return failure("character does not exist")
	}
	current := store.roster[index]
	if current.Name == name && current.Intelligence == request.Intelligence && current.HackerPerkAvailable == *request.HackerPerkAvailable {
		return fixturePlayerManagementResult{OK: true, State: store.snapshotLocked()}
	}
	if store.failNext != "" {
		message := store.failNext
		store.failNext = ""
		return failure(message)
	}
	store.roster[index] = fixturePlayerProfile{
		ID:                  current.ID,
		Name:                name,
		Intelligence:        request.Intelligence,
		HackerPerkAvailable: *request.HackerPerkAvailable,
	}
	store.revision++
	return fixturePlayerManagementResult{OK: true, State: store.snapshotLocked()}
}

func (store *fixturePlayerManagementStore) delete(request fixtureDeletePlayerRequest) fixturePlayerManagementResult {
	store.mu.Lock()
	defer store.mu.Unlock()

	failure := func(message string) fixturePlayerManagementResult {
		return fixturePlayerManagementResult{Error: message, State: store.snapshotLocked()}
	}
	if store.broadcast {
		return failure("player profiles cannot be changed while a broadcast is active")
	}
	if request.ExpectedRevision != store.revision {
		return failure("coordination state changed; review the latest player list")
	}
	characterID := strings.TrimSpace(request.CharacterID)
	if characterID == "" {
		return failure("character ID must not be blank")
	}
	index := -1
	for candidateIndex := range store.roster {
		if store.roster[candidateIndex].ID == characterID {
			index = candidateIndex
			break
		}
	}
	if index < 0 {
		return failure("character does not exist")
	}
	if store.failNext != "" {
		message := store.failNext
		store.failNext = ""
		return failure(message)
	}
	store.roster = append(store.roster[:index:index], store.roster[index+1:]...)
	store.revision++
	return fixturePlayerManagementResult{OK: true, State: store.snapshotLocked()}
}

func (store *fixturePlayerManagementStore) snapshotLocked() fixturePlayerManagementState {
	roster := append([]fixturePlayerProfile(nil), store.roster...)
	if roster == nil {
		roster = []fixturePlayerProfile{}
	}
	var broadcast map[string]any
	if store.broadcast {
		broadcast = map[string]any{"id": "fixture-player-management-broadcast"}
	}
	return fixturePlayerManagementState{
		Revision: store.revision,
		PlayerConfig: map[string]any{
			"status": "loaded", "name": "Player management fixture",
			"filePath": "/private/tmp/fallout-player-management.json", "version": 1,
		},
		Roster: roster, Sessions: []map[string]any{}, Broadcast: broadcast,
	}
}

type fixtureTerminalCatalog struct {
	mu      sync.Mutex
	session domain.Session
}

func (catalog *fixtureTerminalCatalog) replace(session domain.Session) {
	catalog.mu.Lock()
	catalog.session = domain.NormalizeTerminalGroups(domain.CloneSession(session))
	catalog.mu.Unlock()
}

func (catalog *fixtureTerminalCatalog) snapshot() domain.Session {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	return domain.CloneSession(catalog.session)
}

func (catalog *fixtureTerminalCatalog) LookupTerminal(id string) (domain.TerminalTarget, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	terminal := fixtureTerminalByID(&catalog.session, id)
	if terminal == nil {
		return domain.TerminalTarget{}, false
	}
	return fixtureTarget(*terminal), true
}

func (catalog *fixtureTerminalCatalog) LookupTerminalGroup(id string) (domain.TerminalGroupSnapshot, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	return domain.TerminalGroupFor(catalog.session, id)
}

func (catalog *fixtureTerminalCatalog) LookupTerminalTransition(sourceID, commandID string) (domain.TerminalTransitionTarget, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	source := fixtureTerminalByID(&catalog.session, sourceID)
	if source == nil {
		return domain.TerminalTransitionTarget{}, false
	}
	command := fixtureNodeByID(&source.Root, commandID)
	if command == nil || command.TerminalTransition == nil {
		return domain.TerminalTransitionTarget{}, false
	}
	target := fixtureTerminalByID(&catalog.session, command.TerminalTransition.TargetTerminalID)
	if target == nil || target.ID == source.ID {
		return domain.TerminalTransitionTarget{}, false
	}
	sourceGroup, sourceGrouped := domain.TerminalGroupFor(catalog.session, source.ID)
	targetGroup, targetGrouped := domain.TerminalGroupFor(catalog.session, target.ID)
	if !sourceGrouped || !targetGrouped || sourceGroup.ID == "" || sourceGroup.ID != targetGroup.ID {
		return domain.TerminalTransitionTarget{}, false
	}
	return domain.TerminalTransitionTarget{
		SourceTerminalID: source.ID, SourceTerminalName: source.Name,
		CommandID: command.ID, CommandName: command.Name, Target: fixtureTarget(*target),
	}, true
}

func fixtureTarget(terminal domain.Terminal) domain.TerminalTarget {
	clone := domain.CloneSession(domain.Session{Version: 1, Name: "fixture", Terminals: []domain.Terminal{terminal}}).Terminals[0]
	return domain.TerminalTarget{TerminalID: clone.ID, TerminalName: clone.Name, Tree: clone.Root, CommandStates: clone.CommandStates, HackLevel: clone.HackLevel, IntroText: clone.IntroText}
}

func fixtureNodeByID(node *domain.ContentNode, id string) *domain.ContentNode {
	if node == nil {
		return nil
	}
	if node.ID == id {
		return node
	}
	for index := range node.Children {
		if found := fixtureNodeByID(&node.Children[index], id); found != nil {
			return found
		}
	}
	return nil
}

func (store *fixtureTerminalGroupingStore) orderedNavigation() bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.scenario == "ordered-navigation"
}

type fixtureSessionStateResult struct {
	OK       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	Revision uint64          `json:"revision"`
	Session  *domain.Session `json:"session,omitempty"`
}

type fixtureResetCommandRequest struct {
	TerminalID string `json:"terminalId"`
	CommandID  string `json:"commandId"`
}

type fixtureResetTerminalRequest struct {
	TerminalID string `json:"terminalId"`
}

func (store *fixtureAuthoringStore) reset() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.session = stateChangingAuthoringSession()
	store.revision = 1
}

func (store *fixtureAuthoringStore) snapshot() fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.resultLocked()
}

func (store *fixtureAuthoringStore) save(session domain.Session) fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.session = cloneFixtureSession(session)
	store.revision++
	return store.resultLocked()
}

func (store *fixtureAuthoringStore) resetCommand(request fixtureResetCommandRequest) fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	terminal := fixtureTerminalByID(&store.session, strings.TrimSpace(request.TerminalID))
	if terminal == nil {
		return fixtureSessionStateResult{Error: "terminal does not exist", Revision: store.revision}
	}
	commandID := strings.TrimSpace(request.CommandID)
	if _, exists := terminal.CommandStates[commandID]; exists {
		delete(terminal.CommandStates, commandID)
		if len(terminal.CommandStates) == 0 {
			terminal.CommandStates = nil
		}
		store.revision++
	}
	return store.resultLocked()
}

func (store *fixtureAuthoringStore) resetTerminal(request fixtureResetTerminalRequest) fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	terminal := fixtureTerminalByID(&store.session, strings.TrimSpace(request.TerminalID))
	if terminal == nil {
		return fixtureSessionStateResult{Error: "terminal does not exist", Revision: store.revision}
	}
	if len(terminal.CommandStates) != 0 {
		terminal.CommandStates = nil
		store.revision++
	}
	return store.resultLocked()
}

func (store *fixtureAuthoringStore) resultLocked() fixtureSessionStateResult {
	session := cloneFixtureSession(store.session)
	return fixtureSessionStateResult{OK: true, Revision: store.revision, Session: &session}
}

func (store *fixtureTerminalGroupingStore) reset(scenario string) error {
	session, err := terminalGroupingSession(scenario)
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.scenario = strings.TrimSpace(scenario)
	if store.scenario == "" {
		store.scenario = "canonical"
	}
	store.persisted = domain.CloneSession(session)
	store.active = normalizeFixtureTerminalGroups(session)
	store.revision = 1
	store.coordinationRevision = 1
	store.activationHistory = nil
	store.lastActiveTerminalID = ""
	return nil
}

func (store *fixtureTerminalGroupingStore) persistedSnapshot() domain.Session {
	store.mu.Lock()
	defer store.mu.Unlock()
	return domain.CloneSession(store.persisted)
}

func (store *fixtureTerminalGroupingStore) activeSnapshot() fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	session := domain.CloneSession(store.active)
	return fixtureSessionStateResult{OK: true, Revision: store.revision, Session: &session}
}

type fixtureTerminalGroupReplacementRequest struct {
	TerminalGroups               []domain.TerminalGroup `json:"terminalGroups"`
	ExpectedSessionRevision      uint64                 `json:"expectedSessionRevision"`
	ExpectedCoordinationRevision uint64                 `json:"expectedCoordinationRevision"`
}

type fixtureTerminalGroupReplacementResult struct {
	OK                bool                            `json:"ok"`
	Error             string                          `json:"error,omitempty"`
	SessionRevision   uint64                          `json:"sessionRevision"`
	Session           *domain.Session                 `json:"session,omitempty"`
	CoordinationState *domain.MasterCoordinationState `json:"coordinationState"`
}

func (store *fixtureTerminalGroupingStore) replaceGroups(request fixtureTerminalGroupReplacementRequest) fixtureTerminalGroupReplacementResult {
	store.mu.Lock()
	defer store.mu.Unlock()
	canonical := domain.CloneSession(store.active)
	result := fixtureTerminalGroupReplacementResult{
		SessionRevision: store.revision,
		Session:         &canonical,
		CoordinationState: &domain.MasterCoordinationState{
			Revision: store.coordinationRevision,
		},
	}
	if request.ExpectedSessionRevision != store.revision || request.ExpectedCoordinationRevision != store.coordinationRevision {
		result.Error = "СОСТОЯНИЕ ИЗМЕНИЛОСЬ; ПРЕДЛОЖЕНИЕ УСТАРЕЛО, ПРОВЕРЬТЕ ГРУППЫ ЕЩЁ РАЗ"
		return result
	}
	if _, err := domain.ValidateTerminalGroupReplacement(store.active, request.TerminalGroups); err != nil {
		result.Error = err.Error()
		return result
	}
	store.active.TerminalGroups = cloneFixtureGroups(request.TerminalGroups)
	store.persisted = domain.CloneSession(store.active)
	store.revision++
	store.coordinationRevision++
	canonical = domain.CloneSession(store.active)
	result.OK = true
	result.SessionRevision = store.revision
	result.Session = &canonical
	result.CoordinationState = &domain.MasterCoordinationState{Revision: store.coordinationRevision}
	return result
}

func (store *fixtureTerminalGroupingStore) advanceRevisions(session, coordination bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if session {
		store.revision++
	}
	if coordination {
		store.coordinationRevision++
	}
}

func (store *fixtureTerminalGroupingStore) setCoordinationRevision(revision uint64) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.coordinationRevision = revision
}

func (store *fixtureTerminalGroupingStore) runtimeStatus(coordination *domain.MasterCoordinationState) map[string]any {
	store.mu.Lock()
	defer store.mu.Unlock()
	if coordination == nil {
		coordination = &domain.MasterCoordinationState{Revision: store.coordinationRevision}
	}
	return map[string]any{
		"requestedRevision": store.revision,
		"savedRevision":     store.revision,
		"coordinationState": coordination,
	}
}

func (store *fixtureTerminalGroupingStore) navigationState(coordination *domain.MasterCoordinationState) map[string]any {
	store.mu.Lock()
	defer store.mu.Unlock()
	activeTerminalID := ""
	if coordination != nil && coordination.Broadcast != nil && coordination.Broadcast.ActiveTerminalID != nil {
		activeTerminalID = *coordination.Broadcast.ActiveTerminalID
	}
	if activeTerminalID != "" && activeTerminalID != store.lastActiveTerminalID {
		store.activationHistory = append(store.activationHistory, activeTerminalID)
		store.lastActiveTerminalID = activeTerminalID
	}
	var pending *domain.MasterPendingTerminalNavigation
	var pendingCommand *domain.MasterPendingCommandExecution
	if coordination != nil {
		pending = coordination.PendingTerminalNavigation
		pendingCommand = coordination.PendingCommandExecution
	}
	return map[string]any{
		"activeTerminalId":          activeTerminalID,
		"pendingCommandExecution":   pendingCommand,
		"pendingTerminalNavigation": pending,
		"activationHistory":         append([]string(nil), store.activationHistory...),
	}
}

func cloneFixtureGroups(groups []domain.TerminalGroup) []domain.TerminalGroup {
	clone := make([]domain.TerminalGroup, len(groups))
	for index, group := range groups {
		clone[index] = group
		clone[index].TerminalIDs = append([]string(nil), group.TerminalIDs...)
	}
	return clone
}

func (store *fixtureTerminalGroupingStore) save(candidate domain.Session) fixtureSessionStateResult {
	store.mu.Lock()
	defer store.mu.Unlock()

	terminalByID := make(map[string]domain.Terminal, len(candidate.Terminals))
	for _, terminal := range candidate.Terminals {
		terminalByID[terminal.ID] = terminal
	}
	assigned := make(map[string]struct{}, len(candidate.Terminals))
	groups := make([]domain.TerminalGroup, 0, len(store.active.TerminalGroups)+len(candidate.Terminals))
	for _, group := range store.active.TerminalGroups {
		retainedIDs := make([]string, 0, len(group.TerminalIDs))
		for _, terminalID := range group.TerminalIDs {
			if _, exists := terminalByID[terminalID]; !exists {
				continue
			}
			retainedIDs = append(retainedIDs, terminalID)
			assigned[terminalID] = struct{}{}
		}
		if len(retainedIDs) == 0 {
			continue
		}
		group.TerminalIDs = retainedIDs
		groups = append(groups, group)
	}

	usedGroupIDs := make(map[string]struct{}, len(groups))
	usedGroupNames := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		usedGroupIDs[strings.ToLower(strings.TrimSpace(group.ID))] = struct{}{}
		usedGroupNames[strings.ToLower(strings.TrimSpace(group.Name))] = struct{}{}
	}
	for _, terminal := range candidate.Terminals {
		if _, exists := assigned[terminal.ID]; exists {
			continue
		}
		groupID := uniqueFixtureGroupValue("singleton-"+terminal.ID, usedGroupIDs)
		groupName := uniqueFixtureGroupValue(terminal.Name, usedGroupNames)
		groups = append(groups, domain.TerminalGroup{
			ID: groupID, Name: groupName, TerminalIDs: []string{terminal.ID},
		})
	}

	candidate.TerminalGroups = groups
	store.active = domain.CloneSession(candidate)
	store.persisted = domain.CloneSession(candidate)
	store.revision++
	session := domain.CloneSession(store.active)
	return fixtureSessionStateResult{OK: true, Revision: store.revision, Session: &session}
}

func uniqueFixtureGroupValue(base string, used map[string]struct{}) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "terminal-group"
	}
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", base, suffix)
		}
		key := strings.ToLower(candidate)
		if _, exists := used[key]; exists {
			continue
		}
		used[key] = struct{}{}
		return candidate
	}
}

func normalizeFixtureTerminalGroups(session domain.Session) domain.Session {
	if len(session.Terminals) == 0 || len(session.TerminalGroups) != 0 {
		return domain.CloneSession(session)
	}
	normalized := domain.CloneSession(session)
	normalized.TerminalGroups = make([]domain.TerminalGroup, 0, len(normalized.Terminals))
	usedNames := make(map[string]struct{}, len(normalized.Terminals))
	for _, terminal := range normalized.Terminals {
		normalized.TerminalGroups = append(normalized.TerminalGroups, domain.TerminalGroup{
			ID:          "legacy-singleton-" + terminal.ID,
			Name:        uniqueFixtureGroupValue(terminal.Name, usedNames),
			TerminalIDs: []string{terminal.ID},
		})
	}
	return normalized
}

func fixtureTerminalByID(session *domain.Session, terminalID string) *domain.Terminal {
	if session == nil {
		return nil
	}
	for index := range session.Terminals {
		if session.Terminals[index].ID == terminalID {
			return &session.Terminals[index]
		}
	}
	return nil
}

func cloneFixtureSession(session domain.Session) domain.Session {
	data, err := domain.EncodeSession(session)
	if err != nil {
		panic(err)
	}
	clone, err := domain.DecodeSession(data)
	if err != nil {
		panic(err)
	}
	return clone
}

func (store *fixtureCommandStateStore) reset() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.states = nil
	store.revision = 1
	store.executeWrites = 0
	store.failNext = false
}

func (store *fixtureCommandStateStore) failNextSave() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.failNext = true
}

func (store *fixtureCommandStateStore) ExecuteCommandState(_ context.Context, terminalID, commandID string) (control.CommandStateMutation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failNext {
		store.failNext = false
		return control.CommandStateMutation{}, errors.New("fixture atomic persistence failed")
	}
	if terminalID != "terminal-stateful" || commandID != "doors" {
		return control.CommandStateMutation{}, errors.New("fixture command identity is invalid")
	}
	changed := false
	if _, completed := store.states[commandID]; !completed {
		if store.states == nil {
			store.states = make(map[string]domain.CommandExecutionState)
		}
		store.states[commandID] = domain.CommandExecutionState{
			CompletedName: "Двери открыты",
			ResultText:    fixtureCommandResult,
		}
		store.revision++
		store.executeWrites++
		changed = true
	}
	return store.mutationLocked(changed), nil
}

func (store *fixtureCommandStateStore) ResetCommandState(_ context.Context, terminalID, commandID string) (control.CommandStateMutation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if terminalID != "terminal-stateful" || commandID != "doors" {
		return control.CommandStateMutation{}, errors.New("fixture command identity is invalid")
	}
	_, changed := store.states[commandID]
	if changed {
		delete(store.states, commandID)
		store.revision++
	}
	return store.mutationLocked(changed), nil
}

func (store *fixtureCommandStateStore) ResetTerminalCommandStates(_ context.Context, terminalID string) (control.CommandStateMutation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if terminalID != "terminal-stateful" {
		return control.CommandStateMutation{}, errors.New("fixture terminal identity is invalid")
	}
	changed := len(store.states) != 0
	if changed {
		store.states = nil
		store.revision++
	}
	return store.mutationLocked(changed), nil
}

func (store *fixtureCommandStateStore) mutationLocked(changed bool) control.CommandStateMutation {
	return control.CommandStateMutation{
		Changed:  changed,
		Revision: store.revision,
		Session:  stateChangingApprovalSession(store.states),
	}
}

func (store *fixtureCommandStateStore) target() domain.TerminalTarget {
	store.mu.Lock()
	defer store.mu.Unlock()
	target := stateChangingApprovalTarget()
	target.CommandStates = cloneFixtureCommandStates(store.states)
	return target
}

func (store *fixtureCommandStateStore) syncTarget() domain.TerminalTarget {
	store.mu.Lock()
	defer store.mu.Unlock()
	target := stateChangingSyncTarget()
	target.CommandStates = cloneFixtureCommandStates(store.states)
	return target
}

func (store *fixtureCommandStateStore) syncSession() domain.Session {
	store.mu.Lock()
	defer store.mu.Unlock()
	return stateChangingSyncSession(store.states)
}

func (store *fixtureCommandStateStore) audit() (int, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	_, completed := store.states["doors"]
	return store.executeWrites, completed
}

func (random *fixtureRandom) Intn(limit int) int {
	if limit <= 1 {
		return 0
	}
	random.mu.Lock()
	defer random.mu.Unlock()
	if len(random.forced) != 0 {
		value := random.forced[0]
		random.forced = random.forced[1:]
		if value < 0 {
			return limit - 1
		}
		return value % limit
	}
	random.state = random.state*6364136223846793005 + 1442695040888963407
	return int(random.state % uint64(limit))
}

func (random *fixtureRandom) forceDudRemoval(position string) bool {
	random.mu.Lock()
	defer random.mu.Unlock()
	switch position {
	case "revealed":
		random.forced = []int{0, 0}
	case "pending":
		random.forced = []int{0, -1}
	default:
		return false
	}
	return true
}

func (random *fixtureRandom) reset() {
	random.mu.Lock()
	defer random.mu.Unlock()
	random.state = fixtureRandomSeed
	random.forced = nil
}

type fixtureEdge struct {
	active           atomic.Bool
	publicGeneration atomic.Uint64
	service          *control.Service
	connect          *player.ConnectService
	hub              *player.SubscriptionHub
	ingress          tunnel.PublicIngress
	publicURL        string
}

type fixturePresentationGateState struct {
	release     chan struct{}
	blocked     chan struct{}
	unary       bool
	canceled    bool
	releaseOnce sync.Once
	blockedOnce sync.Once
}

// fixturePresentationGate makes presentation races deterministic: it pauses
// exactly one streamed or unary presentation before canonical dispatch.
type fixturePresentationGate struct {
	*control.Service
	context context.Context
	mu      sync.Mutex
	state   *fixturePresentationGateState
}

func (gate *fixturePresentationGate) arm() {
	gate.armTransport(false)
}

func (gate *fixturePresentationGate) armUnary() {
	gate.armTransport(true)
}

func (gate *fixturePresentationGate) armTransport(unary bool) {
	gate.cancel()
	gate.mu.Lock()
	gate.state = &fixturePresentationGateState{
		release: make(chan struct{}),
		blocked: make(chan struct{}),
		unary:   unary,
	}
	gate.mu.Unlock()
}

func (gate *fixturePresentationGate) cancel() {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.state == nil {
		return
	}
	gate.state.canceled = true
	gate.state.releaseOnce.Do(func() { close(gate.state.release) })
}

func (gate *fixturePresentationGate) release() {
	gate.mu.Lock()
	state := gate.state
	gate.mu.Unlock()
	if state == nil {
		return
	}
	state.releaseOnce.Do(func() { close(state.release) })
}

func (gate *fixturePresentationGate) cancelAfter(delay time.Duration) {
	gate.mu.Lock()
	if gate.state == nil {
		gate.mu.Unlock()
		return
	}
	state := gate.state
	state.canceled = true
	gate.mu.Unlock()
	time.AfterFunc(delay, func() {
		state.releaseOnce.Do(func() { close(state.release) })
	})
}

func (gate *fixturePresentationGate) isBlocked() bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.state == nil {
		return false
	}
	select {
	case <-gate.state.blocked:
		return true
	default:
		return false
	}
}

func (gate *fixturePresentationGate) reset() {
	gate.cancel()
	gate.mu.Lock()
	gate.state = nil
	gate.mu.Unlock()
}

func (gate *fixturePresentationGate) DispatchPlayerAction(connectionID domain.ConnectionID, command domain.RuntimeCommand) domain.ActionResult {
	return gate.dispatchPresentation(command, false, func() domain.ActionResult {
		return gate.Service.DispatchPlayerAction(connectionID, command)
	})
}

func (gate *fixturePresentationGate) DispatchPlayerActionForRecognition(handle domain.RecognitionHandle, command domain.RuntimeCommand) domain.ActionResult {
	return gate.dispatchPresentation(command, true, func() domain.ActionResult {
		return gate.Service.DispatchPlayerActionForRecognition(handle, command)
	})
}

func (gate *fixturePresentationGate) dispatchPresentation(command domain.RuntimeCommand, unary bool, dispatch func() domain.ActionResult) domain.ActionResult {
	gate.mu.Lock()
	state := gate.state
	gate.mu.Unlock()
	if command.Kind != domain.RuntimeCommandPresentation || state == nil || state.unary != unary {
		return dispatch()
	}

	state.blockedOnce.Do(func() { close(state.blocked) })
	contextClosed := false
	select {
	case <-state.release:
	case <-gate.context.Done():
		contextClosed = true
	}
	gate.mu.Lock()
	canceled := state.canceled || contextClosed
	if gate.state == state {
		gate.state = nil
	}
	gate.mu.Unlock()
	if canceled {
		return domain.ActionResult{
			RequestID: command.RequestID,
			Reason:    domain.ActionReasonControllerDisconnected,
			Revision:  gate.Service.Snapshot().Revision,
		}
	}
	return dispatch()
}

type fixtureEdgeStatus struct {
	AuthBoundary           string `json:"authBoundary"`
	Upstream               string `json:"upstream"`
	Active                 bool   `json:"active"`
	AuthorizationForwarded bool   `json:"authorizationForwarded"`
	PublicURL              string `json:"publicUrl"`
}

func (source *ids) Next() string {
	return fmt.Sprintf("browser-fixture-%d", source.next.Add(1))
}

func (edge *fixtureEdge) reset() error {
	if edge.ingress == nil || edge.ingress.URL() == nil {
		return errors.New("fixture ingress is unavailable")
	}
	host := edge.ingress.URL().Host
	if publicURL, err := url.Parse(edge.publicURL); err == nil && publicURL.Host != "" {
		host = publicURL.Host
	}
	if err := edge.ingress.Activate(host, fixtureEdgeUsername, []byte(fixtureEdgePassword)); err != nil {
		return err
	}
	edge.active.Store(true)
	edge.publicGeneration.Store(0)
	return nil
}

type publicAccessFixtureSnapshot struct {
	Preferences            publicAccessFixturePreferences `json:"preferences"`
	ProviderTokenPresence  string                         `json:"providerTokenPresence"`
	PlayerPasswordPresence string                         `json:"playerPasswordPresence"`
	Status                 publicAccessFixtureStatus      `json:"status"`
}

type publicAccessFixturePreferences struct {
	Version  int    `json:"version"`
	Username string `json:"username"`
	Revision uint64 `json:"revision"`
}

type publicAccessFixtureStatus struct {
	State            string `json:"state"`
	Generation       uint64 `json:"generation"`
	SettingsRevision uint64 `json:"settingsRevision"`
	PublicURL        string `json:"publicUrl,omitempty"`
	ErrorCategory    string `json:"errorCategory,omitempty"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
}

func (edge *fixtureEdge) publicFailure(kind string) (publicAccessFixtureSnapshot, bool) {
	failures := map[string][2]string{
		"invalid-token":        {"provider_authentication", "The provider rejected the account credential."},
		"revoked-token":        {"provider_authentication", "The provider rejected the account credential."},
		"no-network":           {"network_unavailable", "The network is unavailable; local access remains available."},
		"dns-timeout":          {"timeout", "Public access did not become ready in time."},
		"domain-conflict":      {"domain_unavailable", "The reserved domain is unavailable for this account."},
		"keychain-locked":      {"secret_store_locked", "Unlock Keychain and try again."},
		"keychain-denied":      {"secret_store_denied", "Allow Keychain access and try again."},
		"keychain-unavailable": {"secret_store_unavailable", "Keychain is unavailable; local access remains available."},
		"policy-failure":       {"provider_failure", "The public-access provider stopped unexpectedly."},
		"provider-failure":     {"provider_failure", "The public-access provider stopped unexpectedly."},
		"unexpected-done":      {"provider_failure", "The public-access provider stopped unexpectedly."},
		"close-failure":        {"provider_failure", "The public-access provider stopped unexpectedly."},
	}
	if kind == "stale-completion" {
		generation := edge.publicGeneration.Load()
		if generation > 0 {
			generation--
		}
		return edge.publicSnapshot(publicAccessFixtureStatus{
			State: "ready", Generation: generation, PublicURL: "https://stale.invalid",
		}), true
	}
	failure, ok := failures[kind]
	if !ok {
		return publicAccessFixtureSnapshot{}, false
	}
	return edge.publicSnapshot(publicAccessFixtureStatus{
		State: "error", Generation: edge.publicGeneration.Add(1),
		ErrorCategory: failure[0], ErrorMessage: failure[1],
	}), true
}

func (edge *fixtureEdge) publicRecovery() publicAccessFixtureSnapshot {
	return edge.publicSnapshot(publicAccessFixtureStatus{
		State: "ready", Generation: edge.publicGeneration.Add(1), PublicURL: "https://recovered.example",
	})
}

func (edge *fixtureEdge) publicSnapshot(status publicAccessFixtureStatus) publicAccessFixtureSnapshot {
	status.SettingsRevision = 0
	return publicAccessFixtureSnapshot{
		Preferences:           publicAccessFixturePreferences{Version: 1, Username: "players", Revision: 0},
		ProviderTokenPresence: "present", PlayerPasswordPresence: "present", Status: status,
	}
}

func (edge *fixtureEdge) update(response http.ResponseWriter) {
	updated := fixtureTerminal()
	updated.Tree.Children[0].Children = append(updated.Tree.Children[0].Children, domain.ContentNode{
		ID: "public-update", Type: domain.NodeEntry, Name: "PUBLIC UPDATE", Description: "STREAM UPDATE RECEIVED",
	})
	if _, err := edge.service.UpdateLiveTerminal(updated.Tree, &updated.IntroText); err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (edge *fixtureEdge) activateHacking(response http.ResponseWriter) {
	target := fixtureTerminal()
	target.TerminalID = "terminal-hacking"
	target.TerminalName = "Security"
	target.HackLevel = 1
	if _, err := edge.service.RequestTerminalActivation(target); err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func activateCRTTerminal(service *control.Service, response http.ResponseWriter, target domain.TerminalTarget) bool {
	state, err := service.RequestTerminalActivation(target)
	if err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return false
	}
	if state.PendingSwitch == nil {
		return true
	}
	if _, err = service.ResolveTerminalSwitch(state.PendingSwitch.SwitchID, domain.TerminalSwitchDiscard); err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return false
	}
	return true
}

func activateLifecycleTerminal(service *control.Service, target domain.TerminalTarget) error {
	state, err := service.RequestTerminalActivation(target)
	if err != nil {
		return err
	}
	if state.PendingSwitch == nil {
		return nil
	}
	_, err = service.ResolveTerminalSwitch(state.PendingSwitch.SwitchID, domain.TerminalSwitchDiscard)
	return err
}

func activatePreservingTerminal(service *control.Service, target domain.TerminalTarget) error {
	state, err := service.RequestTerminalActivation(target)
	if err != nil {
		return err
	}
	if state.PendingSwitch == nil {
		return nil
	}
	_, err = service.ResolveTerminalSwitch(state.PendingSwitch.SwitchID, domain.TerminalSwitchPreserve)
	return err
}

func restartStateChangingBroadcast(service *control.Service, target domain.TerminalTarget) error {
	if _, err := service.EndBroadcast(); err != nil {
		return err
	}
	if _, err := service.StartBroadcast(); err != nil {
		return err
	}
	return activateLifecycleTerminal(service, target)
}

func main() {
	rootContext := context.Background()
	ctx, stop := signal.NotifyContext(rootContext, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("fixture context is required")
	}
	playerAssets, err := fs.Sub(os.DirFS("../../frontend/client"), "dist")
	if err != nil {
		return fmt.Errorf("open built player assets: %w", err)
	}
	fixtureHackRandom := &fixtureRandom{state: fixtureRandomSeed}
	approvalStore := &fixtureCommandStateStore{}
	approvalStore.reset()
	authoringStore := &fixtureAuthoringStore{}
	authoringStore.reset()
	terminalGroupingStore := &fixtureTerminalGroupingStore{}
	if err := terminalGroupingStore.reset("canonical"); err != nil {
		return fmt.Errorf("reset terminal-grouping fixture: %w", err)
	}
	playerManagementStore := &fixturePlayerManagementStore{}
	playerManagementStore.reset()
	navigationCatalog := &fixtureTerminalCatalog{}
	navigationCatalog.replace(terminalNavigationSession())
	var navigationPending atomic.Bool
	var navigationProjectionRevision atomic.Uint64
	liveService := live.New(fixtureHackRandom, nil)
	var connectPlayer *player.ConnectService
	service := control.New(control.Config{
		IDs:               &ids{},
		Runtime:           liveService,
		Terminals:         liveService,
		TrustedHack:       liveService,
		CommandStateStore: approvalStore,
		TerminalCatalog:   navigationCatalog,
		Enqueue: func(effect control.Effect) {
			if connectPlayer != nil {
				connectPlayer.PublishEffect(effect)
			}
		},
	})
	presentationGate := &fixturePresentationGate{Service: service, context: ctx}
	presentationHub := player.NewSubscriptionHub()
	connectPlayer, err = player.NewConnectService(player.ConnectServiceConfig{
		Coordinator: presentationGate,
		Hub:         presentationHub,
		Assets:      playerAssets,
	})
	if err != nil {
		return fmt.Errorf("construct fixture Connect service: %w", err)
	}
	rpcPath, rpcHandler := player.NewConnectHandler(connectPlayer)
	applicationHandler := player.NewApplicationHandler(playerAssets, rpcPath, rpcHandler)
	edge := &fixtureEdge{service: service, connect: connectPlayer, hub: presentationHub}

	for _, name := range []string{"Mara", "Boone", "Arcade", "Cass", "Veronica", "Raul", "Lily"} {
		if _, err := service.AddCharacter(domain.CharacterCreatePayload{
			Name: name, Intelligence: 1, ExpectedRevision: service.Snapshot().Revision,
		}); err != nil {
			return err
		}
	}
	if _, err := service.StartBroadcast(); err != nil {
		return err
	}
	if _, err := service.RequestTerminalActivation(fixtureTerminal()); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /__fixture/desktop-api", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response, `<!doctype html><meta charset="utf-8">
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>
<script type="module" src="/__fixture/desktop-api.js"></script>`)
	})
	mux.HandleFunc("GET /__fixture/player-management", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.Replace(page, `class="start-screen" id="startScreen"`, `class="start-screen" id="startScreen" style="display:none"`, 1)
		page = strings.Replace(page, `id="mainLayout" style="display:none"`, `id="mainLayout" style="display:flex"`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("POST /__fixture/player-management/reset", func(response http.ResponseWriter, _ *http.Request) {
		playerManagementStore.reset()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/player-management/state", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.snapshot())
	})
	mux.HandleFunc("POST /__fixture/player-management/add", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload fixtureAddPlayerRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid player profile", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.add(payload))
	})
	mux.HandleFunc("POST /__fixture/player-management/update", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload fixtureUpdatePlayerRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid player profile", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.update(payload))
	})
	mux.HandleFunc("POST /__fixture/player-management/delete", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload fixtureDeletePlayerRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid player deletion", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.delete(payload))
	})
	mux.HandleFunc("POST /__fixture/player-management/set-broadcast", func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload struct {
			Active *bool `json:"active"`
		}
		if err := decoder.Decode(&payload); err != nil || payload.Active == nil {
			http.Error(response, "invalid broadcast state", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.setBroadcast(*payload.Active))
	})
	mux.HandleFunc("POST /__fixture/player-management/fail-next-save", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(request.Body).Decode(&payload)
		playerManagementStore.failNextSave(payload.Error)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/player-management/advance-revision", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(playerManagementStore.advanceRevision())
	})
	mux.HandleFunc("GET /__fixture/public-access-settings", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := string(raw)
		page = strings.Replace(page, `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.Replace(page, `class="start-screen" id="startScreen"`, `class="start-screen" id="startScreen" style="display:none"`, 1)
		page = strings.Replace(page, `id="mainLayout" style="display:none"`, `id="mainLayout" style="display:flex"`, 1)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-authoring", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-authoring/reset", func(response http.ResponseWriter, _ *http.Request) {
		authoringStore.reset()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-authoring/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(authoringStore.snapshot())
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-authoring/save", func(response http.ResponseWriter, request *http.Request) {
		var session domain.Session
		if err := json.NewDecoder(request.Body).Decode(&session); err != nil {
			http.Error(response, "invalid authoring session", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(authoringStore.save(session))
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-authoring/reset-command", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureResetCommandRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid reset-command request", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(authoringStore.resetCommand(payload))
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-authoring/reset-terminal", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureResetTerminalRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid reset-terminal request", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(authoringStore.resetTerminal(payload))
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/terminal-grouping/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/desktop-bindings.js", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = fmt.Fprint(response, `export * from "/__fixture/desktop-bindings.js";

export async function OpenSession() {
  const response = await fetch("/__fixture/terminal-grouping/open-session");
  const result = response.ok ? await response.json() : null;
  return {
    ok: response.ok && result?.ok === true,
    error: response.ok ? (result?.error ?? "") : "terminal grouping fixture is unavailable",
    filePath: "/private/tmp/fallout-terminal-grouping.json",
    session: result?.session ?? null,
  };
}

export async function SaveSession(session) {
  const retained = structuredClone(session);
  const response = await fetch("/__fixture/terminal-grouping/save", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(retained),
  });
  const result = response.ok ? await response.json() : null;
  return {
    ok: response.ok && result?.ok === true,
    error: response.ok ? (result?.error ?? "") : "terminal grouping save failed",
    savedRevision: Number(result?.revision ?? 0),
  };
}

export async function RequestTerminalActivation(payload) {
  const response = await fetch("/__fixture/terminal-grouping/activate-terminal", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  return response.ok
    ? response.json()
    : { ok: false, error: "terminal grouping activation fixture is unavailable" };
}
`)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/reset", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Scenario string `json:"scenario"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid terminal-grouping reset", http.StatusBadRequest)
			return
		}
		if err := terminalGroupingStore.reset(payload.Scenario); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		active := terminalGroupingStore.activeSnapshot().Session
		if active == nil || len(active.Terminals) == 0 {
			http.Error(response, "terminal-grouping scenario has no terminal", http.StatusInternalServerError)
			return
		}
		navigationCatalog.replace(*active)
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		initialTerminalID := active.Terminals[0].ID
		if strings.TrimSpace(payload.Scenario) == "ordered-navigation" {
			initialTerminalID = "gamma"
		}
		initial, ok := navigationCatalog.LookupTerminal(initialTerminalID)
		if !ok {
			http.Error(response, "terminal-grouping initial terminal is unavailable", http.StatusInternalServerError)
			return
		}
		if _, err := service.RequestTerminalActivation(initial); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		terminalGroupingStore.setCoordinationRevision(service.Snapshot().Revision)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.persistedSnapshot())
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/open-session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.activeSnapshot())
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/save", func(response http.ResponseWriter, request *http.Request) {
		var candidate domain.Session
		if err := json.NewDecoder(request.Body).Decode(&candidate); err != nil {
			http.Error(response, "invalid terminal-grouping session", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.save(candidate))
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/status", func(response http.ResponseWriter, _ *http.Request) {
		var coordination *domain.MasterCoordinationState
		if terminalGroupingStore.orderedNavigation() {
			coordination = service.Snapshot()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.runtimeStatus(coordination))
	})
	mux.HandleFunc("GET /__fixture/terminal-grouping/navigation-state", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(terminalGroupingStore.navigationState(service.Snapshot()))
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/activate-terminal", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			TerminalID string `json:"terminalId"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid terminal grouping activation", http.StatusBadRequest)
			return
		}
		target, ok := navigationCatalog.LookupTerminal(payload.TerminalID)
		if !ok {
			http.Error(response, "terminal grouping activation target is unavailable", http.StatusNotFound)
			return
		}
		state, err := service.RequestTerminalActivation(target)
		result := map[string]any{"ok": err == nil, "status": "activated", "state": state}
		if err != nil {
			result["error"] = err.Error()
			result["status"] = ""
		} else if state.PendingSwitch != nil {
			result["status"] = "decision-required"
			result["switchId"] = state.PendingSwitch.SwitchID
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/resolve-navigation", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string                            `json:"requestId"`
			Decision  domain.TerminalNavigationDecision `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid terminal grouping navigation decision", http.StatusBadRequest)
			return
		}
		state, err := service.ResolveTerminalNavigation(payload.RequestID, payload.Decision)
		result := map[string]any{"ok": err == nil, "state": state}
		if err != nil {
			result["error"] = err.Error()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/resolve-command", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string                          `json:"requestId"`
			Decision  domain.CommandExecutionDecision `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid terminal grouping command decision", http.StatusBadRequest)
			return
		}
		state, _, err := service.ResolveCommandExecution(request.Context(), payload.RequestID, payload.Decision)
		result := map[string]any{"ok": err == nil, "state": state}
		if err != nil {
			result["error"] = err.Error()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/replace-groups", func(response http.ResponseWriter, request *http.Request) {
		var payload fixtureTerminalGroupReplacementRequest
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(response, "invalid terminal group replacement", http.StatusBadRequest)
			return
		}
		result := terminalGroupingStore.replaceGroups(payload)
		if result.OK && result.Session != nil {
			navigationCatalog.replace(*result.Session)
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/terminal-grouping/advance-revisions", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Session      bool `json:"session"`
			Coordination bool `json:"coordination"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid revision advance", http.StatusBadRequest)
			return
		}
		terminalGroupingStore.advanceRevisions(payload.Session, payload.Coordination)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/terminal-navigation/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/terminal-navigation/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(navigationCatalog.snapshot())
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/save", func(response http.ResponseWriter, request *http.Request) {
		var candidate domain.Session
		if err := json.NewDecoder(request.Body).Decode(&candidate); err != nil {
			http.Error(response, "invalid terminal navigation session", http.StatusBadRequest)
			return
		}
		candidate.TerminalGroups = navigationCatalog.snapshot().TerminalGroups
		candidate = domain.EnsureTerminalGroups(candidate)
		if err := domain.ValidateSession(candidate); err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		navigationCatalog.replace(candidate)
		navigationProjectionRevision.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"ok": true, "revision": navigationProjectionRevision.Load()})
	})
	mux.HandleFunc("GET /__fixture/terminal-navigation/state", func(response http.ResponseWriter, _ *http.Request) {
		state := service.Snapshot()
		state.Revision += navigationProjectionRevision.Load()
		if navigationPending.Load() {
			state.PendingTerminalNavigation = &domain.MasterPendingTerminalNavigation{
				RequestID: "navigation-forward-1", BroadcastID: state.Broadcast.ID,
				Direction:        domain.TerminalNavigationForward,
				SourceTerminalID: "residential", SourceTerminalName: "Жилой терминал",
				CommandID: "go-security", CommandName: "ПЕРЕЙТИ В ОХРАНУ",
				TargetTerminalID: "security", TargetTerminalName: "Терминал охраны",
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(state)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/reset", func(response http.ResponseWriter, _ *http.Request) {
		fixtureHackRandom.reset()
		navigationCatalog.replace(terminalNavigationSession())
		navigationPending.Store(false)
		navigationProjectionRevision.Store(0)
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		source, _ := navigationCatalog.LookupTerminal("residential")
		if _, err := service.RequestTerminalActivation(source); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/group-full-route", func(response http.ResponseWriter, _ *http.Request) {
		session := navigationCatalog.snapshot()
		session.TerminalGroups = []domain.TerminalGroup{{
			ID: "navigation-full-route", Name: "Полный навигационный маршрут",
			TerminalIDs: []string{"residential", "security", "vault"},
		}}
		navigationCatalog.replace(session)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/pending-forward", func(response http.ResponseWriter, _ *http.Request) {
		navigationPending.Store(true)
		navigationProjectionRevision.Add(1)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/resolve", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string `json:"requestId"`
			Decision  string `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || (payload.Decision != "approve" && payload.Decision != "reject") {
			http.Error(response, "invalid terminal navigation decision", http.StatusBadRequest)
			return
		}
		var state *domain.MasterCoordinationState
		if navigationPending.Load() && payload.RequestID == "navigation-forward-1" {
			navigationPending.Store(false)
			navigationProjectionRevision.Add(1)
			state = service.Snapshot()
		} else if pending := service.Snapshot().PendingCommandExecution; pending != nil && pending.RequestID == payload.RequestID {
			resolved, _, resolveErr := service.ResolveCommandExecution(request.Context(), payload.RequestID, domain.CommandExecutionDecision(payload.Decision))
			if resolveErr != nil {
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]any{"ok": false, "error": resolveErr.Error(), "state": resolved})
				return
			}
			state = resolved
		} else {
			decision := domain.TerminalNavigationReject
			if payload.Decision == "approve" {
				decision = domain.TerminalNavigationApprove
			}
			resolved, resolveErr := service.ResolveTerminalNavigation(payload.RequestID, decision)
			if resolveErr != nil {
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]any{"ok": false, "error": resolveErr.Error(), "state": resolved})
				return
			}
			state = resolved
		}
		state.Revision += navigationProjectionRevision.Load()
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{"ok": true, "state": state})
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/switch-source", func(response http.ResponseWriter, _ *http.Request) {
		target, _ := navigationCatalog.LookupTerminal("residential")
		if err := activatePreservingTerminal(service, target); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/switch-target", func(response http.ResponseWriter, _ *http.Request) {
		target, _ := navigationCatalog.LookupTerminal("security")
		if err := activatePreservingTerminal(service, target); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/force-hack", func(response http.ResponseWriter, _ *http.Request) {
		if _, ok := service.ForceHackSuccess(); !ok {
			http.Error(response, "active terminal has no unfinished hack", http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/move-source-folder", func(response http.ResponseWriter, _ *http.Request) {
		session := navigationCatalog.snapshot()
		source := fixtureTerminalByID(&session, "residential")
		if source == nil {
			http.Error(response, "source terminal is unavailable", http.StatusConflict)
			return
		}
		var navigation domain.ContentNode
		for index := range source.Root.Children {
			if source.Root.Children[index].ID == "navigation" {
				navigation = source.Root.Children[index]
				source.Root.Children = append(source.Root.Children[:index], source.Root.Children[index+1:]...)
				break
			}
		}
		source.Root.Children = append(source.Root.Children, domain.ContentNode{
			ID: "archive", Type: domain.NodeFolder, Name: "АРХИВ", Children: []domain.ContentNode{navigation},
		})
		navigationCatalog.replace(session)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/delete-source-folder", func(response http.ResponseWriter, _ *http.Request) {
		session := navigationCatalog.snapshot()
		source := fixtureTerminalByID(&session, "residential")
		if source == nil {
			http.Error(response, "source terminal is unavailable", http.StatusConflict)
			return
		}
		for index := range source.Root.Children {
			if source.Root.Children[index].ID == "navigation" {
				source.Root.Children = append(source.Root.Children[:index], source.Root.Children[index+1:]...)
				break
			}
		}
		navigationCatalog.replace(session)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/remove-target", func(response http.ResponseWriter, _ *http.Request) {
		session := navigationCatalog.snapshot()
		for index := range session.Terminals {
			if session.Terminals[index].ID == "security" {
				session.Terminals = append(session.Terminals[:index], session.Terminals[index+1:]...)
				break
			}
		}
		navigationCatalog.replace(session)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/terminal-navigation/new-broadcast", func(response http.ResponseWriter, _ *http.Request) {
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		source, _ := navigationCatalog.LookupTerminal("residential")
		if _, err := service.RequestTerminalActivation(source); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-approval/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-approval/state", func(response http.ResponseWriter, _ *http.Request) {
		state := service.Snapshot()
		if state.PendingCommandExecution != nil {
			state.PendingCommandExecution.RequestID = fixtureApprovalRequestID
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(state)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-approval/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(stateChangingApprovalSession(nil))
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-approval/audit", func(response http.ResponseWriter, _ *http.Request) {
		executeWrites, completed := approvalStore.audit()
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"executeWrites": executeWrites,
			"completed":     completed,
		})
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-approval/reset", func(response http.ResponseWriter, _ *http.Request) {
		approvalStore.reset()
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := service.RequestTerminalActivation(approvalStore.target()); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-approval/reemit-pending", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-approval/fail-next-save", func(response http.ResponseWriter, _ *http.Request) {
		approvalStore.failNextSave()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-approval/resolve", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string `json:"requestId"`
			Decision  string `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid fixture command decision", http.StatusBadRequest)
			return
		}
		requestID := payload.RequestID
		if requestID == fixtureApprovalRequestID {
			if current := service.Snapshot().PendingCommandExecution; current != nil {
				requestID = current.RequestID
			}
		}
		state, _, resolveErr := service.ResolveCommandExecution(request.Context(), requestID, domain.CommandExecutionDecision(payload.Decision))
		result := map[string]any{"ok": resolveErr == nil, "state": state}
		if resolveErr != nil {
			result["error"] = resolveErr.Error()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-sync/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
		page = strings.ReplaceAll(page, `./overseer.css`, `/__fixture/overseer.css`)
		page = strings.ReplaceAll(page, `./overseer.js`, `/__fixture/overseer.js`)
		page = strings.ReplaceAll(page, `./desktop-api.js`, `/__fixture/desktop-api.js`)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte(page))
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-sync/state", func(response http.ResponseWriter, _ *http.Request) {
		state := service.Snapshot()
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(state)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-sync/session", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(approvalStore.syncSession())
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/reset", func(response http.ResponseWriter, _ *http.Request) {
		approvalStore.reset()
		if err := restartStateChangingBroadcast(service, approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/resolve", func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			RequestID string `json:"requestId"`
			Decision  string `json:"decision"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(response, "invalid fixture command decision", http.StatusBadRequest)
			return
		}
		requestID := payload.RequestID
		if requestID == fixtureApprovalRequestID {
			if current := service.Snapshot().PendingCommandExecution; current != nil {
				requestID = current.RequestID
			}
		}
		state, _, resolveErr := service.ResolveCommandExecution(request.Context(), requestID, domain.CommandExecutionDecision(payload.Decision))
		result := map[string]any{"ok": resolveErr == nil, "state": state}
		if resolveErr != nil {
			result["error"] = resolveErr.Error()
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(result)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/switch-away", func(response http.ResponseWriter, _ *http.Request) {
		if err := activateLifecycleTerminal(service, stateChangingSyncReserveTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/switch-back", func(response http.ResponseWriter, _ *http.Request) {
		if err := activateLifecycleTerminal(service, approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/restart-broadcast", func(response http.ResponseWriter, _ *http.Request) {
		if err := restartStateChangingBroadcast(service, approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/state-changing-command-sync/reopen-session", func(response http.ResponseWriter, _ *http.Request) {
		if err := restartStateChangingBroadcast(service, approvalStore.syncTarget()); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/state-changing-command-sync/audit", func(response http.ResponseWriter, _ *http.Request) {
		executeWrites, completed := approvalStore.audit()
		pendingRequests := 0
		if service.Snapshot().PendingCommandExecution != nil {
			pendingRequests = 1
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"executeWrites":   executeWrites,
			"pendingRequests": pendingRequests,
			"completed":       completed,
		})
	})
	mux.HandleFunc("GET /__fixture/overseer.css", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "../../frontend/overseer/src/overseer.css")
	})
	mux.HandleFunc("GET /__fixture/overseer.js", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "../../frontend/overseer/src/overseer.js")
	})
	mux.HandleFunc("GET /__fixture/desktop-api.js", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "../../frontend/overseer/src/desktop-api.js")
	})
	mux.HandleFunc("GET /__fixture/desktop-bindings.js", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "fixtures/desktop-bindings.js")
	})
	mux.HandleFunc("POST /__fixture/reset", func(response http.ResponseWriter, _ *http.Request) {
		fixtureHackRandom.reset()
		presentationGate.reset()
		if err := edge.reset(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := service.EndBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if _, err := service.StartBroadcast(); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := service.RequestTerminalActivation(fixtureTerminal()); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/reassign-controller", func(response http.ResponseWriter, _ *http.Request) {
		state := service.Snapshot()
		var target domain.LogicalSessionID
		for _, session := range state.Sessions {
			if session.Connected && session.Character != nil && session.Role == domain.PlayerRoleObserver {
				target = session.ID
				break
			}
		}
		if target == "" {
			http.Error(response, "connected assigned observer does not exist", http.StatusConflict)
			return
		}
		updated, err := service.SetActiveController(target)
		if err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"controllerSessionId": target,
			"revision":            updated.Revision,
		})
	})
	mux.HandleFunc("POST /__fixture/update", func(response http.ResponseWriter, _ *http.Request) {
		updated := fixtureTerminal()
		updated.Tree.Children = append(updated.Tree.Children, domain.ContentNode{
			ID: "public-update", Type: domain.NodeEntry, Name: "PUBLIC UPDATE", Description: "STREAM UPDATE RECEIVED",
		})
		if _, err := service.UpdateLiveTerminal(updated.Tree, &updated.IntroText); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/force-hack", func(response http.ResponseWriter, _ *http.Request) {
		if _, ok := service.ForceHackSuccess(); !ok {
			http.Error(response, "active terminal has no unfinished hack", http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/local/hacking", func(response http.ResponseWriter, _ *http.Request) {
		edge.activateHacking(response)
	})
	mux.HandleFunc("POST /__fixture/local/crt/approve-command", func(response http.ResponseWriter, request *http.Request) {
		pending := service.Snapshot().PendingCommandExecution
		if pending == nil {
			http.Error(response, "CRT command approval is not pending", http.StatusConflict)
			return
		}
		if _, _, err := service.ResolveCommandExecution(request.Context(), pending.RequestID, domain.CommandExecutionApprove); err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/local/crt/{state}", func(response http.ResponseWriter, request *http.Request) {
		state := request.PathValue("state")
		switch state {
		case "content":
			if _, err := service.RequestTerminalActivation(crtFixtureTerminal()); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "unchanged":
			target := crtFixtureTerminal()
			if _, err := service.UpdateLiveTerminal(target.Tree, &target.IntroText); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "replacement":
			target := crtReplacementTerminal()
			if _, err := service.UpdateLiveTerminal(target.Tree, &target.IntroText); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "waiting":
			if _, err := service.RequestTerminalClear(); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "hacking", "blocked":
			if !activateCRTTerminal(service, response, crtHackingTerminal("a")) {
				return
			}
		case "hacking-unchanged":
			target := crtHackingTerminal("a")
			if _, err := service.UpdateLiveTerminal(target.Tree, &target.IntroText); err != nil {
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
		case "hacking-replacement":
			if !activateCRTTerminal(service, response, crtHackingTerminal("b")) {
				return
			}
		default:
			http.NotFound(response, request)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/local/crt/hacking-dud/{position}", func(response http.ResponseWriter, request *http.Request) {
		if !fixtureHackRandom.forceDudRemoval(request.PathValue("position")) {
			http.NotFound(response, request)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/local/disconnect", func(response http.ResponseWriter, _ *http.Request) {
		connectPlayer.CloseSubscriptions()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/presentation-uplinks/close", func(response http.ResponseWriter, _ *http.Request) {
		presentationHub.CloseUplinks(errors.New("fixture presentation uplinks closed"))
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/public-access/failure/", func(response http.ResponseWriter, request *http.Request) {
		kind := strings.TrimPrefix(request.URL.Path, "/__fixture/public-access/failure/")
		snapshot, ok := edge.publicFailure(kind)
		if !ok || strings.Contains(kind, "/") {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(snapshot)
	})
	mux.HandleFunc("POST /__fixture/public-access/recover", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(edge.publicRecovery())
	})
	mux.HandleFunc("GET /__fixture/edge/status", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(fixtureEdgeStatus{
			AuthBoundary: "application-ingress", Upstream: "http://" + fixtureAddress,
			Active: edge.active.Load(), AuthorizationForwarded: request.Header.Get("Authorization") != "",
			PublicURL: edge.publicURL,
		})
	})
	mux.HandleFunc("POST /__fixture/edge/update", func(response http.ResponseWriter, _ *http.Request) {
		edge.update(response)
	})
	mux.HandleFunc("POST /__fixture/edge/hacking", func(response http.ResponseWriter, _ *http.Request) {
		edge.activateHacking(response)
	})
	mux.HandleFunc("POST /__fixture/edge/disconnect", func(response http.ResponseWriter, _ *http.Request) {
		edge.connect.CloseSubscriptions()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/presentation-gate/arm", func(response http.ResponseWriter, _ *http.Request) {
		presentationGate.arm()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/presentation-gate/arm-unary", func(response http.ResponseWriter, _ *http.Request) {
		presentationGate.armUnary()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /__fixture/edge/presentation-gate/blocked", func(response http.ResponseWriter, _ *http.Request) {
		if !presentationGate.isBlocked() {
			response.WriteHeader(http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/presentation-gate/release", func(response http.ResponseWriter, _ *http.Request) {
		presentationGate.release()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/presentation-gate/cancel-uplinks", func(response http.ResponseWriter, _ *http.Request) {
		edge.hub.CloseUplinks(errors.New("fixture presentation uplinks rotated"))
		presentationGate.cancelAfter(2 * time.Second)
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /__fixture/edge/disable", func(response http.ResponseWriter, _ *http.Request) {
		edge.ingress.Deny()
		edge.active.Store(false)
		edge.connect.CloseSubscriptions()
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/__fixture/protected/", func(response http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != fixtureEdgeUsername || password != fixtureEdgePassword {
			response.Header().Set("WWW-Authenticate", `Basic realm="Fallout Terminal Players"`)
			http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		forwarded := request.Clone(request.Context())
		forwarded.URL.Path = "/" + strings.TrimPrefix(request.URL.Path, "/__fixture/protected/")
		applicationHandler.ServeHTTP(response, forwarded)
	})
	mux.Handle("/", applicationHandler)

	listener, err := net.Listen("tcp4", fixtureAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", fixtureAddress, err)
	}
	ingress, err := tunnel.NewPublicIngressFactory().Start(ctx, "http://"+fixtureAddress)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("start fixture public ingress: %w", err)
	}
	edge.ingress = ingress
	publicTLS := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !edge.active.Load() {
			http.NotFound(response, request)
			return
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != fixtureEdgeUsername || password != fixtureEdgePassword {
			response.Header().Set("WWW-Authenticate", `Basic realm="Fallout Terminal Players"`)
			http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		forwarded := request.Clone(request.Context())
		forwarded.Header.Del("Authorization")
		forwarded.Header.Del("Proxy-Authorization")
		mux.ServeHTTP(response, forwarded)
	}))
	publicTLS.EnableHTTP2 = true
	publicTLS.StartTLS()
	edge.publicURL = publicTLS.URL
	if err := edge.reset(); err != nil {
		publicTLS.Close()
		_ = ingress.Close(ctx)
		_ = listener.Close()
		return fmt.Errorf("activate fixture public ingress: %w", err)
	}
	fixtureProtocols := new(http.Protocols)
	fixtureProtocols.SetHTTP1(true)
	fixtureProtocols.SetUnencryptedHTTP2(true)
	httpServer := &http.Server{Handler: mux, Protocols: fixtureProtocols}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}

	shutdownDeadline, stopShutdownDeadline := context.WithTimeoutCause(context.WithoutCancel(ctx), 3*time.Second, errors.New("fixture shutdown timed out"))
	shutdownContext, cancelShutdown := context.WithCancelCause(shutdownDeadline)
	defer func() {
		cancelShutdown(errors.New("fixture shutdown complete"))
		stopShutdownDeadline()
	}()
	ingress.Deny()
	publicTLS.Close()
	return errors.Join(ingress.Close(shutdownContext), httpServer.Shutdown(shutdownContext))
}

func fixtureTerminal() domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: "terminal-1", TerminalName: "Overseer", HackLevel: 0, IntroText: "WELCOME",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{
				{ID: "docs", Type: domain.NodeFolder, Name: "DOCS", Children: []domain.ContentNode{
					{ID: "report", Type: domain.NodeEntry, Name: "REPORT", Description: "SYSTEM NOMINAL"},
				}},
				{ID: "status", Type: domain.NodeEntry, Name: "STATUS", Description: "ALL SYSTEMS OPERATIONAL"},
			},
		},
	}
}

func stateChangingApprovalTarget() domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: "terminal-stateful", TerminalName: "Терминал охраны", HackLevel: 0,
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{
				{
					ID: "approval-guide", Type: domain.NodeFolder, Name: "СПРАВКА",
					Children: []domain.ContentNode{{
						ID: "archive-note", Type: domain.NodeEntry, Name: "ПАМЯТКА",
						Description: "Команды требуют разрешения смотрителя.",
					}},
				},
				{
					ID: "renderer-reference", Type: domain.NodeEntry, Name: "ЭТАЛОН РЕНДЕРА",
					Description: fixtureCommandResult,
				},
				{
					ID: "diagnostics", Type: domain.NodeCommand, Name: "Запустить диагностику",
					Text: "Диагностика завершена.",
				},
				{
					ID: "doors", Type: domain.NodeCommand, Name: "Открыть двери",
					Text: "Доступ в сектор разрешён.",
					StateChange: &domain.StateChangeConfig{
						CompletedName:    "Двери открыты",
						ConfirmationText: "Разрешить доступ в защищённый сектор?",
					},
				},
			},
		},
	}
}

func stateChangingApprovalSession(states map[string]domain.CommandExecutionState) domain.Session {
	target := stateChangingApprovalTarget()
	return domain.Session{
		Version: 1,
		Name:    "State-changing approval fixture",
		Terminals: []domain.Terminal{{
			ID: target.TerminalID, Name: target.TerminalName, HackLevel: target.HackLevel,
			IntroText: target.IntroText, Root: target.Tree,
			CommandStates: cloneFixtureCommandStates(states),
		}},
	}
}

func stateChangingAuthoringSession() domain.Session {
	return domain.Session{
		Version: 1,
		Name:    "State-changing authoring fixture",
		Terminals: []domain.Terminal{{
			ID: "terminal-stateful", Name: "Терминал охраны",
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{
					{
						ID: "emergency-lights", Type: domain.NodeCommand,
						Name: "Включить аварийный свет", Text: "Аварийное освещение включено.",
					},
					{
						ID: "doors", Type: domain.NodeCommand, Name: "Открыть двери",
						Text: "Новая редакция результата открытия.",
						StateChange: &domain.StateChangeConfig{
							CompletedName: "Двери разблокированы", ConfirmationText: "Открыть двери?",
						},
					},
					{
						ID: "alarm", Type: domain.NodeCommand, Name: "Включить тревогу",
						Text: "Сигнал тревоги активирован.",
						StateChange: &domain.StateChangeConfig{
							CompletedName: "Сигнал тревоги активен", ConfirmationText: "Включить тревогу?",
						},
					},
				},
			},
			CommandStates: map[string]domain.CommandExecutionState{
				"doors": {CompletedName: "Двери открыты", ResultText: "Доступ в сектор разрешён."},
				"alarm": {CompletedName: "Тревога включена", ResultText: "Охрана сектора предупреждена."},
			},
		}},
	}
}

func terminalGroupingSession(scenario string) (domain.Session, error) {
	scenario = strings.TrimSpace(scenario)
	if scenario == "" {
		scenario = "canonical"
	}
	terminal := func(id, name string) domain.Terminal {
		return domain.Terminal{
			ID: id, Name: name,
			Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}},
		}
	}
	switch scenario {
	case "canonical":
		return domain.Session{
			Version: 1, Name: "Terminal grouping canonical fixture",
			Terminals: []domain.Terminal{
				terminal("residential", "Жилой терминал"),
				terminal("security", "Терминал охраны"),
				terminal("vault", "Терминал хранилища"),
			},
			TerminalGroups: []domain.TerminalGroup{
				{ID: "vault-route", Name: "Маршрут хранилища", TerminalIDs: []string{"security", "residential"}},
				{ID: "vault-standalone", Name: "Отдельное хранилище", TerminalIDs: []string{"vault"}},
			},
		}, nil
	case "singleton":
		return domain.Session{
			Version: 1, Name: "Terminal grouping singleton fixture",
			Terminals: []domain.Terminal{
				terminal("medical", "Медицинский терминал"),
				terminal("reactor", "Терминал реактора"),
				terminal("archive", "Архивный терминал"),
			},
			TerminalGroups: []domain.TerminalGroup{
				{ID: "medical-singleton", Name: "Медицинский блок", TerminalIDs: []string{"medical"}},
				{ID: "reactor-singleton", Name: "Реакторный блок", TerminalIDs: []string{"reactor"}},
				{ID: "archive-singleton", Name: "Архивный блок", TerminalIDs: []string{"archive"}},
			},
		}, nil
	case "ordered":
		return domain.Session{
			Version: 1, Name: "Terminal grouping ordered fixture",
			Terminals: []domain.Terminal{
				terminal("alpha", "Терминал Альфа"),
				terminal("beta", "Терминал Бета"),
				terminal("gamma", "Терминал Гамма"),
				terminal("delta", "Терминал Дельта"),
				terminal("epsilon", "Терминал Эпсилон"),
			},
			TerminalGroups: []domain.TerminalGroup{
				{ID: "south-route", Name: "Южный маршрут", TerminalIDs: []string{"delta", "beta"}},
				{ID: "north-route", Name: "Северный маршрут", TerminalIDs: []string{"gamma", "alpha"}},
				{ID: "epsilon-singleton", Name: "Терминал Гамма", TerminalIDs: []string{"epsilon"}},
			},
		}, nil
	case "legacy":
		legacyOne := terminal("legacy-one", "Старый терминал 1")
		legacyOne.Root.Children = append(legacyOne.Root.Children, domain.ContentNode{
			ID: "legacy-transition", Type: domain.NodeCommand, Name: "СТАРЫЙ ПЕРЕХОД",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "legacy-two"},
		})
		return domain.Session{
			Version: 1, Name: "Terminal grouping legacy fixture",
			Terminals: []domain.Terminal{
				legacyOne,
				terminal("legacy-two", "Старый терминал 2"),
				terminal("legacy-three", "Старый терминал 3"),
			},
		}, nil
	case "legacy-multi-link":
		service := terminal("t-krel-service", "K-REL / СЕРВИСНЫЙ КОНТУР")
		service.Root.Children = append(service.Root.Children, domain.ContentNode{
			ID: "svc-access-admin", Type: domain.NodeCommand, Name: "ВХОД АДМИНИСТРАТОРА",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "t-krel-admin"},
		})
		admin := terminal("t-krel-admin", "K-REL / АДМИНИСТРАТОР")
		admin.Root.Children = append(admin.Root.Children, domain.ContentNode{
			ID: "adm-emergency", Type: domain.NodeCommand, Name: "АВАРИЙНОЕ УПРАВЛЕНИЕ",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "t-krel-emergency"},
		})
		emergency := terminal("t-krel-emergency", "K-REL / АВАРИЙНОЕ УПРАВЛЕНИЕ")
		emergency.HackLevel = 4
		return domain.Session{
			Version: 1, Name: "session-05-cold-storage",
			Terminals: []domain.Terminal{service, admin, emergency},
		}, nil
	case "legacy-multi-link-authored":
		return bug004AuthoredSession()
	case "ordered-navigation":
		alpha := terminal("alpha", "Терминал Альфа")
		alpha.Root.Children = append(alpha.Root.Children, domain.ContentNode{
			ID: "go-beta", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ТЕРМИНАЛ БЕТА",
			TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "beta"},
		})
		beta := terminal("beta", "Терминал Бета")
		beta.Root.Children = append(beta.Root.Children,
			domain.ContentNode{
				ID: "go-gamma", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ТЕРМИНАЛ ГАММА",
				TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "gamma"},
			},
			domain.ContentNode{
				ID: "go-gamma-backup", Type: domain.NodeCommand, Name: "РЕЗЕРВНЫЙ МАРШРУТ К ГАММЕ",
				TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "gamma"},
			},
		)
		gamma := terminal("gamma", "Терминал Гамма")
		gamma.Root.Children = append(gamma.Root.Children,
			domain.ContentNode{ID: "check-link", Type: domain.NodeCommand, Name: "ПРОВЕРИТЬ СВЯЗЬ", Text: "СВЯЗЬ СТАБИЛЬНА"},
			domain.ContentNode{
				ID: "go-delta", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ТЕРМИНАЛ ДЕЛЬТА",
				TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "delta"},
			},
		)
		return domain.Session{
			Version: 1, Name: "Terminal grouping ordered navigation fixture",
			Terminals: []domain.Terminal{
				alpha, beta, gamma, terminal("delta", "Терминал Дельта"), terminal("epsilon", "Терминал Эпсилон"),
			},
			TerminalGroups: []domain.TerminalGroup{
				{ID: "ordered-route", Name: "Полный маршрут", TerminalIDs: []string{"alpha", "beta", "gamma", "delta"}},
				{ID: "epsilon-standalone", Name: "Отдельный терминал", TerminalIDs: []string{"epsilon"}},
			},
		}, nil
	default:
		return domain.Session{}, fmt.Errorf("unknown terminal-grouping scenario %q", scenario)
	}
}

func bug004AuthoredSession() (domain.Session, error) {
	const exactSHA256 = "b4ca8b89b7d7af32e05a9b598a007e36a747ef59ce3e2bd15a60d0b3f0ec9438"
	exactPath := strings.TrimSpace(os.Getenv("FALLOUT_BUG004_SOURCE"))
	candidates := []string{exactPath}
	if exactPath == "" {
		candidates = []string{
			filepath.Join("..", "fixtures", "session-05-cold-storage.json"),
			filepath.Join("tests", "fixtures", "session-05-cold-storage.json"),
			filepath.Join("..", "..", "tests", "fixtures", "session-05-cold-storage.json"),
		}
	}
	var raw []byte
	var sourcePath string
	var readErr error
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		raw, readErr = os.ReadFile(candidate)
		if readErr == nil {
			sourcePath = candidate
			break
		}
	}
	if readErr != nil || sourcePath == "" {
		return domain.Session{}, fmt.Errorf("read BUG-004 authored fixture: %w", readErr)
	}
	if exactPath != "" && fmt.Sprintf("%x", sha256.Sum256(raw)) != exactSHA256 {
		return domain.Session{}, fmt.Errorf("BUG-004 authored source SHA-256 does not match %s", exactSHA256)
	}
	session, err := domain.DecodeSession(raw)
	if err != nil {
		return domain.Session{}, fmt.Errorf("decode BUG-004 authored fixture %s: %w", sourcePath, err)
	}
	return session, nil
}

func terminalNavigationSession() domain.Session {
	return domain.Session{
		Version: 1, Name: "Terminal navigation fixture",
		Terminals: []domain.Terminal{
			{
				ID: "residential", Name: "Жилой терминал",
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
					{ID: "ordinary", Type: domain.NodeCommand, Name: "ЗАПУСТИТЬ ДИАГНОСТИКУ", Text: "СИСТЕМА ИСПРАВНА"},
					{ID: "go-security", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ОХРАНУ", TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "security"}},
					{ID: "navigation", Type: domain.NodeFolder, Name: "НАВИГАЦИЯ", Children: []domain.ContentNode{
						{ID: "go-security-nested", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ОХРАНУ ИЗ ПАПКИ", TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "security"}},
					}},
					{ID: "completed", Type: domain.NodeCommand, Name: "ЗАВЕРШЁННАЯ КОМАНДА", Text: "Done", StateChange: &domain.StateChangeConfig{CompletedName: "ЗАВЕРШЁННАЯ КОМАНДА", ConfirmationText: "Run?"}},
				}},
				CommandStates: map[string]domain.CommandExecutionState{"completed": {CompletedName: "ЗАВЕРШЁННАЯ КОМАНДА", ResultText: "Done"}},
			},
			{
				ID: "security", Name: "Терминал охраны", HackLevel: 1,
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
					{ID: "go-vault", Type: domain.NodeCommand, Name: "ПЕРЕЙТИ В ХРАНИЛИЩЕ", TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "vault"}},
				}},
			},
			{
				ID: "vault", Name: "Терминал хранилища",
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}},
			},
		},
		TerminalGroups: []domain.TerminalGroup{
			{ID: "navigation-route", Name: "Навигационный маршрут", TerminalIDs: []string{"residential", "security"}},
			{ID: "vault-singleton", Name: "Терминал хранилища", TerminalIDs: []string{"vault"}},
		},
	}
}

func stateChangingSyncTarget() domain.TerminalTarget {
	target := stateChangingApprovalTarget()
	target.TerminalName = "Терминал синхронизации"
	target.Tree.Children = append(target.Tree.Children, domain.ContentNode{
		ID: "archive", Type: domain.NodeFolder, Name: "АРХИВ",
		Children: []domain.ContentNode{{
			ID: "journal", Type: domain.NodeEntry, Name: "ЖУРНАЛ",
			Description: "Состояние дверей зарегистрировано.",
		}},
	})
	return target
}

func stateChangingSyncReserveTarget() domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: "terminal-reserve", TerminalName: "Резервный терминал", HackLevel: 0,
		Tree: domain.ContentNode{
			ID: "reserve-root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: "reserve-status", Type: domain.NodeEntry, Name: "РЕЗЕРВНЫЙ СТАТУС",
				Description: "Резервный канал активен.",
			}},
		},
	}
}

func stateChangingSyncSession(states map[string]domain.CommandExecutionState) domain.Session {
	target := stateChangingSyncTarget()
	return domain.Session{
		Version: 1,
		Name:    "State-changing synchronization fixture",
		Terminals: []domain.Terminal{{
			ID: target.TerminalID, Name: target.TerminalName, HackLevel: target.HackLevel,
			IntroText: target.IntroText, Root: target.Tree,
			CommandStates: cloneFixtureCommandStates(states),
		}},
	}
}

func cloneFixtureCommandStates(states map[string]domain.CommandExecutionState) map[string]domain.CommandExecutionState {
	if len(states) == 0 {
		return nil
	}
	clone := make(map[string]domain.CommandExecutionState, len(states))
	maps.Copy(clone, states)
	return clone
}

func crtFixtureTerminal() domain.TerminalTarget {
	children := make([]domain.ContentNode, 0, 25)
	for index := 1; index <= 21; index++ {
		children = append(children, domain.ContentNode{
			ID:          fmt.Sprintf("crt-row-%02d", index),
			Type:        domain.NodeEntry,
			Name:        fmt.Sprintf("ARCHIVE %02d", index),
			Description: fmt.Sprintf("CRT ARCHIVE LINE %02d", index),
		})
	}
	children = append(children,
		domain.ContentNode{ID: "crt-empty", Type: domain.NodeFolder, Name: "EMPTY", Children: []domain.ContentNode{}},
		domain.ContentNode{
			ID: "crt-record", Type: domain.NodeEntry, Name: "LONG RECORD",
			Description: strings.Repeat("ROBCO RECORD LINE\n", 48) + "RECORD COMPLETE",
		},
		domain.ContentNode{
			ID: "crt-command", Type: domain.NodeCommand, Name: "RUN DIAGNOSTIC",
			Text: strings.Repeat("DIAGNOSTIC OUTPUT\n", 96) + "DIAGNOSTIC COMPLETE",
		},
		domain.ContentNode{
			ID: "crt-literal", Type: domain.NodeEntry, Name: `<img data-crt-injected src=x onerror="window.__crtInjected=true">`,
			Description: `<script>window.__crtInjected=true</script> & literal terminal text`,
		},
	)

	return domain.TerminalTarget{
		TerminalID: "terminal-crt", TerminalName: "CRT Acceptance", HackLevel: 0,
		IntroText: "CRT PRESENTATION ACCEPTANCE",
		Tree:      domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: children},
	}
}

func crtReplacementTerminal() domain.TerminalTarget {
	target := crtFixtureTerminal()
	target.IntroText = "CRT REPLACEMENT ACCEPTANCE"
	target.Tree.Children = []domain.ContentNode{
		{ID: "crt-replacement-a", Type: domain.NodeEntry, Name: "REPLACEMENT ALPHA", Description: "ALPHA"},
		{ID: "crt-replacement-b", Type: domain.NodeEntry, Name: "REPLACEMENT BETA", Description: "BETA"},
		{ID: "crt-replacement-c", Type: domain.NodeEntry, Name: "REPLACEMENT GAMMA", Description: "GAMMA"},
	}
	return target
}

func crtHackingTerminal(identity string) domain.TerminalTarget {
	target := crtFixtureTerminal()
	target.TerminalID = "terminal-crt-hacking-" + identity
	target.TerminalName = "CRT Security " + strings.ToUpper(identity)
	target.HackLevel = 1
	return target
}
