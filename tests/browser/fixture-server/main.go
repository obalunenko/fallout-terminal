package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/obalunenko/Fallout-Terminal/internal/control"
	"github.com/obalunenko/Fallout-Terminal/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/internal/live"
	"github.com/obalunenko/Fallout-Terminal/internal/player"
	"github.com/obalunenko/Fallout-Terminal/internal/tunnel"
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
	catalog.session = domain.CloneSession(session)
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
	ingress          tunnel.PublicIngress
	publicURL        string
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
	if err := edge.ingress.Activate(edge.ingress.URL().Host, fixtureEdgeUsername, []byte(fixtureEdgePassword)); err != nil {
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
	playerManagementStore := &fixturePlayerManagementStore{}
	playerManagementStore.reset()
	navigationCatalog := &fixtureTerminalCatalog{session: terminalNavigationSession()}
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
	connectPlayer, err = player.NewConnectService(player.ConnectServiceConfig{Coordinator: service, Assets: playerAssets})
	if err != nil {
		return fmt.Errorf("construct fixture Connect service: %w", err)
	}
	rpcPath, rpcHandler := player.NewConnectHandler(connectPlayer)
	applicationHandler := player.NewApplicationHandler(playerAssets, rpcPath, rpcHandler)
	edge := &fixtureEdge{service: service, connect: connectPlayer}

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
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>
<script type="module" src="/__fixture/desktop-api.js"></script>`)
	})
	mux.HandleFunc("GET /__fixture/player-management", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
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
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
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
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
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
	mux.HandleFunc("GET /__fixture/terminal-navigation/overseer", func(response http.ResponseWriter, _ *http.Request) {
		raw, readErr := os.ReadFile(filepath.Clean("../../frontend/overseer/src/index.html"))
		if readErr != nil {
			http.Error(response, "fixture overseer page is unavailable", http.StatusInternalServerError)
			return
		}
		page := strings.Replace(string(raw), `<head>`, `<head>
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
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
		if err := json.NewDecoder(request.Body).Decode(&candidate); err != nil || domain.ValidateSession(candidate) != nil {
			http.Error(response, "invalid terminal navigation session", http.StatusBadRequest)
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
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
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
<script type="importmap">{"imports":{"@wailsio/runtime":"/__fixture/desktop-bindings.js","/bindings/github.com/obalunenko/Fallout-Terminal/desktopservice.js":"/__fixture/desktop-bindings.js"}}</script>`, 1)
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
	edge.publicURL = ingress.URL().String()
	if err := edge.reset(); err != nil {
		_ = ingress.Close(ctx)
		_ = listener.Close()
		return fmt.Errorf("activate fixture public ingress: %w", err)
	}
	httpServer := &http.Server{Handler: mux}
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
				Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT"},
			},
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
