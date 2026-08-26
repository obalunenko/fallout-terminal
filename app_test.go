package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	controlservice "github.com/obalunenko/Fallout-Terminal/internal/control"
	"github.com/obalunenko/Fallout-Terminal/internal/domain"
	liveservice "github.com/obalunenko/Fallout-Terminal/internal/live"
	playerconfigservice "github.com/obalunenko/Fallout-Terminal/internal/playerconfig"
	sessionservice "github.com/obalunenko/Fallout-Terminal/internal/session"
	"github.com/obalunenko/Fallout-Terminal/internal/testutil"
	tunnelservice "github.com/obalunenko/Fallout-Terminal/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationStartsPlayerBeforePublishingReady(t *testing.T) {
	recorder := &callRecorder{}
	player := &recordingPlayerServer{
		recorder: recorder,
		info: domain.ServerInfo{
			IP: "192.0.2.10", Port: 3690, URL: "http://192.0.2.10:3690",
		},
	}
	events := &recordingEventSink{recorder: recorder}
	desktop := &recordingDesktop{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Player:  player,
		Events:  events,
		Desktop: desktop,
	})
	{

		err := app.Start(t.Context())
		require.Falsef(t, err != nil,
			"Start() error = %v", err)
	}
	{

		got, want := recorder.Calls(), []string{"player:start", "event:server-info", "desktop:ready"}
		require.Falsef(t, !cmp.Equal(got, want),
			"startup calls = %v, want %v", got, want)
	}

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.ServerInfo == nil || status.ServerInfo.Port != 3690 || status.StartupError != "",
		"runtime status = %#v", status)

}

func TestApplicationLifecycleEmitsStructuredOperationalLogs(t *testing.T) {
	t.Parallel()

	logs := testutil.NewRecordingLogger()
	recorder := &callRecorder{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Logger: logs,
		Player: &recordingPlayerServer{recorder: recorder, info: domain.ServerInfo{
			IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690",
		}},
		Events:  &recordingEventSink{recorder: recorder},
		Desktop: &recordingDesktop{recorder: recorder},
	})

	require.NoError(t, app.Start(t.Context()))
	require.NoError(t, app.Shutdown(t.Context()))

	records := logs.Records()
	for _, message := range []string{
		"application startup started",
		"player server ready",
		"desktop runtime ready",
		"application ready",
		"application shutdown started",
		"application shutdown completed",
	} {
		requireLogRecord(t, records, message)
	}
	ready := requireLogRecord(t, records, "application ready")
	require.Equal(t, "ready-local", ready.Fields["phase"])
	player := requireLogRecord(t, records, "player server ready")
	require.Equal(t, 3690, player.Fields["port"])
	require.Equal(t, []string{
		"player:start", "event:server-info", "desktop:ready", "player:stop", "desktop:close",
	}, recorder.Calls())
}

func TestProductionLoggingUsesRequiredLoggerInitializedOnce(t *testing.T) {
	t.Parallel()

	mainSource, err := os.ReadFile("main.go")
	require.NoError(t, err)
	moduleSource, err := os.ReadFile("go.mod")
	require.NoError(t, err)

	require.Equal(t, 1, strings.Count(string(mainSource), "logger.Init("))
	require.Contains(t, string(mainSource), `"github.com/obalunenko/logger"`)
	require.NotContains(t, string(mainSource), `"log"`)
	require.Contains(t, string(moduleSource), "github.com/obalunenko/logger v1.2.0")
}

func requireLogRecord(t *testing.T, records []testutil.LogRecord, message string) testutil.LogRecord {
	t.Helper()
	return requireMatchingLogRecord(t, records, "message "+message, func(record testutil.LogRecord) bool {
		return record.Message == message
	})
}

func TestApplicationCommandLogsRecordSafeOutcomesAndSwallowedEventErrors(t *testing.T) {
	t.Parallel()

	const (
		providerCanary  = "PROVIDER-SECRET-CANARY-0123456789"
		passwordCanary  = "PLAYER-PASSWORD-CANARY"
		contentCanary   = "SESSION-CONTENT-CANARY"
		characterCanary = "CHARACTER-NAME-CANARY"
	)

	t.Run("session outcomes", func(t *testing.T) {
		t.Parallel()
		logs := testutil.NewRecordingLogger()
		sessions := &loggingSessionCommands{
			createResult: sessionservice.SessionResult{OK: true, Session: &domain.Session{Version: 1, Name: contentCanary}},
			openResult:   sessionservice.SessionResult{Canceled: true},
			copyResult:   sessionservice.SessionResult{Error: contentCanary},
			saveResult:   sessionservice.SaveResult{Error: contentCanary, RequestedRevision: 1},
		}
		app := NewAppWithDependencies(t.Context(), AppDependencies{Logger: logs, Sessions: sessions})

		require.True(t, app.NewSession().OK)
		require.True(t, app.OpenSession().Canceled)
		require.False(t, app.CopyDemo().OK)
		require.False(t, app.SaveSession(domain.Session{Version: 1, Name: contentCanary}).OK)

		records := logs.Records()
		requireOperationRecord(t, records, "session.create", "succeeded")
		requireOperationRecord(t, records, "session.open", "cancelled")
		requireOperationRecord(t, records, "session.copy-demo", "failed")
		save := requireOperationRecord(t, records, "session.save", "failed")
		require.Equal(t, uint64(1), save.Fields["revision"])
		require.NotContains(t, fmt.Sprintf("%#v", records), contentCanary)
	})

	t.Run("player config and broadcast outcomes", func(t *testing.T) {
		t.Parallel()
		logs := testutil.NewRecordingLogger()
		sessions := &recordingPlayerConfigSession{snapshot: sessionservice.ActiveSession{
			Path: "/Campaigns/game.json",
			Session: &domain.Session{
				Version: 1, Name: contentCanary, Terminals: []domain.Terminal{},
			},
		}}
		configs := &recordingPlayerConfigService{next: playerconfigservice.Result{
			OK: true, FilePath: "/Campaigns/players/private.json", ContentDigest: "digest",
			Config: &domain.PlayerConfig{Version: 1, Name: contentCanary, Roster: []domain.CharacterRosterEntry{{
				ID: "private-character", Name: characterCanary,
			}}},
		}}
		coordination := &loggingPlayerConfigBroadcastCoordination{
			state:      &domain.MasterCoordinationState{Roster: []domain.MasterRosterEntry{}, Sessions: []domain.MasterSessionEntry{}},
			startState: &domain.MasterCoordinationState{Revision: 1, Roster: []domain.MasterRosterEntry{}, Sessions: []domain.MasterSessionEntry{}, Broadcast: &domain.MasterBroadcastState{}},
			endState:   &domain.MasterCoordinationState{Revision: 2, Roster: []domain.MasterRosterEntry{}, Sessions: []domain.MasterSessionEntry{}},
		}
		app := NewAppWithDependencies(t.Context(), AppDependencies{
			Logger: logs, Sessions: sessions, PlayerConfigs: configs, Coordination: coordination,
		})

		require.True(t, app.OpenPlayerConfig().OK)
		require.True(t, app.StartBroadcast().OK)
		require.True(t, app.EndBroadcast().OK)

		records := logs.Records()
		requireOperationRecord(t, records, "player-config.open", "succeeded")
		requireOperationRecord(t, records, "broadcast.start", "succeeded")
		requireOperationRecord(t, records, "broadcast.end", "succeeded")
		captured := fmt.Sprintf("%#v", records)
		require.NotContains(t, captured, contentCanary)
		require.NotContains(t, captured, characterCanary)
		require.NotContains(t, captured, "/Campaigns")
	})

	t.Run("public access and event delivery", func(t *testing.T) {
		t.Parallel()
		logs := testutil.NewRecordingLogger()
		recorder := &callRecorder{}
		preferences := tunnelservice.DefaultPublicAccessPreferences()
		preferences.Revision = 7
		disabled := tunnelservice.PublicAccessSnapshot{
			Preferences: preferences,
			Status:      tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, SettingsRevision: 7},
		}
		preferences8 := preferences
		preferences8.Revision = 8
		failed := tunnelservice.PublicAccessSnapshot{
			Preferences: preferences8,
			Status: tunnelservice.PublicAccessStatus{
				State: tunnelservice.LifecycleFailed, Generation: 2, SettingsRevision: 8,
				ErrorCategory: tunnelservice.ErrorNetworkUnavailable,
				ErrorMessage:  tunnelservice.ErrorNetworkUnavailable.SafeMessage(),
			},
		}
		core := &recordingPublicAccessCore{
			recorder: recorder, snapshot: disabled,
			reconfigureResults: []tunnelservice.PublicAccessResult{{
				OK: true, Snapshot: tunnelservice.PublicAccessSnapshot{
					Preferences: preferences8,
					Status:      tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, SettingsRevision: 8},
				},
			}},
			start: tunnelservice.PublicAccessResult{
				Error: tunnelservice.ErrorNetworkUnavailable.SafeMessage(), DiagnosticCode: tunnelservice.DiagnosticPublicIngressListenFailed,
				Snapshot: failed,
			},
			stop: tunnelservice.PublicAccessResult{OK: true, Snapshot: tunnelservice.PublicAccessSnapshot{
				Preferences: preferences8,
				Status:      tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, Generation: 3, SettingsRevision: 8},
			}},
		}
		eventErr := errors.New("event transport unavailable")
		app := NewAppWithDependencies(t.Context(), AppDependencies{
			Logger: logs, PublicAccess: core,
			Events: &recordingEventSink{recorder: recorder, err: eventErr},
		})

		saved := app.SavePublicAccessSettings(SavePublicAccessSettingsPayload{
			ExpectedRevision: 7, EnabledPreference: true, ReservedDomain: "private.example", Username: "private-user",
			ReplacementProviderToken: providerCanary, ReplacementPlayerPassword: passwordCanary,
		})
		require.True(t, saved.OK)
		require.False(t, app.StartPublicAccess(PublicAccessCommandPayload{ExpectedRevision: 8}).OK)
		require.True(t, app.StopPublicAccess(PublicAccessCommandPayload{ExpectedRevision: 8}).OK)
		app.updateClientCount(3)

		records := logs.Records()
		requireOperationRecord(t, records, "public-access.settings", "succeeded")
		started := requireOperationRecord(t, records, "public-access.start", "failed")
		require.Equal(t, "error", started.Fields["state"])
		require.Equal(t, "network_unavailable", started.Fields["error_category"])
		require.Equal(t, "public_ingress_listen_failed", started.Fields["diagnostic_code"])
		requireOperationRecord(t, records, "public-access.stop", "succeeded")
		eventRecord := requireEventLogRecord(t, records, clientCountEvent)
		require.ErrorIs(t, eventRecord.Fields["error"].(error), eventErr)

		captured := fmt.Sprintf("%#v", records)
		for _, forbidden := range []string{providerCanary, passwordCanary, "private.example", "private-user"} {
			require.NotContains(t, captured, forbidden)
		}
	})
}

func requireOperationRecord(t *testing.T, records []testutil.LogRecord, operation, outcome string) testutil.LogRecord {
	t.Helper()
	return requireMatchingLogRecord(t, records, "operation "+operation+" with outcome "+outcome, func(record testutil.LogRecord) bool {
		return record.Fields["operation"] == operation && record.Fields["outcome"] == outcome
	})
}

func requireEventLogRecord(t *testing.T, records []testutil.LogRecord, event string) testutil.LogRecord {
	t.Helper()
	return requireMatchingLogRecord(t, records, "failed event delivery for "+event, func(record testutil.LogRecord) bool {
		return record.Message == "desktop event delivery failed" && record.Fields["event"] == event
	})
}

func requireMatchingLogRecord(
	t *testing.T,
	records []testutil.LogRecord,
	description string,
	matches func(testutil.LogRecord) bool,
) testutil.LogRecord {
	t.Helper()
	var match *testutil.LogRecord
	for index := range records {
		if matches(records[index]) {
			match = &records[index]
			break
		}
	}
	require.NotNilf(t, match, "matching log record: %s; records=%#v", description, records)
	return *match
}

func TestApplicationLifetimeContextIsRetainedWhileAcquisitionUsesBoundedChild(t *testing.T) {
	t.Parallel()

	composition := context.WithValue(t.Context(), lifecycleContextKey{}, "composition")
	lifetime := context.WithValue(t.Context(), lifecycleContextKey{}, "lifetime")
	player := &contextCapturingPlayer{info: domain.ServerInfo{IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690"}}
	desktop := &contextCapturingDesktop{}
	events := &contextCapturingEvents{}
	app := NewAppWithDependencies(composition, AppDependencies{
		Player: player, Desktop: desktop, Events: events, StartupTimeout: time.Minute,
	})
	require.Equal(t, "composition", app.contextSnapshot().Value(lifecycleContextKey{}))

	require.NoError(t, app.Start(lifetime))
	require.Equal(t, "lifetime", app.contextSnapshot().Value(lifecycleContextKey{}))
	require.Equal(t, "lifetime", desktop.readyContext.Value(lifecycleContextKey{}))
	require.Equal(t, "lifetime", events.context.Value(lifecycleContextKey{}))
	require.Equal(t, "lifetime", player.startContext.Value(lifecycleContextKey{}))
	require.NoError(t, player.contextErrAtStart)
	_, bounded := player.startContext.Deadline()
	require.True(t, bounded, "player acquisition must receive the startup-timeout child")
	require.ErrorIs(t, context.Cause(player.startContext), errApplicationStartupComplete)
	require.NoError(t, app.contextSnapshot().Err(), "successful acquisition completion must not cancel the application lifetime")
	require.NoError(t, app.Shutdown(t.Context()))
	require.ErrorIs(t, context.Cause(app.contextSnapshot()), errApplicationShutdown)
}

func TestLifecyclePhaseStaysGoOnlyWhileStatusProjectsActionableState(t *testing.T) {
	t.Parallel()

	t.Run("local ready", func(t *testing.T) {
		t.Parallel()
		app := NewAppWithDependencies(t.Context(), AppDependencies{
			Player: &recordingPlayerServer{recorder: &callRecorder{}, info: domain.ServerInfo{
				IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690",
			}},
			Events:  &recordingEventSink{recorder: &callRecorder{}},
			Desktop: &recordingDesktop{recorder: &callRecorder{}},
		})
		require.Equal(t, "constructed", app.lifecyclePhase())
		require.NoError(t, app.Start(t.Context()))
		require.Equal(t, "ready-local", app.lifecyclePhase())

		status := app.GetRuntimeStatus()
		require.NotNil(t, status.ServerInfo)
		require.Equal(t, "http://127.0.0.1:3690", status.ServerInfo.URL)
		require.Empty(t, status.StartupError)
		raw, err := json.Marshal(status)
		require.NoError(t, err)
		require.NotContains(t, string(raw), `"phase"`)
	})

	t.Run("failed startup", func(t *testing.T) {
		t.Parallel()
		app := NewAppWithDependencies(t.Context(), AppDependencies{
			Player: &recordingPlayerServer{recorder: &callRecorder{}, startErr: errors.New("listener occupied")},
		})
		require.Error(t, app.Start(t.Context()))
		require.Equal(t, "failed", app.lifecyclePhase())

		status := app.GetRuntimeStatus()
		require.Nil(t, status.ServerInfo)
		require.Contains(t, status.StartupError, "listener occupied")
		routed := routeRuntimeStatus(status)
		require.Equal(t, status.StartupError, routed.StartupError)
		require.Nil(t, routed.ServerInfo)
	})
}

func TestPlayerConfigCommandsAssociateBeforeInstallingRoster(t *testing.T) {
	t.Parallel()

	coordination := &recordingPlayerConfigCoordination{state: &domain.MasterCoordinationState{Roster: []domain.MasterRosterEntry{}, Sessions: []domain.MasterSessionEntry{}}}
	sessions := &recordingPlayerConfigSession{snapshot: sessionservice.ActiveSession{Path: "/Campaigns/game.json", Session: &domain.Session{Version: 1, Name: "Game", Terminals: []domain.Terminal{}}}}
	configs := &recordingPlayerConfigService{next: playerconfigservice.Result{
		OK: true, FilePath: "/Campaigns/players/shared.json",
		Config: &domain.PlayerConfig{Version: 1, Name: "Shared", Roster: []domain.CharacterRosterEntry{{ID: "mara", Name: "Mara"}}},
	}}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Sessions: sessions, PlayerConfigs: configs, Coordination: coordination})

	result := app.OpenPlayerConfig()
	require.Falsef(t, !result.OK || result.Config == nil || result.State == nil || len(result.State.Roster) != 1,
		"OpenPlayerConfig() = %#v", result)
	{

		got, want := sessions.associations, []string{"/Campaigns/players/shared.json"}
		require.Falsef(t, !cmp.Equal(got, want),
			"session associations = %#v, want %#v", got, want)
	}
	{

		got, want := coordination.installs, []string{"Shared:mara"}
		require.Falsef(t, !cmp.Equal(got, want),
			"coordinator installs = %#v, want %#v", got, want)
	}

}

func TestNewPlayerConfigInstallsEmptyRosterAndPersistsFirstCharacter(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := "/Campaigns/players/empty.json"
	configs := playerconfigservice.NewService(
		playerconfigservice.NewStorage(fileSystem),
		&testutil.FakeDialog{SaveResult: target},
		"/Campaigns",
	)
	coordination := controlservice.New(controlservice.Config{RosterStore: configs})
	sessions := &recordingPlayerConfigSession{snapshot: sessionservice.ActiveSession{
		Path: "/Campaigns/game.json",
		Session: &domain.Session{
			Version:   1,
			Name:      "Game",
			Terminals: []domain.Terminal{},
		},
	}}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions:      sessions,
		PlayerConfigs: configs,
		Coordination:  coordination,
	})

	created := app.NewPlayerConfig()
	require.Falsef(t, !created.OK || created.Error != "" || created.Config == nil || created.Session == nil || created.State == nil,
		"NewPlayerConfig() = %#v", created)
	require.Falsef(t, created.Config.FilePath != target || created.Session.PlayerConfig == "" || created.State.PlayerConfig == nil || len(created.State.Roster) != 0,
		"new empty player config was not associated and installed: %#v", created)

	hackerUnavailable := false
	added := app.AddCharacter(CharacterCreatePayload{
		Name: "Mara", Intelligence: 10, HackerPerkAvailable: &hackerUnavailable,
		ExpectedRevision: created.State.Revision,
	})
	require.Falsef(t, !added.OK || added.State == nil || len(added.State.Roster) != 1 || added.State.Roster[0].Name != "Mara" || added.State.Roster[0].Intelligence != 10 || added.State.Roster[0].HackerPerkAvailable,
		"AddCharacter() after empty config = %#v", added)

	stored, ok := fileSystem.File(target)
	require.Falsef(t, !ok,
		"player config was not written to %q", target)

	persisted, err := domain.DecodePlayerConfig(stored)
	require.Falsef(t, err != nil,
		"DecodePlayerConfig() after first add: %v", err)
	require.Falsef(t, len(persisted.Roster) != 1 || persisted.Roster[0].Name != "Mara" || persisted.Roster[0].Intelligence != 10 || persisted.Roster[0].HackerPerkAvailable,
		"persisted roster after first add = %#v", persisted.Roster)

	reopened := playerconfigservice.NewService(
		playerconfigservice.NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		"/Campaigns",
	).Open(t.Context())
	require.True(t, reopened.OK, "Open() after add = %#v", reopened)
	require.NotNil(t, reopened.Config)
	require.Equal(t, persisted, *reopened.Config)
	require.NotEmpty(t, reopened.ContentDigest)

}

func TestRosterMutationConflictReturnsAuthoritativeReselectionGuidance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(t *testing.T, fileSystem *testutil.FakeFileSystem, target string, replacement []byte)
		check  func(t *testing.T, fileSystem *testutil.FakeFileSystem, target string, original, replacement []byte)
	}{
		{
			name: "missing",
			mutate: func(t *testing.T, fileSystem *testutil.FakeFileSystem, target string, _ []byte) {
				t.Helper()
				require.NoError(t, fileSystem.Remove(target))
			},
			check: func(t *testing.T, fileSystem *testutil.FakeFileSystem, target string, _, _ []byte) {
				t.Helper()
				_, exists := fileSystem.File(target)
				require.False(t, exists)
			},
		},
		{
			name: "unreadable",
			mutate: func(_ *testing.T, fileSystem *testutil.FakeFileSystem, target string, _ []byte) {
				fileSystem.ReadErrors[target] = errors.New("permission denied: private path detail")
			},
			check: func(t *testing.T, fileSystem *testutil.FakeFileSystem, target string, original, _ []byte) {
				t.Helper()
				stored, exists := fileSystem.File(target)
				require.True(t, exists)
				require.Equal(t, original, stored)
			},
		},
		{
			name: "replaced",
			mutate: func(_ *testing.T, fileSystem *testutil.FakeFileSystem, target string, replacement []byte) {
				fileSystem.SeedFile(target, replacement)
			},
			check: func(t *testing.T, fileSystem *testutil.FakeFileSystem, target string, _, replacement []byte) {
				t.Helper()
				stored, exists := fileSystem.File(target)
				require.True(t, exists)
				require.Equal(t, replacement, stored)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fileSystem := testutil.NewFakeFileSystem()
			target := "/Campaigns/players/conflict-" + test.name + ".json"
			configs := playerconfigservice.NewService(
				playerconfigservice.NewStorage(fileSystem),
				&testutil.FakeDialog{SaveResult: target},
				"/Campaigns",
			)
			coordination := controlservice.New(controlservice.Config{RosterStore: configs})
			app := NewAppWithDependencies(t.Context(), AppDependencies{
				Sessions: &recordingPlayerConfigSession{snapshot: sessionservice.ActiveSession{
					Path: "/Campaigns/game.json",
					Session: &domain.Session{
						Version: 1, Name: "Game", Terminals: []domain.Terminal{},
					},
				}},
				PlayerConfigs: configs,
				Coordination:  coordination,
			})

			created := app.NewPlayerConfig()
			require.True(t, created.OK, "NewPlayerConfig() = %#v", created)
			hackerUnavailable := false
			added := app.AddCharacter(CharacterCreatePayload{
				Name: "Mara", Intelligence: 8, HackerPerkAvailable: &hackerUnavailable,
				ExpectedRevision: created.State.Revision,
			})
			require.True(t, added.OK, "AddCharacter() = %#v", added)
			require.Len(t, added.State.Roster, 1)
			original, exists := fileSystem.File(target)
			require.True(t, exists)
			replacement := []byte("{\n  \"version\": 1,\n  \"name\": \"External\",\n  \"roster\": []\n}\n")
			beforeWrites := len(fileSystem.WriteCalls())
			beforeRenames := len(fileSystem.RenameCalls())
			test.mutate(t, fileSystem, target, replacement)

			hackerAvailable := true
			failed := app.UpdateCharacter(CharacterUpdatePayload{
				CharacterID: added.State.Roster[0].ID,
				Name:        "Changed Draft", Intelligence: 10, HackerPerkAvailable: &hackerAvailable,
				ExpectedRevision: added.State.Revision,
			})
			require.False(t, failed.OK)
			require.Contains(t, failed.Error, "reopen or reselect it")
			require.NotContains(t, failed.Error, "private path detail")
			require.Equal(t, added.State, failed.State, "failure must return the last authoritative roster")
			require.Equal(t, beforeWrites, len(fileSystem.WriteCalls()))
			require.Equal(t, beforeRenames, len(fileSystem.RenameCalls()))
			test.check(t, fileSystem, target, original, replacement)
		})
	}
}

func TestDesktopSessionFacadePreservesExplicitPathUnknownFieldsAndNewestRevision(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := "/Campaigns/vault-13/session.json"
	fileSystem.SeedFile(target, []byte(`{
  "version": 1,
  "name": "before",
  "campaignNote": {"keep": true},
  "terminals": [{
    "id": "terminal-1",
    "name": "Overseer",
    "hackLevel": 0,
    "introText": "",
    "terminalNote": 13,
    "root": {"id":"root","type":"folder","name":"ROOT","children":[]}
  }]
}`))
	sessions := sessionservice.NewService(
		sessionservice.NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		sessionservice.Locations{
			DocumentsDefault: "/Users/test/Documents/Fallout Terminal/Sessions",
			BundledDemo:      "/Applications/Fallout Terminal.app/Contents/Resources/sessions/demo.json",
		},
	)
	t.Cleanup(func() { require.NoError(t, sessions.Shutdown(context.WithoutCancel(t.Context()))) })
	app := NewAppWithDependencies(t.Context(), AppDependencies{Sessions: sessions})

	opened := app.OpenSession()
	require.True(t, opened.OK)
	require.Equal(t, target, opened.FilePath)
	require.NotNil(t, opened.Session)

	for revision := uint64(1); revision <= 3; revision++ {
		edited := *opened.Session
		edited.Name = fmt.Sprintf("revision-%d", revision)
		result := app.SaveSession(edited)
		require.True(t, result.OK, "revision %d: %#v", revision, result)
		require.Equal(t, revision, result.RequestedRevision)
		require.GreaterOrEqual(t, result.SavedRevision, revision)
	}

	written, ok := fileSystem.File(target)
	require.True(t, ok)
	require.Contains(t, string(written), `"campaignNote"`)
	require.Contains(t, string(written), `"terminalNote"`)
	decoded, err := domain.DecodeSession(written)
	require.NoError(t, err)
	require.Equal(t, "revision-3", decoded.Name)
	status := app.GetRuntimeStatus()
	require.Equal(t, uint64(3), status.RequestedRevision)
	require.Equal(t, uint64(3), status.SavedRevision)
}

func TestDesktopSessionFacadeSavesRealDemoCrossTerminalLinkAndReopensIt(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("sessions/demo.json")
	require.NoError(t, err)
	target := "/Campaigns/demo-boundary.json"
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, raw)
	sessions := sessionservice.NewService(
		sessionservice.NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		sessionservice.Locations{
			DocumentsDefault: "/Campaigns",
			BundledDemo:      "/Applications/Fallout Terminal.app/Contents/Resources/sessions/demo.json",
		},
	)
	t.Cleanup(func() { require.NoError(t, sessions.Shutdown(context.WithoutCancel(t.Context()))) })
	app := NewAppWithDependencies(t.Context(), AppDependencies{Sessions: sessions})

	opened := app.OpenSession()
	require.True(t, opened.OK, "OpenSession() = %#v", opened)
	require.NotNil(t, opened.Session)
	require.Equal(t, []string{"t_demo1", "t_demo2"}, []string{
		opened.Session.Terminals[0].ID,
		opened.Session.Terminals[1].ID,
	})
	require.Equal(t, "t_demo2", opened.Session.Terminals[0].Root.Children[4].TerminalTransition.TargetTerminalID)

	saved := app.SaveSession(*opened.Session)
	require.True(t, saved.OK, "SaveSession() = %#v", saved)
	require.Equal(t, uint64(1), saved.SavedRevision)
	reopened := app.OpenSession()
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.Len(t, reopened.Session.Terminals, 2)
	require.Equal(t, "t_demo2", reopened.Session.Terminals[0].Root.Children[4].TerminalTransition.TargetTerminalID)
}

func TestTerminalGroupReplacementPublishesCanonicalSessionBeforeCoordinationAndReturnsDetachedState(t *testing.T) {
	recorder := &callRecorder{}
	canonicalSession := terminalGroupApplicationSession([]domain.TerminalGroup{
		{ID: "operations", Name: "Operations", TerminalIDs: []string{"terminal-b", "terminal-a"}},
	})
	canonicalState := &domain.MasterCoordinationState{
		Revision: 22,
		Broadcast: &domain.MasterBroadcastState{
			ID: "broadcast-group-replacement",
		},
	}
	coordination := &recordingTerminalGroupCoordinationService{
		recordingCoordinationService: recordingCoordinationService{
			state: &domain.MasterCoordinationState{Revision: 21},
		},
		recorder:         recorder,
		replacementState: canonicalState,
		mutation: &controlservice.TerminalGroupMutation{
			Changed: true, Revision: 12, Session: canonicalSession,
		},
	}
	sessions := &loggingSessionCommands{active: sessionservice.ActiveSession{
		Path: "/Campaigns/grouped.json", Session: &canonicalSession,
		RequestedRevision: 12, SavedRevision: 12, SaveState: sessionservice.SaveStateSaved,
	}}
	events := &recordingEventSink{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions, Coordination: coordination, Events: events,
	})
	payload := TerminalGroupReplacementPayload{
		TerminalGroups: []domain.TerminalGroup{
			{ID: "operations", Name: "Operations", TerminalIDs: []string{"terminal-b", "terminal-a"}},
		},
		ExpectedSessionRevision: 11, ExpectedCoordinationRevision: 21,
	}

	result := app.ReplaceTerminalGroups(payload)
	require.True(t, result.OK, "ReplaceTerminalGroups() = %#v", result)
	require.Empty(t, result.Error)
	require.Equal(t, uint64(12), result.SessionRevision)
	require.Equal(t, &canonicalSession, result.Session)
	require.Equal(t, canonicalState, result.CoordinationState)
	require.Len(t, coordination.calls, 1)
	require.Equal(t, domain.TerminalGroupCandidate{
		TerminalGroups:               payload.TerminalGroups,
		ExpectedSessionRevision:      11,
		ExpectedCoordinationRevision: 21,
	}, coordination.calls[0].candidate)
	require.Same(t, t.Context(), coordination.calls[0].ctx)
	require.Equal(t, []string{
		"coordinator:replace-terminal-groups",
		"event:session-state",
		"event:coordination-state",
	}, recorder.Calls())

	records := events.Records()
	require.Len(t, records, 2)
	require.Equal(t, sessionStateEvent, records[0].Name)
	sessionEvent, ok := records[0].Payload.(SessionStateEvent)
	require.True(t, ok, "session event = %#v", records[0].Payload)
	require.Equal(t, uint64(12), sessionEvent.Revision)
	require.Equal(t, &canonicalSession, sessionEvent.Session)
	require.Equal(t, coordinationStateEvent, records[1].Name)
	coordinationEvent, ok := records[1].Payload.(*domain.MasterCoordinationState)
	require.True(t, ok, "coordination event = %#v", records[1].Payload)
	require.Equal(t, canonicalState, coordinationEvent)

	result.Session.TerminalGroups[0].TerminalIDs[0] = "mutated-result"
	result.CoordinationState.Revision = 999
	require.Equal(t, []string{"terminal-b", "terminal-a"}, sessionEvent.Session.TerminalGroups[0].TerminalIDs)
	require.Equal(t, uint64(22), coordinationEvent.Revision)
	status := app.GetRuntimeStatus()
	require.Equal(t, uint64(12), status.RequestedRevision)
	require.Equal(t, uint64(12), status.SavedRevision)
	require.Equal(t, uint64(22), status.CoordinationState.Revision)
	sessionEvent.Session.TerminalGroups[0].TerminalIDs[0] = "mutated-event"
	coordinationEvent.Revision = 1000
	require.Equal(t, []string{"terminal-b", "terminal-a"}, sessions.Snapshot().Session.TerminalGroups[0].TerminalIDs)
	require.Equal(t, uint64(22), app.GetRuntimeStatus().CoordinationState.Revision)
}

func TestTerminalGroupReplacementForwardsExactLegacyRepairCandidate(t *testing.T) {
	t.Parallel()

	candidate := []domain.TerminalGroup{{
		ID: "singleton-source", Name: "Source", TerminalIDs: []string{"terminal-a", "terminal-b"},
	}}
	canonicalSession := terminalGroupApplicationSession(candidate)
	coordination := &recordingTerminalGroupCoordinationService{
		replacementState: &domain.MasterCoordinationState{Revision: 9},
		mutation: &controlservice.TerminalGroupMutation{
			Changed: true, Revision: 6, Session: canonicalSession,
		},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: coordination})

	result := app.ReplaceTerminalGroups(TerminalGroupReplacementPayload{
		TerminalGroups: candidate, ExpectedSessionRevision: 5, ExpectedCoordinationRevision: 8,
	})

	require.True(t, result.OK, "ReplaceTerminalGroups() = %#v", result)
	require.Len(t, coordination.calls, 1)
	require.Equal(t, domain.TerminalGroupCandidate{
		TerminalGroups: candidate, ExpectedSessionRevision: 5, ExpectedCoordinationRevision: 8,
	}, coordination.calls[0].candidate)
	require.Equal(t, candidate, result.Session.TerminalGroups)
}

func TestTerminalGroupReplacementRepairsLegacyDocumentThroughProductionServices(t *testing.T) {
	t.Parallel()

	target := "/Campaigns/legacy-group-repair.json"
	legacy := domain.Session{
		Version: 1, Name: "Legacy group repair",
		Terminals: []domain.Terminal{
			{
				ID: "terminal-a", Name: "A",
				Root: domain.ContentNode{
					ID: "root", Type: domain.NodeFolder, Name: "ROOT",
					Children: []domain.ContentNode{{
						ID: "open-b", Type: domain.NodeCommand, Name: "OPEN B",
						TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-b"},
					}},
				},
			},
			{ID: "terminal-b", Name: "B", Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}}},
		},
	}
	raw, err := domain.EncodeSession(legacy)
	require.NoError(t, err)
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, raw)
	locations := sessionservice.Locations{DocumentsDefault: "/Campaigns"}

	newProductionBoundary := func() (*sessionservice.Service, *App) {
		sessions := sessionservice.NewService(
			sessionservice.NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, locations,
		)
		coordination := controlservice.New(controlservice.Config{
			TerminalCatalog:    sessions,
			TerminalGroupStore: &sessionCommandStateStore{service: sessions},
		})
		return sessions, NewAppWithDependencies(t.Context(), AppDependencies{
			Sessions: sessions, Coordination: coordination,
		})
	}

	sessions, app := newProductionBoundary()
	t.Cleanup(func() { _ = sessions.Shutdown(context.WithoutCancel(t.Context())) })
	opened := app.OpenSession()
	require.True(t, opened.OK, "OpenSession() = %#v", opened)
	require.NotNil(t, opened.Session)
	require.Len(t, opened.Session.TerminalGroups, 2)
	var sourceGroup domain.TerminalGroup
	for _, group := range opened.Session.TerminalGroups {
		if slices.Contains(group.TerminalIDs, "terminal-a") {
			sourceGroup = group
		}
	}
	require.NotEmpty(t, sourceGroup.ID)
	candidate := []domain.TerminalGroup{{
		ID: sourceGroup.ID, Name: sourceGroup.Name, TerminalIDs: []string{"terminal-a", "terminal-b"},
	}}
	status := app.GetRuntimeStatus()
	require.NotNil(t, status.CoordinationState)
	result := app.ReplaceTerminalGroups(TerminalGroupReplacementPayload{
		TerminalGroups:               candidate,
		ExpectedSessionRevision:      status.SavedRevision,
		ExpectedCoordinationRevision: status.CoordinationState.Revision,
	})
	require.True(t, result.OK, "ReplaceTerminalGroups() = %#v", result)
	require.Equal(t, candidate, result.Session.TerminalGroups)
	require.NoError(t, sessions.Shutdown(context.WithoutCancel(t.Context())))

	restartedSessions, restartedApp := newProductionBoundary()
	t.Cleanup(func() { _ = restartedSessions.Shutdown(context.WithoutCancel(t.Context())) })
	reopened := restartedApp.OpenSession()
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.NotNil(t, reopened.Session)
	require.Equal(t, candidate, reopened.Session.TerminalGroups)
	require.Equal(t, legacy.Terminals, reopened.Session.Terminals)
	transition, ok := restartedSessions.LookupTerminalTransition("terminal-a", "open-b")
	require.True(t, ok)
	require.Equal(t, "terminal-b", transition.Target.TerminalID)
}

func TestTerminalGroupReplacementRepairsMultiLinkLegacyFixtureThroughProductionServices(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("tests/fixtures/session-05-cold-storage.json")
	require.NoError(t, err)
	testTerminalGroupReplacementRepairsMultiLinkLegacyDocumentThroughProductionServices(t, raw)
}

func TestTerminalGroupReplacementRepairsExactAuthoredMultiLinkLegacyDocumentThroughProductionServices(t *testing.T) {
	t.Parallel()

	exactPath := os.Getenv("FALLOUT_BUG004_SOURCE")
	if exactPath == "" {
		t.Skip("set FALLOUT_BUG004_SOURCE to run the exact authored-file regression")
	}
	raw, err := os.ReadFile(exactPath)
	if os.IsNotExist(err) {
		t.Skipf("exact BUG-004 source unavailable at %q", exactPath)
	}
	require.NoError(t, err)
	require.Equal(t,
		"b4ca8b89b7d7af32e05a9b598a007e36a747ef59ce3e2bd15a60d0b3f0ec9438",
		fmt.Sprintf("%x", sha256.Sum256(raw)),
	)
	testTerminalGroupReplacementRepairsMultiLinkLegacyDocumentThroughProductionServices(t, raw)
}

func testTerminalGroupReplacementRepairsMultiLinkLegacyDocumentThroughProductionServices(t *testing.T, raw []byte) {
	t.Helper()

	target := "/Campaigns/session-05-cold-storage.json"
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, raw)
	locations := sessionservice.Locations{DocumentsDefault: "/Campaigns"}

	newProductionBoundary := func() (*sessionservice.Service, *App) {
		sessions := sessionservice.NewService(
			sessionservice.NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, locations,
		)
		coordination := controlservice.New(controlservice.Config{
			TerminalCatalog:    sessions,
			TerminalGroupStore: &sessionCommandStateStore{service: sessions},
		})
		return sessions, NewAppWithDependencies(t.Context(), AppDependencies{
			Sessions: sessions, Coordination: coordination,
		})
	}
	groupByMember := func(groups []domain.TerminalGroup, terminalID string) domain.TerminalGroup {
		t.Helper()
		var found domain.TerminalGroup
		for _, group := range groups {
			if slices.Contains(group.TerminalIDs, terminalID) {
				found = group
				break
			}
		}
		require.NotEmpty(t, found.ID, "terminal %q has no group", terminalID)
		return found
	}

	sessions, app := newProductionBoundary()
	t.Cleanup(func() { _ = sessions.Shutdown(context.WithoutCancel(t.Context())) })
	opened := app.OpenSession()
	require.True(t, opened.OK, "OpenSession() = %#v", opened)
	require.NotNil(t, opened.Session)
	require.Len(t, opened.Session.TerminalGroups, 3)
	serviceGroup := groupByMember(opened.Session.TerminalGroups, "t-krel-service")
	emergencyGroup := groupByMember(opened.Session.TerminalGroups, "t-krel-emergency")
	status := app.GetRuntimeStatus()
	require.NotNil(t, status.CoordinationState)
	beforeSessionRevision := status.SavedRevision
	beforeCoordinationRevision := status.CoordinationState.Revision

	partial := app.ReplaceTerminalGroups(TerminalGroupReplacementPayload{
		TerminalGroups: []domain.TerminalGroup{
			{
				ID: serviceGroup.ID, Name: serviceGroup.Name,
				TerminalIDs: []string{"t-krel-service", "t-krel-admin"},
			},
			emergencyGroup,
		},
		ExpectedSessionRevision:      status.SavedRevision,
		ExpectedCoordinationRevision: status.CoordinationState.Revision,
	})
	require.False(t, partial.OK)
	assert.NotContains(t, partial.Error, `command "svc-access-admin"`)
	assert.Contains(t, partial.Error, `command "adm-emergency"`)
	assert.Equal(t, beforeSessionRevision, partial.SessionRevision)
	require.NotNil(t, partial.Session)
	assert.Equal(t, opened.Session.TerminalGroups, partial.Session.TerminalGroups)
	require.NotNil(t, partial.CoordinationState)
	assert.Equal(t, beforeCoordinationRevision, partial.CoordinationState.Revision)

	status = app.GetRuntimeStatus()
	require.NotNil(t, status.CoordinationState)
	assert.Equal(t, beforeSessionRevision, status.SavedRevision)
	assert.Equal(t, beforeCoordinationRevision, status.CoordinationState.Revision)
	complete := []domain.TerminalGroup{{
		ID: serviceGroup.ID, Name: serviceGroup.Name,
		TerminalIDs: []string{"t-krel-service", "t-krel-admin", "t-krel-emergency"},
	}}
	replaced := app.ReplaceTerminalGroups(TerminalGroupReplacementPayload{
		TerminalGroups:               complete,
		ExpectedSessionRevision:      status.SavedRevision,
		ExpectedCoordinationRevision: status.CoordinationState.Revision,
	})
	require.True(t, replaced.OK, "ReplaceTerminalGroups() = %#v", replaced)
	require.NotNil(t, replaced.Session)
	assert.Equal(t, beforeSessionRevision+1, replaced.SessionRevision)
	require.NotNil(t, replaced.CoordinationState)
	assert.Equal(t, beforeCoordinationRevision+1, replaced.CoordinationState.Revision)
	assert.Equal(t, complete, replaced.Session.TerminalGroups)
	assert.Equal(t, opened.Session.Terminals, replaced.Session.Terminals)
	assert.Equal(t, opened.Session.PlayerConfig, replaced.Session.PlayerConfig)
	require.NoError(t, sessions.Shutdown(context.WithoutCancel(t.Context())))

	restartedSessions, restartedApp := newProductionBoundary()
	t.Cleanup(func() { _ = restartedSessions.Shutdown(context.WithoutCancel(t.Context())) })
	reopened := restartedApp.OpenSession()
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.NotNil(t, reopened.Session)
	assert.Equal(t, complete, reopened.Session.TerminalGroups)
	assert.Equal(t, opened.Session.Terminals, reopened.Session.Terminals)
	assert.Equal(t, opened.Session.PlayerConfig, reopened.Session.PlayerConfig)
	_, serviceToAdmin := restartedSessions.LookupTerminalTransition("t-krel-service", "svc-access-admin")
	assert.True(t, serviceToAdmin)
	_, adminToEmergency := restartedSessions.LookupTerminalTransition("t-krel-admin", "adm-emergency")
	assert.True(t, adminToEmergency)
}

func TestTerminalGroupReplacementStaleFailureReturnsLatestProjectionsWithoutPublishing(t *testing.T) {
	recorder := &callRecorder{}
	canonicalSession := terminalGroupApplicationSession([]domain.TerminalGroup{
		{ID: "left", Name: "Left", TerminalIDs: []string{"terminal-a"}},
		{ID: "right", Name: "Right", TerminalIDs: []string{"terminal-b"}},
	})
	canonicalState := &domain.MasterCoordinationState{Revision: 32}
	coordination := &recordingTerminalGroupCoordinationService{
		recordingCoordinationService: recordingCoordinationService{state: canonicalState},
		recorder:                     recorder,
		replacementState:             canonicalState,
		err:                          errors.New("coordination revision changed; review the latest group state"),
	}
	sessions := &loggingSessionCommands{active: sessionservice.ActiveSession{
		Path: "/Campaigns/latest-grouped.json", Session: &canonicalSession,
		RequestedRevision: 17, SavedRevision: 17, SaveState: sessionservice.SaveStateSaved,
	}}
	events := &recordingEventSink{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions, Coordination: coordination, Events: events,
	})

	result := app.ReplaceTerminalGroups(TerminalGroupReplacementPayload{
		TerminalGroups: []domain.TerminalGroup{
			{ID: "combined", Name: "Combined", TerminalIDs: []string{"terminal-a", "terminal-b"}},
		},
		ExpectedSessionRevision: 16, ExpectedCoordinationRevision: 31,
	})
	require.False(t, result.OK)
	require.Contains(t, result.Error, "coordination revision")
	require.Equal(t, uint64(17), result.SessionRevision)
	require.Equal(t, &canonicalSession, result.Session)
	require.Equal(t, canonicalState, result.CoordinationState)
	require.Len(t, coordination.calls, 1)
	require.Equal(t, []string{"coordinator:replace-terminal-groups"}, recorder.Calls())
	require.Empty(t, events.Records(), "stale rejection must not publish canonical-change events")
	status := app.GetRuntimeStatus()
	require.Equal(t, uint64(17), status.RequestedRevision)
	require.Equal(t, uint64(17), status.SavedRevision)
	require.Equal(t, canonicalState, status.CoordinationState)

	result.Session.TerminalGroups[0].TerminalIDs[0] = "mutated-failure"
	result.CoordinationState.Revision = 999
	require.Equal(t, []string{"terminal-a"}, sessions.Snapshot().Session.TerminalGroups[0].TerminalIDs)
	require.Equal(t, uint64(32), app.GetRuntimeStatus().CoordinationState.Revision)
}

func TestTerminalGroupReplacementUsesCanonicalStoreRejectionProjection(t *testing.T) {
	t.Parallel()

	canonicalSession := terminalGroupApplicationSession([]domain.TerminalGroup{
		{ID: "left", Name: "Left", TerminalIDs: []string{"terminal-a"}},
		{ID: "right", Name: "Right", TerminalIDs: []string{"terminal-b"}},
	})
	canonicalState := &domain.MasterCoordinationState{Revision: 44}
	coordination := &recordingTerminalGroupCoordinationService{
		recordingCoordinationService: recordingCoordinationService{state: canonicalState},
		replacementState:             canonicalState,
		mutation: &controlservice.TerminalGroupMutation{
			Revision: 19,
			Session:  canonicalSession,
		},
		err: errors.New(`terminal group candidate invalidates authored transitions: ` +
			`terminal transition command "open-b" in terminal "terminal-a" targets terminal "terminal-b" and crosses groups "left" and "right"; ` +
			`terminal transition command "open-b-backup" in terminal "terminal-a" targets terminal "terminal-b" and crosses groups "left" and "right"`),
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: coordination})

	result := app.ReplaceTerminalGroups(TerminalGroupReplacementPayload{
		TerminalGroups: []domain.TerminalGroup{
			{ID: "combined", Name: "Combined", TerminalIDs: []string{"terminal-a", "terminal-b"}},
		},
		ExpectedSessionRevision: 18, ExpectedCoordinationRevision: 43,
	})

	require.False(t, result.OK)
	require.Contains(t, result.Error, `command "open-b"`)
	require.Contains(t, result.Error, `command "open-b-backup"`)
	require.Contains(t, result.Error, "terminal-a")
	require.Contains(t, result.Error, "terminal-b")
	require.Equal(t, uint64(19), result.SessionRevision)
	require.Equal(t, &canonicalSession, result.Session)
	require.Equal(t, canonicalState, result.CoordinationState)
}

func terminalGroupApplicationSession(groups []domain.TerminalGroup) domain.Session {
	return domain.Session{
		Version: 1,
		Name:    "Terminal group application fixture",
		Terminals: []domain.Terminal{
			{ID: "terminal-a", Name: "A", Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}}},
			{ID: "terminal-b", Name: "B", Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}}},
		},
		TerminalGroups: groups,
	}
}

func TestTerminalActivationValidatesRealDemoLinkAgainstCompleteActiveSession(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("sessions/demo.json")
	require.NoError(t, err)
	target := "/Campaigns/demo-activation.json"
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, raw)
	sessions := sessionservice.NewService(
		sessionservice.NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		sessionservice.Locations{DocumentsDefault: "/Campaigns"},
	)
	t.Cleanup(func() { require.NoError(t, sessions.Shutdown(context.WithoutCancel(t.Context()))) })
	coordination := &recordingTerminalCoordinationService{
		state: &domain.MasterCoordinationState{
			Revision:  1,
			Broadcast: &domain.MasterBroadcastState{ID: "broadcast-demo-activation"},
		},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Sessions: sessions, Coordination: coordination})

	opened := app.OpenSession()
	require.True(t, opened.OK, "OpenSession() = %#v", opened)
	require.Len(t, opened.Session.Terminals, 2)
	source := opened.Session.Terminals[0]
	result := app.RequestTerminalActivation(LiveTerminalPayload{
		TerminalID: source.ID, TerminalName: source.Name, Tree: source.Root,
		HackLevel: source.HackLevel, IntroText: source.IntroText,
	})
	require.True(t, result.OK, "RequestTerminalActivation() = %#v", result)
	require.Len(t, coordination.targets, 1)
	require.Equal(t, "t_demo2", coordination.targets[0].Tree.Children[4].TerminalTransition.TargetTerminalID)
}

func TestApplicationUnwindsPartialStartup(t *testing.T) {
	recorder := &callRecorder{}
	player := &recordingPlayerServer{
		recorder: recorder,
		info:     domain.ServerInfo{IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690"},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Player: player,
		Events: &recordingEventSink{recorder: recorder, err: errors.New("bridge unavailable")},
		Desktop: &recordingDesktop{
			recorder: recorder,
		},
	})

	err := app.Start(t.Context())
	require.Falsef(t, err == nil || !strings.Contains(err.Error(), "bridge"),
		"Start() error = %v, want actionable bridge error", err)
	{

		got, want := recorder.Calls(), []string{"player:start", "event:server-info", "player:stop"}
		require.Falsef(t, !cmp.Equal(got, want),
			"partial-start calls = %v, want %v", got, want)
	}

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.ServerInfo != nil || !strings.Contains(status.StartupError, "bridge"),
		"failure status = %#v", status)

}

func TestApplicationShutdownIsReverseOrderedAndIdempotent(t *testing.T) {
	recorder := &callRecorder{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: &recordingSessionService{recorder: recorder},
		Player: &recordingPlayerServer{
			recorder: recorder,
			info:     domain.ServerInfo{IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690"},
		},
		Events:  &recordingEventSink{recorder: recorder},
		Desktop: &recordingDesktop{recorder: recorder},
	})
	{
		err := app.Start(t.Context())
		require.Falsef(t, err != nil,
			"Start() error = %v", err)
	}

	recorder.Reset()
	{
		err := app.Shutdown(t.Context())
		require.Falsef(t, err != nil,
			"Shutdown() error = %v", err)
	}
	{

		err := app.Shutdown(t.Context())
		require.Falsef(t, err != nil,
			"second Shutdown() error = %v", err)
	}
	{

		got, want := recorder.Calls(), []string{"player:stop", "session:shutdown", "desktop:close"}
		require.Falsef(t, !cmp.Equal(got, want),
			"shutdown calls = %v, want %v", got, want)
	}

}

func TestApplicationStartupFailureCleansBoundedResourcesExactlyOnce(t *testing.T) {
	recorder := &callRecorder{}
	player := &recordingPlayerServer{
		recorder: recorder,
		info:     domain.ServerInfo{IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690"},
	}
	sessions := &recordingSessionService{recorder: recorder}
	publicAccess := &recordingPublicAccessCore{
		recorder: recorder,
		snapshot: tunnelservice.PublicAccessSnapshot{
			Preferences: tunnelservice.DefaultPublicAccessPreferences(),
			Status: tunnelservice.PublicAccessStatus{
				State: tunnelservice.LifecycleReady, PublicURL: "https://public.example",
			},
		},
	}
	live := &recordingLiveService{}
	coordination := &recordingBroadcastLifecycleService{}
	const cleanupTimeout = 50 * time.Millisecond
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions, Live: live, Coordination: coordination,
		Player: player, PublicAccess: publicAccess,
		Events:          &recordingEventSink{recorder: recorder, err: errors.New("bridge unavailable")},
		ShutdownTimeout: cleanupTimeout,
	})

	startedAt := time.Now()
	err := app.Start(t.Context())
	require.ErrorContains(t, err, "bridge unavailable")
	require.NoError(t, app.Shutdown(context.Background()))

	assert.Equal(t, []string{
		"player:start", "public:initialize", "event:server-info",
		"public:shutdown", "player:stop", "session:shutdown",
	}, recorder.Calls())
	assert.Equal(t, 1, publicAccess.shutdowns)
	assert.Equal(t, 1, live.clearCalls)
	assert.Equal(t, 1, coordination.shutdownCalls)
	require.Len(t, publicAccess.shutdownContexts, 1)
	require.Len(t, player.stopContexts, 1)
	require.Len(t, sessions.shutdownContexts, 1)
	requireCleanupContextBounded(t, publicAccess.shutdownContexts[0], startedAt, cleanupTimeout)
	requireCleanupContextBounded(t, player.stopContexts[0], startedAt, cleanupTimeout)
	requireCleanupContextBounded(t, sessions.shutdownContexts[0], startedAt, cleanupTimeout)
}

func TestApplicationNormalCloseCleansConnectedPlayersPublicAccessAndProcessesExactlyOnce(t *testing.T) {
	recorder := &callRecorder{}
	player := &recordingPlayerServer{
		recorder: recorder,
		info: domain.ServerInfo{
			IP: "127.0.0.1", Port: 3690,
			URL: "http://127.0.0.1:3690", LocalURL: "http://127.0.0.1:3690",
		},
	}
	sessions := &recordingSessionService{recorder: recorder}
	publicAccess := &recordingPublicAccessCore{
		recorder: recorder,
		snapshot: tunnelservice.PublicAccessSnapshot{
			Preferences: tunnelservice.DefaultPublicAccessPreferences(),
			Status: tunnelservice.PublicAccessStatus{
				State: tunnelservice.LifecycleReady, PublicURL: "https://public.example",
			},
		},
	}
	live := &recordingLiveService{}
	coordination := &recordingBroadcastLifecycleService{}
	const cleanupTimeout = 50 * time.Millisecond
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions, Live: live, Coordination: coordination,
		Player: player, PublicAccess: publicAccess,
		Events: &recordingEventSink{recorder: recorder}, Desktop: &recordingDesktop{recorder: recorder},
		ShutdownTimeout: cleanupTimeout,
	})
	require.NoError(t, app.Start(t.Context()))
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		require.NoError(t, app.Shutdown(cleanupContext))
	})
	app.updateClientCount(2)
	require.Equal(t, 2, app.GetRuntimeStatus().ClientCount)
	recorder.Reset()

	startedAt := time.Now()
	require.NoError(t, app.Shutdown(context.Background()))
	require.NoError(t, app.Shutdown(context.Background()))

	assert.Equal(t, []string{
		"public:shutdown", "player:stop", "session:shutdown", "desktop:close",
	}, recorder.Calls())
	assert.Equal(t, 1, publicAccess.shutdowns)
	assert.Equal(t, 1, live.clearCalls)
	assert.Equal(t, 1, coordination.shutdownCalls)
	require.Len(t, publicAccess.shutdownContexts, 1)
	require.Len(t, player.stopContexts, 1)
	require.Len(t, sessions.shutdownContexts, 1)
	requireCleanupContextBounded(t, publicAccess.shutdownContexts[0], startedAt, cleanupTimeout)
	requireCleanupContextBounded(t, player.stopContexts[0], startedAt, cleanupTimeout)
	requireCleanupContextBounded(t, sessions.shutdownContexts[0], startedAt, cleanupTimeout)
}

func requireCleanupContextBounded(t *testing.T, ctx context.Context, startedAt time.Time, timeout time.Duration) {
	t.Helper()
	require.NotNil(t, ctx)
	deadline, ok := ctx.Deadline()
	require.True(t, ok, "cleanup context must have a deadline")
	assert.LessOrEqual(t, deadline.Sub(startedAt), timeout+250*time.Millisecond,
		"cleanup context must be bounded by the configured shutdown timeout")
}

func TestApplicationPlayerStartFailureNeverReportsReady(t *testing.T) {
	recorder := &callRecorder{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Player:  &recordingPlayerServer{recorder: recorder, startErr: errors.New("port 3690 is already in use")},
		Events:  &recordingEventSink{recorder: recorder},
		Desktop: &recordingDesktop{recorder: recorder},
	})

	err := app.Start(t.Context())
	require.Falsef(t, err == nil || !strings.Contains(err.Error(), "3690"),
		"Start() error = %v, want port detail", err)
	{

		got, want := recorder.Calls(), []string{"player:start"}
		require.Falsef(t, !cmp.Equal(got, want),
			"failed startup calls = %v, want %v", got, want)
	}
	{

		status := app.GetRuntimeStatus()
		require.Falsef(t, status.ServerInfo != nil || !strings.Contains(status.StartupError, "3690"),
			"failure status = %#v", status)
	}

}

func TestBridgeRejectsInvalidLivePayloadsBeforeMutation(t *testing.T) {
	coordination := &recordingTerminalCoordinationService{
		state: &domain.MasterCoordinationState{
			Revision: 1, Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1"},
		},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: coordination})

	activationResult := app.RequestTerminalActivation(LiveTerminalPayload{
		TerminalID:   "terminal-1",
		TerminalName: "Overseer",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeCommand, Name: "not a folder", Text: "invalid root",
		},
		HackLevel: 1,
	})
	require.Falsef(t, activationResult.OK || activationResult.Error == "",
		"RequestTerminalActivation(invalid) = %#v, want structured validation error", activationResult)

	updateResult := app.UpdateLiveTerminal(LiveUpdatePayload{
		Tree: domain.ContentNode{ID: "root", Type: "script", Name: "unsupported"},
	})
	require.Falsef(t, updateResult.OK || updateResult.Error == "",
		"UpdateLiveTerminal(invalid) = %#v, want structured validation error", updateResult)
	require.Falsef(t, len(coordination.targets) != 0 || coordination.updateCalls != 0,
		"invalid live payloads reached coordinator: activations=%d updates=%d", len(coordination.targets), coordination.updateCalls)

}

func TestRuntimeStatusReturnsCompleteDetachedSnapshot(t *testing.T) {
	app := NewAppWithDependencies(t.Context(), AppDependencies{})
	app.serverInfo = &domain.ServerInfo{
		IP: "192.0.2.10", Port: 3690, URL: "http://192.0.2.10:3690",
	}
	app.clientCount = 4
	app.hackState = &domain.PublicHackState{
		Level: 2, AttemptsMax: 4, AttemptsLeft: 3, Log: []string{"ENTRY DENIED"},
		Columns: []domain.HackColumn{{Addresses: []string{"0xF000"}, Text: "...."}},
	}
	app.saveState = "saving"
	app.requestedRevision = 8
	app.savedRevision = 7

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.ServerInfo == nil || status.ServerInfo.URL != "http://192.0.2.10:3690",
		"RuntimeStatus.ServerInfo = %#v", status.ServerInfo)
	require.Falsef(t, status.ClientCount != 4 || status.HackState == nil || status.HackState.AttemptsLeft != 3,
		"RuntimeStatus bridge state = %#v", status)
	require.Falsef(t, status.SaveState != "saving" || status.RequestedRevision != 8 || status.SavedRevision != 7,
		"RuntimeStatus save state = %#v", status)

	status.ServerInfo.URL = "mutated"
	status.HackState.Log[0] = "mutated"
	status.HackState.Columns[0].Addresses[0] = "mutated"
	again := app.GetRuntimeStatus()
	require.Falsef(t, again.ServerInfo.URL == "mutated" || again.HackState.Log[0] == "mutated" || again.HackState.Columns[0].Addresses[0] == "mutated",
		"GetRuntimeStatus returned aliases into canonical state: %#v", again)

}

func TestCoordinationBridgeAddsCharacterStartsBroadcastAndReplaysDetachedState(t *testing.T) {
	recorder := &callRecorder{}
	events := &recordingEventSink{recorder: recorder}
	service := &recordingCoordinationService{
		state: &domain.MasterCoordinationState{Revision: 4},
		addState: &domain.MasterCoordinationState{
			Revision: 5,
			Roster: []domain.MasterRosterEntry{{
				ID: "character-1", Name: "Mara", Intelligence: 8, HackerPerkAvailable: false,
			}},
		},
		startState: &domain.MasterCoordinationState{
			Revision: 6,
			Roster:   []domain.MasterRosterEntry{{ID: "character-1", Name: "Mara"}},
			Broadcast: &domain.MasterBroadcastState{
				ID: "broadcast-1",
			},
		},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: service, Events: events})

	initial := app.GetRuntimeStatus()
	require.Falsef(t, initial.CoordinationState == nil || initial.CoordinationState.Revision != 4,
		"initial coordination status = %#v, want replayable revision 4", initial.CoordinationState)

	hackerUnavailable := false
	added := app.AddCharacter(CharacterCreatePayload{
		Name: "  Mara  ", Intelligence: 8, HackerPerkAvailable: &hackerUnavailable, ExpectedRevision: 4,
	})
	require.Falsef(t, !added.OK || added.Error != "" || added.State == nil || added.State.Revision != 5 || added.State.Roster[0].Intelligence != 8 || added.State.Roster[0].HackerPerkAvailable,
		"AddCharacter() = %#v, want accepted revision 5", added)
	{

		got, want := service.addPayloads, []domain.CharacterCreatePayload{{
			Name: "Mara", Intelligence: 8, HackerPerkAvailable: false, ExpectedRevision: 4,
		}}
		require.Falsef(t, !cmp.Equal(got, want),
			"coordinator AddCharacter calls = %v, want %v", got, want)
	}

	added.State.Roster[0].Name = "mutated result"

	started := app.StartBroadcast()
	require.Falsef(t, !started.OK || started.Error != "" || started.State == nil || started.State.Broadcast == nil || started.State.Broadcast.ID != "broadcast-1",
		"StartBroadcast() = %#v, want accepted broadcast", started)

	started.State.Roster[0].Name = "mutated start result"

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.CoordinationState == nil || status.CoordinationState.Revision != 6 || status.CoordinationState.Roster[0].Name != "Mara",
		"detached coordination status = %#v", status.CoordinationState)

	records := events.Records()
	require.Falsef(t, len(records) != 2 || records[0].Name != "coordination-state" || records[1].Name != "coordination-state",
		"coordination events = %#v, want add then start snapshots", records)

	last, ok := records[1].Payload.(*domain.MasterCoordinationState)
	require.Falsef(t, !ok || last == nil || last.Broadcast == nil || last.Broadcast.ID != "broadcast-1",
		"last coordination event = %#v", records[1])

	last.Roster[0].Name = "mutated event"
	{
		replay := app.GetRuntimeStatus().CoordinationState
		require.Falsef(t, replay == nil || replay.Roster[0].Name != "Mara",
			"event payload aliased runtime status: %#v", replay)
	}

}

func TestCoordinationBridgeRejectsInvalidOrFailedCommandsWithoutPartialState(t *testing.T) {
	recorder := &callRecorder{}
	service := &recordingCoordinationService{
		state:    &domain.MasterCoordinationState{Revision: 9, Roster: []domain.MasterRosterEntry{{ID: "character-1", Name: "Mara"}}},
		addErr:   errors.New("roster unavailable"),
		startErr: errors.New("broadcast already active"),
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: service,
		Events:       &recordingEventSink{recorder: recorder},
	})

	hackerAvailable := true
	invalidPayloads := []CharacterCreatePayload{
		{Name: "   ", Intelligence: 5, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 9},
		{Name: strings.Repeat("x", 81), Intelligence: 5, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 9},
		{Name: "Boone", Intelligence: 0, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 9},
		{Name: "Boone", Intelligence: 11, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 9},
		{Name: "Boone", Intelligence: 5, HackerPerkAvailable: nil, ExpectedRevision: 9},
	}
	for index, payload := range invalidPayloads {
		invalid := app.AddCharacter(payload)
		require.Falsef(t, invalid.OK || invalid.Error == "" || len(service.addPayloads) != 0,
			"AddCharacter(invalid %d) = %#v, calls = %v", index, invalid, service.addPayloads)
	}

	failedAdd := app.AddCharacter(CharacterCreatePayload{
		Name: "Boone", Intelligence: 5, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 9,
	})
	require.Falsef(t, failedAdd.OK || !strings.Contains(failedAdd.Error, "roster"),
		"AddCharacter(failed) = %#v", failedAdd)

	failedStart := app.StartBroadcast()
	require.Falsef(t, failedStart.OK || !strings.Contains(failedStart.Error, "already"),
		"StartBroadcast(failed) = %#v", failedStart)

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.CoordinationState == nil || status.CoordinationState.Revision != 9 || len(status.CoordinationState.Roster) != 1,
		"failed commands partially changed status: %#v", status.CoordinationState)
	{

		records := recorder.Calls()
		require.Falsef(t, len(records) != 0,
			"failed commands emitted coordination events: %v", records)
	}

}

func TestBroadcastLifecycleBridgeEndsRestartsReplaysAndDisposesWithoutDurableMutation(t *testing.T) {
	recorder := &callRecorder{}
	activeID := "terminal-1"
	coordination := &recordingBroadcastLifecycleService{
		state: &domain.MasterCoordinationState{
			Revision: 80, Roster: []domain.MasterRosterEntry{{ID: "character-1", Name: "Mara"}},
			Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1", ActiveTerminalID: &activeID},
		},
		endState: &domain.MasterCoordinationState{Revision: 81, Roster: []domain.MasterRosterEntry{{ID: "character-1", Name: "Mara"}}},
	}
	coordination.startState = &domain.MasterCoordinationState{
		Revision: 82, Roster: []domain.MasterRosterEntry{{ID: "character-1", Name: "Mara"}},
		Broadcast: &domain.MasterBroadcastState{ID: "broadcast-2"},
	}
	durable := &recordingSessionService{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: coordination, Sessions: durable, Events: &recordingEventSink{recorder: recorder},
	})

	ended := app.EndBroadcast()
	require.Falsef(t, !ended.OK || ended.Error != "" || ended.State == nil || ended.State.Broadcast != nil || ended.State.Revision != 81,
		"EndBroadcast() = %#v", ended)

	restarted := app.StartBroadcast()
	require.Falsef(t, !restarted.OK || restarted.State == nil || restarted.State.Broadcast == nil || restarted.State.Broadcast.ID != "broadcast-2",
		"second StartBroadcast() = %#v", restarted)
	{

		status := app.GetRuntimeStatus()
		require.Falsef(t, status.CoordinationState == nil || status.CoordinationState.Broadcast == nil || status.CoordinationState.Broadcast.ID != "broadcast-2",
			"broadcast lifecycle replay status = %#v", status)
	}
	require.Falsef(t, coordination.endCalls != 1 || coordination.startCalls != 1,
		"lifecycle calls end/start = %d/%d", coordination.endCalls, coordination.startCalls)
	{

		err := app.Shutdown(t.Context())
		require.False(t, err != nil,
			err)
	}
	require.Falsef(t, coordination.shutdownCalls != 1,
		"coordination shutdown calls = %d, want 1", coordination.shutdownCalls)
	require.Falsef(t, durable.shutdownCalls != 1,
		"durable session shutdown calls = %d, want 1 without mutation commands", durable.shutdownCalls)

}

func TestCoordinationBridgeValidatesSessionAndAssignmentCorrections(t *testing.T) {
	recorder := &callRecorder{}
	initial := &domain.MasterCoordinationState{
		Revision: 20,
		Roster: []domain.MasterRosterEntry{
			{ID: "character-1", Name: "Mara"},
			{ID: "character-2", Name: "Boone"},
		},
		Sessions: []domain.MasterSessionEntry{
			{ID: "session-1", FallbackName: "DEVICE 1", Connected: true},
			{ID: "session-2", FallbackName: "DEVICE 2", Connected: true},
		},
		Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1"},
	}
	service := &recordingCorrectionCoordinationService{
		state: initial,
	}
	events := &recordingEventSink{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: service,
		Sessions:     &recordingSessionService{recorder: recorder},
		Events:       events,
	})

	commands := []struct {
		name string
		run  func() CoordinationCommandResult
	}{
		{"rename-session", func() CoordinationCommandResult {
			return app.RenameLogicalSession(LogicalSessionRenamePayload{SessionID: "session-1", FallbackName: "  TABLET LEFT  "})
		}},
		{"assign-character", func() CoordinationCommandResult {
			return app.AssignCharacter(AssignmentPayload{SessionID: "session-1", CharacterID: "character-1"})
		}},
		{"release-character", func() CoordinationCommandResult { return app.ReleaseCharacter("session-1") }},
		{"move-character", func() CoordinationCommandResult {
			return app.MoveCharacter(MoveCharacterPayload{CharacterID: "character-1", ToSessionID: "session-2"})
		}},
	}
	for _, command := range commands {
		service.nextRevision++
		result := command.run()
		require.Falsef(t, !result.OK || result.Error != "" || result.State == nil || result.State.Revision != uint64(20+service.nextRevision),
			"%s result = %#v", command.name, result)

		result.State.Roster[0].Name = "mutated result"
		{
			got := app.GetRuntimeStatus().CoordinationState.Roster[0].Name
			require.Falsef(t, got == "mutated result",
				"%s returned an aliased coordination state", command.name)
		}

	}

	wantCalls := []string{
		"rename-session:session-1:TABLET LEFT",
		"assign-character:session-1:character-1",
		"release-character:session-1",
		"move-character:character-1:session-2",
	}
	require.Falsef(t, !cmp.Equal(service.calls, wantCalls),
		"coordination correction calls = %v, want %v", service.calls, wantCalls)
	{

		got := recorder.Calls()
		require.Falsef(t, len(got) != len(commands),
			"coordination event/save calls = %v, want only %d coordination events", got, len(commands))
	}

	for _, call := range recorder.Calls() {
		require.Falsef(t, call != "event:coordination-state",
			"coordination correction touched durable session path: %v", recorder.Calls())

	}

	eventState, ok := events.Records()[0].Payload.(*domain.MasterCoordinationState)
	require.Falsef(t, !ok || eventState == nil,
		"first correction event payload = %#v", events.Records()[0].Payload)

	eventState.Roster[0].Name = "mutated event"
	{
		got := app.GetRuntimeStatus().CoordinationState.Roster[0].Name
		require.False(t, got == "mutated event",
			"coordination event payload aliases replayable runtime status")
	}

	before := app.GetRuntimeStatus().CoordinationState
	service.failCommand = "move-character"
	service.commandErr = errors.New("destination session already has a character")
	rejected := app.MoveCharacter(MoveCharacterPayload{CharacterID: "character-1", ToSessionID: "session-2"})
	require.Falsef(t, rejected.OK || !strings.Contains(rejected.Error, "already") || !cmp.Equal(rejected.State, before),
		"conflicting MoveCharacter() = %#v, want unchanged authoritative snapshot", rejected)
	require.False(t, !cmp.Equal(app.GetRuntimeStatus().CoordinationState, before),
		"conflicting correction changed replayable coordination state")

	invalidCalls := len(service.calls)
	invalid := []CoordinationCommandResult{
		app.RenameLogicalSession(LogicalSessionRenamePayload{SessionID: "session-1", FallbackName: "  "}),
		app.AssignCharacter(AssignmentPayload{SessionID: "", CharacterID: "character-1"}),
		app.ReleaseCharacter("   "),
		app.MoveCharacter(MoveCharacterPayload{CharacterID: "character-1", ToSessionID: ""}),
	}
	for index, result := range invalid {
		require.Falsef(t, result.OK || result.Error == "",
			"invalid correction %d = %#v, want validation refusal", index, result)

	}
	require.Falsef(t, len(service.calls) != invalidCalls,
		"invalid payloads reached coordinator: %v", service.calls[invalidCalls:])

}

func TestCoordinationBridgeRoutesCompleteUpdateAndDeletePayloads(t *testing.T) {
	recorder := &callRecorder{}
	service := &recordingCorrectionCoordinationService{
		state: &domain.MasterCoordinationState{
			Revision: 20,
			Roster: []domain.MasterRosterEntry{
				{ID: "character-1", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true},
				{ID: "character-2", Name: "Boone", Intelligence: 3},
			},
		},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: service,
		Events:       &recordingEventSink{recorder: recorder},
	})

	hackerUnavailable := false
	service.nextRevision = 1
	updated := app.UpdateCharacter(CharacterUpdatePayload{
		CharacterID: "character-1", Name: "  Mara Voss  ", Intelligence: 10,
		HackerPerkAvailable: &hackerUnavailable, ExpectedRevision: 20,
	})
	require.True(t, updated.OK)
	require.Empty(t, updated.Error)
	require.NotNil(t, updated.State)
	require.Equal(t, uint64(21), updated.State.Revision)
	require.Equal(t, []domain.CharacterUpdatePayload{{
		CharacterID: "character-1", Name: "Mara Voss", Intelligence: 10,
		HackerPerkAvailable: false, ExpectedRevision: 20,
	}}, service.updatePayloads)

	service.nextRevision = 2
	deleted := app.DeleteCharacter(CharacterDeletePayload{
		CharacterID: "character-2", ExpectedRevision: 21,
	})
	require.True(t, deleted.OK)
	require.Empty(t, deleted.Error)
	require.NotNil(t, deleted.State)
	require.Equal(t, uint64(22), deleted.State.Revision)
	require.Equal(t, []domain.CharacterDeletePayload{{
		CharacterID: "character-2", ExpectedRevision: 21,
	}}, service.deletePayloads)
	require.Equal(t, []string{
		"update-character:character-1:Mara Voss:10:false:20",
		"delete-character:character-2:21",
	}, service.calls)
	require.Equal(t, []string{"event:coordination-state", "event:coordination-state"}, recorder.Calls())
}

func TestCoordinationBridgeRejectsInvalidCompleteRosterMutationsBeforeCoordinator(t *testing.T) {
	service := &recordingCorrectionCoordinationService{
		state: &domain.MasterCoordinationState{
			Revision: 20,
			Roster:   []domain.MasterRosterEntry{{ID: "character-1", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true}},
		},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: service})
	hackerAvailable := true

	invalid := []CoordinationCommandResult{
		app.UpdateCharacter(CharacterUpdatePayload{Name: "Mara", Intelligence: 8, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 20}),
		app.UpdateCharacter(CharacterUpdatePayload{CharacterID: "character-1", Name: "   ", Intelligence: 8, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 20}),
		app.UpdateCharacter(CharacterUpdatePayload{CharacterID: "character-1", Name: strings.Repeat("x", 81), Intelligence: 8, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 20}),
		app.UpdateCharacter(CharacterUpdatePayload{CharacterID: "character-1", Name: "Mara", Intelligence: 0, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 20}),
		app.UpdateCharacter(CharacterUpdatePayload{CharacterID: "character-1", Name: "Mara", Intelligence: 11, HackerPerkAvailable: &hackerAvailable, ExpectedRevision: 20}),
		app.UpdateCharacter(CharacterUpdatePayload{CharacterID: "character-1", Name: "Mara", Intelligence: 8, ExpectedRevision: 20}),
		app.DeleteCharacter(CharacterDeletePayload{ExpectedRevision: 20}),
	}
	for index, result := range invalid {
		require.Falsef(t, result.OK || result.Error == "", "invalid roster mutation %d = %#v", index, result)
		require.NotNil(t, result.State, "validation failure must return authoritative state")
		require.Equal(t, uint64(20), result.State.Revision)
	}
	require.Empty(t, service.updatePayloads)
	require.Empty(t, service.deletePayloads)
	require.Empty(t, service.calls)
}

func TestCoordinationBridgeRosterMutationFailureReturnsAuthoritativeState(t *testing.T) {
	recorder := &callRecorder{}
	canonical := &domain.MasterCoordinationState{
		Revision: 32,
		Roster:   []domain.MasterRosterEntry{{ID: "character-1", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true}},
	}
	service := &recordingCorrectionCoordinationService{
		state:       canonical,
		failCommand: "update-character",
		commandErr:  errors.New("player config changed on disk"),
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: service,
		Events:       &recordingEventSink{recorder: recorder},
	})
	hackerUnavailable := false

	result := app.UpdateCharacter(CharacterUpdatePayload{
		CharacterID: "character-1", Name: "Mara Voss", Intelligence: 10,
		HackerPerkAvailable: &hackerUnavailable, ExpectedRevision: 32,
	})
	require.False(t, result.OK)
	require.ErrorContains(t, errors.New(result.Error), "changed on disk")
	require.Equal(t, canonical, result.State)
	require.Equal(t, canonical, app.GetRuntimeStatus().CoordinationState)
	require.Empty(t, recorder.Calls(), "failed mutation must not publish")
}

func TestCoordinationBridgeValidatesAndPublishesActiveControllerReassignment(t *testing.T) {
	order := &callRecorder{}
	firstID := domain.LogicalSessionID("session-1")
	secondID := domain.LogicalSessionID("session-2")
	service := &recordingCorrectionCoordinationService{
		state: &domain.MasterCoordinationState{
			Revision: 30,
			Sessions: []domain.MasterSessionEntry{
				{ID: firstID, FallbackName: "DEVICE 1", Connected: true, Character: &domain.PlayerCharacter{ID: "character-1", Name: "Mara"}, Role: domain.PlayerRoleActive},
				{ID: secondID, FallbackName: "DEVICE 2", Connected: true, Character: &domain.PlayerCharacter{ID: "character-2", Name: "Boone"}, Role: domain.PlayerRoleObserver},
			},
			Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1", ControllerSessionID: &firstID},
		},
		order: order,
	}
	events := &recordingEventSink{recorder: order}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: service, Events: events})

	result := app.SetActiveController(string(secondID))
	require.Falsef(t, !result.OK || result.Error != "" || result.State == nil || result.State.Revision != 31,
		"SetActiveController(second) = %#v", result)
	require.Falsef(t, result.State.Broadcast == nil || result.State.Broadcast.ControllerSessionID == nil || *result.State.Broadcast.ControllerSessionID != secondID,
		"reassigned broadcast = %#v, want controller %q", result.State.Broadcast, secondID)
	{

		got := masterSessionEntryForAppTest(t, result.State, firstID).Role
		require.Falsef(t, got != domain.PlayerRoleObserver,
			"former controller role = %q, want observer", got)
	}
	{

		got := masterSessionEntryForAppTest(t, result.State, secondID).Role
		require.Falsef(t, got != domain.PlayerRoleActive,
			"new controller role = %q, want active", got)
	}
	{

		got, want := order.Calls(), []string{"coordinator:set-controller:session-2", "event:coordination-state"}
		require.Falsef(t, !cmp.Equal(got, want),
			"reassignment order = %v, want %v", got, want)
	}

	result.State.Sessions[0].FallbackName = "mutated result"
	eventState, ok := events.Records()[0].Payload.(*domain.MasterCoordinationState)
	require.Falsef(t, !ok || eventState == nil,
		"controller event = %#v", events.Records())

	eventState.Sessions[1].FallbackName = "mutated event"
	status := app.GetRuntimeStatus().CoordinationState
	require.Falsef(t, status.Sessions[0].FallbackName == "mutated result" || status.Sessions[1].FallbackName == "mutated event",
		"controller result/event alias replay status: %#v", status)

	before := app.GetRuntimeStatus().CoordinationState
	service.failCommand = "set-active-controller"
	service.commandErr = errors.New("controller must be connected and assigned")
	rejected := app.SetActiveController(string(firstID))
	require.Falsef(t, rejected.OK || !strings.Contains(rejected.Error, "connected") || !cmp.Equal(rejected.State, before),
		"ineligible SetActiveController() = %#v", rejected)
	require.False(t, len(events.Records()) != 1 || !cmp.Equal(app.GetRuntimeStatus().CoordinationState, before),
		"ineligible reassignment published or changed the authoritative snapshot")

	callsBeforeBlank := len(service.calls)
	blank := app.SetActiveController("   ")
	require.Falsef(t, blank.OK || blank.Error == "" || len(service.calls) != callsBeforeBlank,
		"blank SetActiveController() = %#v calls=%v", blank, service.calls)

}

func TestCoordinationStatusReplaysDisconnectedControllerWithoutChangingClaimOrRole(t *testing.T) {
	controllerID := domain.LogicalSessionID("session-controller")
	characterID := domain.CharacterID("character-mara")
	state := &domain.MasterCoordinationState{
		Revision: 41,
		Roster:   []domain.MasterRosterEntry{{ID: characterID, Name: "Mara", ClaimedBySessionID: &controllerID}},
		Sessions: []domain.MasterSessionEntry{{
			ID: controllerID, FallbackName: "TABLET LEFT", Connected: false,
			Character: &domain.PlayerCharacter{ID: characterID, Name: "Mara"}, Role: domain.PlayerRoleActive,
		}},
		Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1", ControllerSessionID: &controllerID},
	}
	events := &recordingEventSink{recorder: &callRecorder{}}
	service := &recordingCoordinationService{state: state}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: service, Events: events})

	initial := app.GetRuntimeStatus().CoordinationState
	assertDisconnectedControllerSnapshot(t, initial, controllerID, characterID)
	app.publishCoordinationState(state)
	replay := app.GetRuntimeStatus().CoordinationState
	assertDisconnectedControllerSnapshot(t, replay, controllerID, characterID)
	records := events.Records()
	require.Falsef(t, len(records) != 1,
		"presence events = %#v, want one coordination snapshot", records)

	published, ok := records[0].Payload.(*domain.MasterCoordinationState)
	require.Falsef(t, !ok,
		"presence payload = %#v", records[0].Payload)

	assertDisconnectedControllerSnapshot(t, published, controllerID, characterID)
	published.Sessions[0].Connected = true
	published.Roster[0].Name = "mutated"
	assertDisconnectedControllerSnapshot(t, app.GetRuntimeStatus().CoordinationState, controllerID, characterID)
}

func assertDisconnectedControllerSnapshot(t *testing.T, state *domain.MasterCoordinationState, sessionID domain.LogicalSessionID, characterID domain.CharacterID) {
	t.Helper()
	require.Falsef(t, state == nil || state.Broadcast == nil || state.Broadcast.ControllerSessionID == nil || *state.Broadcast.ControllerSessionID != sessionID,
		"disconnected controller broadcast = %#v", state)

	session := masterSessionEntryForAppTest(t, state, sessionID)
	require.Falsef(t, session.Connected || session.Role != domain.PlayerRoleActive || session.Character == nil || session.Character.ID != characterID,
		"disconnected controller session = %#v", session)
	require.Falsef(t, len(state.Roster) != 1 || state.Roster[0].ClaimedBySessionID == nil || *state.Roster[0].ClaimedBySessionID != sessionID,
		"disconnected controller claim = %#v", state.Roster)

}

func masterSessionEntryForAppTest(t *testing.T, state *domain.MasterCoordinationState, sessionID domain.LogicalSessionID) domain.MasterSessionEntry {
	t.Helper()
	for _, session := range state.Sessions {
		if session.ID == sessionID {
			return session
		}
	}
	assert.Fail(t, "assertion failed", "coordination state has no session %q", sessionID)
	return domain.MasterSessionEntry{}
}

func TestCoordinationBridgeOrdersTerminalActivationClearAndUpdateWithoutLegacyMutation(t *testing.T) {
	recorder := &callRecorder{}
	initial := &domain.MasterCoordinationState{Revision: 40, Broadcast: &domain.MasterBroadcastState{ID: "broadcast-terminal-bridge"}}
	coordination := &recordingTerminalCoordinationService{
		state: initial,
		order: recorder,
	}
	legacy := &recordingLiveService{
		setState: &domain.PublicLiveState{TerminalID: "legacy-terminal"}, updateState: &domain.PublicLiveState{TerminalID: "legacy-terminal"},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: coordination, Live: legacy,
		Player: &recordingPlayerServer{recorder: recorder}, Events: &recordingEventSink{recorder: recorder},
	})
	app.hackState = &domain.PublicHackState{Level: 1, AttemptsMax: 4, AttemptsLeft: 2}
	tree := domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{{ID: "docs", Type: domain.NodeFolder, Name: "DOCS"}},
	}

	activated := app.RequestTerminalActivation(LiveTerminalPayload{
		TerminalID: "  terminal-1  ", TerminalName: "  Overseer  ", Tree: tree, HackLevel: 2, IntroText: "WELCOME",
	})
	require.Falsef(t, !activated.OK || activated.Error != "" || activated.Status != "activated" || activated.SwitchID != "" || activated.State == nil || activated.State.Revision != 41,
		"RequestTerminalActivation() = %#v", activated)
	require.Falsef(t, activated.State.Broadcast == nil || activated.State.Broadcast.ActiveTerminalID == nil || *activated.State.Broadcast.ActiveTerminalID != "terminal-1",
		"activation authoritative state = %#v", activated.State)
	require.Falsef(t, len(coordination.targets) != 1 || coordination.targets[0].TerminalID != "terminal-1" || coordination.targets[0].TerminalName != "Overseer",
		"activation payload was not trimmed before coordinator: %#v", coordination.targets)

	intro := "UPDATED INTRO"
	updated := app.UpdateLiveTerminal(LiveUpdatePayload{Tree: tree, IntroText: &intro})
	require.Falsef(t, !updated.OK || updated.Error != "" || updated.State == nil || updated.State.Revision != 42,
		"UpdateLiveTerminal() = %#v, want authoritative revision 42", updated)
	require.Falsef(t, coordination.updateCalls != 1 || coordination.updateIntro == nil || *coordination.updateIntro != intro || !cmp.Equal(coordination.updateTree, tree),
		"ordered update payload = calls %d tree %#v intro %#v", coordination.updateCalls, coordination.updateTree, coordination.updateIntro)

	cleared := app.RequestTerminalClear()
	require.Falsef(t, !cleared.OK || cleared.Error != "" || cleared.Status != "cleared" || cleared.SwitchID != "" || cleared.State == nil || cleared.State.Revision != 43,
		"RequestTerminalClear() = %#v", cleared)
	require.Falsef(t, cleared.State.Broadcast == nil || cleared.State.Broadcast.ActiveTerminalID != nil,
		"clear authoritative state = %#v", cleared.State)

	wantOrder := []string{
		"coordinator:request-terminal-activation:terminal-1", "event:coordination-state",
		"coordinator:update-live-terminal:terminal-1", "event:coordination-state",
		"coordinator:request-terminal-clear", "event:coordination-state",
	}
	{
		got := recorder.Calls()
		require.Falsef(t, !cmp.Equal(got, wantOrder),
			"terminal coordination order = %v, want %v", got, wantOrder)
	}
	require.Falsef(t, legacy.setCalls != 0 || legacy.updateCalls != 0 || legacy.clearCalls != 0,
		"coordinated commands mutated legacy live state: set/update/clear=%d/%d/%d", legacy.setCalls, legacy.updateCalls, legacy.clearCalls)

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.CoordinationState == nil || status.CoordinationState.Revision != 43 || status.HackState == nil || status.HackState.AttemptsLeft != 2,
		"terminal runtime status = %#v, want revision 43 and unchanged hack mirror", status)

	cleared.State.Revision = 999
	require.False(t, app.GetRuntimeStatus().CoordinationState.Revision != 43,
		"terminal command result aliases replayable coordination state")

}

func TestCoordinationBridgeRejectsInvalidOrFailedTerminalRequestsWithoutOptimisticState(t *testing.T) {
	recorder := &callRecorder{}
	initial := &domain.MasterCoordinationState{Revision: 50, Broadcast: &domain.MasterBroadcastState{ID: "broadcast-terminal-refusal"}}
	coordination := &recordingTerminalCoordinationService{
		state: initial, order: recorder,
	}
	legacy := &recordingLiveService{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: coordination, Live: legacy,
		Player: &recordingPlayerServer{recorder: recorder}, Events: &recordingEventSink{recorder: recorder},
	})

	invalidTree := domain.ContentNode{ID: "root", Type: domain.NodeCommand, Name: "not a folder"}
	invalid := []TerminalSwitchCommandResult{
		app.RequestTerminalActivation(LiveTerminalPayload{TerminalID: " ", TerminalName: "Overseer", Tree: invalidTree}),
		app.RequestTerminalActivation(LiveTerminalPayload{TerminalID: "terminal-1", TerminalName: " ", Tree: invalidTree}),
		app.RequestTerminalActivation(LiveTerminalPayload{TerminalID: "terminal-1", TerminalName: "Overseer", Tree: invalidTree}),
	}
	for index, result := range invalid {
		require.Falsef(t, result.OK || result.Error == "" || result.State == nil || result.State.Revision != 50,
			"invalid terminal activation %d = %#v", index, result)

	}
	require.Falsef(t, len(coordination.targets) != 0 || len(recorder.Calls()) != 0,
		"invalid payload reached coordinator/publication: targets=%#v calls=%v", coordination.targets, recorder.Calls())

	coordination.commandErr = errors.New("active terminal has an unfinished puzzle")
	validTree := domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}}
	rejected := app.RequestTerminalActivation(LiveTerminalPayload{
		TerminalID: "terminal-2", TerminalName: "Overseer 2", Tree: validTree, HackLevel: 1,
	})
	require.Falsef(t, rejected.OK || !strings.Contains(rejected.Error, "unfinished") || rejected.Status != "" || rejected.State == nil || rejected.State.Revision != 50,
		"failed terminal activation = %#v", rejected)
	{

		status := app.GetRuntimeStatus().CoordinationState
		require.Falsef(t, !cmp.Equal(status, initial),
			"failed activation changed replay state: %#v", status)
	}
	{

		got := recorder.Calls()
		require.Falsef(t, !cmp.Equal(got, []string{"coordinator:request-terminal-activation:terminal-2"}),
			"failed activation publications = %v", got)
	}
	require.Falsef(t, legacy.setCalls != 0 || legacy.updateCalls != 0 || legacy.clearCalls != 0,
		"failed request touched legacy live: set/update/clear=%d/%d/%d", legacy.setCalls, legacy.updateCalls, legacy.clearCalls)

}

func TestResetFailedHackValidatesPrivatePayloadAndReturnsAuthoritativeState(t *testing.T) {
	recorder := &callRecorder{}
	initial := &domain.MasterCoordinationState{
		Revision:  70,
		Broadcast: &domain.MasterBroadcastState{ID: "broadcast-reset", ActiveTerminalID: new("terminal-1")},
	}
	coordination := &recordingTerminalCoordinationService{
		state: initial, order: recorder,
	}
	legacy := &recordingLiveService{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: coordination, Live: legacy, Events: &recordingEventSink{recorder: recorder},
	})
	tree := domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}}

	invalid := app.ResetFailedHack(LiveTerminalPayload{TerminalID: " ", TerminalName: "Overseer", Tree: tree, HackLevel: 2})
	require.Falsef(t, invalid.OK || invalid.Error == "" || invalid.State == nil || invalid.State.Revision != 70 || len(coordination.resetTargets) != 0,
		"invalid ResetFailedHack() = %#v targets=%#v", invalid, coordination.resetTargets)

	result := app.ResetFailedHack(LiveTerminalPayload{
		TerminalID: "  terminal-1 ", TerminalName: " Overseer Latest ", Tree: tree, HackLevel: 2, IntroText: "LATEST",
	})
	require.Falsef(t, !result.OK || result.Error != "" || result.State == nil || result.State.Revision != 71,
		"ResetFailedHack() = %#v", result)
	require.Falsef(t, len(coordination.resetTargets) != 1 || coordination.resetTargets[0].TerminalID != "terminal-1" || coordination.resetTargets[0].TerminalName != "Overseer Latest" || coordination.resetTargets[0].HackLevel != 2,
		"validated reset payloads = %#v", coordination.resetTargets)
	{

		got := recorder.Calls()
		require.Falsef(t, !cmp.Equal(got, []string{"coordinator:reset-failed-hack:terminal-1", "event:coordination-state"}),
			"reset order = %v", got)
	}
	require.Falsef(t, legacy.setCalls != 0 || legacy.updateCalls != 0 || legacy.clearCalls != 0 || legacy.forceCalls != 0,
		"reset bypassed coordinator through legacy live service: %#v", legacy)

	coordination.commandErr = errors.New("active hacking puzzle is not failed")
	rejected := app.ResetFailedHack(LiveTerminalPayload{
		TerminalID: "terminal-1", TerminalName: "Overseer Latest", Tree: tree, HackLevel: 2, IntroText: "LATEST",
	})
	require.Falsef(t, rejected.OK || !strings.Contains(rejected.Error, "not failed") || rejected.State == nil || rejected.State.Revision != 71 || app.GetRuntimeStatus().CoordinationState.Revision != 71,
		"ineligible ResetFailedHack() = %#v", rejected)

}

func TestCommandStateResetRejectsBlankStableIDsBeforeMutation(t *testing.T) {
	recorder := &callRecorder{}
	sessions := &recordingCommandStateSession{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions,
		Events:   &recordingEventSink{recorder: recorder},
		Player:   &recordingPlayerServer{recorder: recorder},
	})

	for _, payload := range []ResetCommandStatePayload{
		{TerminalID: " ", CommandID: "command-stable-1"},
		{TerminalID: "terminal-stable-1", CommandID: "\t"},
	} {
		result := app.ResetCommandState(payload)
		require.False(t, result.OK)
		require.NotEmpty(t, result.Error)
		require.Zero(t, result.Revision)
		require.Nil(t, result.Session)
	}

	terminalResult := app.ResetTerminalCommandStates(ResetTerminalCommandStatesPayload{TerminalID: "\n"})
	require.False(t, terminalResult.OK)
	require.NotEmpty(t, terminalResult.Error)
	require.Empty(t, sessions.resetOneCalls)
	require.Empty(t, sessions.resetTerminalCalls)
	require.Empty(t, recorder.Calls())
}

func TestResetCommandStatePublishesCanonicalSessionAfterDurabilityAndRefreshesActiveTerminalOnce(t *testing.T) {
	recorder := &callRecorder{}
	session := commandStateResetSessionFixture()
	delete(session.Terminals[0].CommandStates, "command-stable-1")
	sessions := &recordingCommandStateSession{
		recorder: recorder,
		resetOneResult: sessionservice.CommandStateResult{
			OK: true, Changed: true, Revision: 41, Session: &session,
		},
	}
	activeTerminalID := "terminal-stable-1"
	coordination := &recordingTerminalCoordinationService{
		state: &domain.MasterCoordinationState{
			Revision: 9,
			Broadcast: &domain.MasterBroadcastState{
				ID: "broadcast-1", ActiveTerminalID: &activeTerminalID,
			},
		},
		order: recorder,
	}
	events := &recordingEventSink{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions, Coordination: coordination, Events: events,
		Player: &recordingPlayerServer{recorder: recorder},
	})

	result := app.ResetCommandState(ResetCommandStatePayload{
		TerminalID: " terminal-stable-1 ", CommandID: " command-stable-1 ",
	})
	require.True(t, result.OK)
	require.Empty(t, result.Error)
	require.Equal(t, uint64(41), result.Revision)
	require.Equal(t, &session, result.Session)
	require.Equal(t, [][2]string{{"terminal-stable-1", "command-stable-1"}}, sessions.resetOneCalls)
	require.Equal(t, 1, coordination.updateCalls)
	require.Equal(t, session.Terminals[0].Root, coordination.updateTree)
	calls := recorder.Calls()
	require.NotEmpty(t, calls)
	require.Equal(t, "session:reset-command-state:terminal-stable-1:command-stable-1", calls[0])
	require.Equal(t, "coordinator:update-live-terminal:terminal-stable-1", calls[1])
	require.ElementsMatch(t, []string{"event:coordination-state", "event:session-state"}, calls[2:])

	records := events.Records()
	require.Len(t, records, 2)
	require.Equal(t, coordinationStateEvent, records[0].Name)
	require.Equal(t, sessionStateEvent, records[1].Name)
	event, ok := records[1].Payload.(SessionStateEvent)
	require.True(t, ok, "session-state payload = %#v", records[1].Payload)
	require.Equal(t, uint64(41), event.Revision)
	require.Equal(t, &session, event.Session)
}

func TestResetCommandStateNoOpReturnsCanonicalRevisionWithoutPublication(t *testing.T) {
	recorder := &callRecorder{}
	session := commandStateResetSessionFixture()
	delete(session.Terminals[0].CommandStates, "command-stable-1")
	sessions := &recordingCommandStateSession{
		recorder: recorder,
		resetOneResult: sessionservice.CommandStateResult{
			OK: true, Changed: false, Revision: 19, Session: &session,
		},
	}
	activeTerminalID := "terminal-stable-1"
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions,
		Coordination: &recordingCoordinationService{state: &domain.MasterCoordinationState{
			Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1", ActiveTerminalID: &activeTerminalID},
		}},
		Events: &recordingEventSink{recorder: recorder}, Player: &recordingPlayerServer{recorder: recorder},
	})

	result := app.ResetCommandState(ResetCommandStatePayload{TerminalID: "terminal-stable-1", CommandID: "command-stable-1"})
	require.True(t, result.OK)
	require.Equal(t, uint64(19), result.Revision)
	require.Equal(t, &session, result.Session)
	require.Equal(t, []string{"session:reset-command-state:terminal-stable-1:command-stable-1"}, recorder.Calls())
}

func TestResetTerminalCommandStatesUsesOneMutationAndDoesNotRefreshInactiveTerminal(t *testing.T) {
	recorder := &callRecorder{}
	session := commandStateResetSessionFixture()
	session.Terminals[0].CommandStates = nil
	sessions := &recordingCommandStateSession{
		recorder: recorder,
		resetTerminalResult: sessionservice.CommandStateResult{
			OK: true, Changed: true, Revision: 52, Session: &session,
		},
	}
	activeTerminalID := "another-terminal"
	events := &recordingEventSink{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions,
		Coordination: &recordingCoordinationService{state: &domain.MasterCoordinationState{
			Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1", ActiveTerminalID: &activeTerminalID},
		}},
		Events: events, Player: &recordingPlayerServer{recorder: recorder},
	})

	result := app.ResetTerminalCommandStates(ResetTerminalCommandStatesPayload{TerminalID: " terminal-stable-1 "})
	require.True(t, result.OK)
	require.Equal(t, uint64(52), result.Revision)
	require.Equal(t, &session, result.Session)
	require.Equal(t, []string{"terminal-stable-1"}, sessions.resetTerminalCalls)
	require.Equal(t, []string{
		"session:reset-terminal-command-states:terminal-stable-1",
		"event:session-state",
	}, recorder.Calls())
	require.Len(t, events.Records(), 1)
}

func TestResetTerminalCommandStatesRefreshesActiveCanonicalRuntime(t *testing.T) {
	live := liveservice.New(nil, nil)
	var effects []controlservice.Effect
	coordination := controlservice.New(controlservice.Config{
		Runtime: live, Terminals: live, TrustedHack: live,
		Enqueue: func(effect controlservice.Effect) { effects = append(effects, effect) },
	})
	master, err := coordination.AddCharacter(domain.CharacterCreatePayload{
		Name: "Mara", Intelligence: 1, HackerPerkAvailable: false, ExpectedRevision: coordination.Revision(),
	})
	require.NoError(t, err)
	characterID := master.Roster[0].ID
	master, err = coordination.StartBroadcast()
	require.NoError(t, err)
	require.NotNil(t, master.Broadcast)
	connectionID := domain.ConnectionID("reset-terminal-controller")
	controller := coordination.CreateSession(connectionID)
	selected := coordination.SelectCharacter(controlservice.CharacterSelection{
		ConnectionID: connectionID,
		SessionID:    controller.SessionID,
		RequestID:    "reset-terminal-select-controller",
		BroadcastID:  master.Broadcast.ID,
		CharacterID:  characterID,
	})
	require.True(t, selected.Accepted)

	session := commandStateResetSessionFixture()
	terminal := session.Terminals[0]
	_, err = coordination.RequestTerminalActivation(domain.TerminalTarget{
		TerminalID: terminal.ID, TerminalName: terminal.Name, Tree: terminal.Root,
		CommandStates: cloneCommandExecutionStates(terminal.CommandStates),
		HackLevel:     terminal.HackLevel, IntroText: terminal.IntroText,
	})
	require.NoError(t, err)
	shown := coordination.DispatchPlayerAction(connectionID, domain.RuntimeCommand{
		RequestID:   "show-completed-command",
		BroadcastID: master.Broadcast.ID,
		TerminalID:  terminal.ID,
		Kind:        domain.RuntimeCommandNavigate,
		Action:      "command",
		NodeID:      "command-stable-1",
	})
	require.True(t, shown.Accepted)
	pending := coordination.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	_, mutation, err := coordination.ResolveCommandExecution(t.Context(), pending.RequestID, domain.CommandExecutionApprove)
	require.NoError(t, err)
	require.Nil(t, mutation, "completed command approval must not write durable state")
	require.NotEmpty(t, effects)
	var beforeReset *domain.PublicLiveState
	for _, effect := range slices.Backward(effects) {
		if effect.Live != nil {
			beforeReset = effect.Live
			break
		}
	}
	require.NotNil(t, beforeReset)
	require.NotNil(t, beforeReset.Nav.CommandNodeID)

	terminal.CommandStates = nil
	session.Terminals[0] = terminal
	sessions := &recordingCommandStateSession{
		resetTerminalResult: sessionservice.CommandStateResult{
			OK: true, Changed: true, Revision: 52, Session: &session,
		},
	}
	effects = nil
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions, Coordination: coordination,
		Events: &recordingEventSink{recorder: &callRecorder{}},
	})

	result := app.ResetTerminalCommandStates(ResetTerminalCommandStatesPayload{TerminalID: terminal.ID})
	require.True(t, result.OK, "ResetTerminalCommandStates() = %#v", result)
	require.Equal(t, uint64(52), result.Revision)
	require.Empty(t, result.Session.Terminals[0].CommandStates)
	require.NotEmpty(t, effects)
	var afterReset *domain.PublicLiveState
	for _, effect := range slices.Backward(effects) {
		if effect.Live != nil {
			afterReset = effect.Live
			break
		}
	}
	require.NotNil(t, afterReset)
	require.Equal(t, "Open doors", afterReset.Tree.Children[0].Name)
	require.Nil(t, afterReset.Nav.CommandNodeID, "reset terminal retained the completed command result view")
}

func TestResetTerminalCommandStatesProductionPathPersistsAndRefreshesActiveTerminal(t *testing.T) {
	target := "/Campaigns/reset-terminal-production.json"
	fileSystem := testutil.NewFakeFileSystem()
	seed := commandStateResetSessionFixture()
	seed.Terminals[0].Root.Children = append(seed.Terminals[0].Root.Children, domain.ContentNode{
		ID: "command-stable-2", Type: domain.NodeCommand, Name: "Enable alarm", Text: "Alarm enabled",
		StateChange: &domain.StateChangeConfig{CompletedName: "Alarm enabled", ConfirmationText: "Enable the alarm?"},
	})
	seed.Terminals[0].CommandStates["command-stable-2"] = domain.CommandExecutionState{
		CompletedName: "Alarm enabled", ResultText: "Alarm enabled",
	}
	seed.Terminals = append(seed.Terminals, domain.Terminal{
		ID: "terminal-stable-2", Name: "Reserve terminal",
		Root: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: "reserve-command", Type: domain.NodeCommand, Name: "Unlock reserve", Text: "Reserve unlocked",
				StateChange: &domain.StateChangeConfig{CompletedName: "Reserve unlocked", ConfirmationText: "Unlock reserve?"},
			}},
		},
		CommandStates: map[string]domain.CommandExecutionState{
			"reserve-command": {CompletedName: "Reserve unlocked", ResultText: "Reserve unlocked"},
		},
	})
	encoded, err := domain.EncodeSession(seed)
	require.NoError(t, err)
	fileSystem.SeedFile(target, encoded)
	sessions := sessionservice.NewService(
		sessionservice.NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		sessionservice.Locations{
			DocumentsDefault: "/Campaigns", BundledDemo: "/Applications/Fallout/demo.json",
		},
	)
	t.Cleanup(func() { require.NoError(t, sessions.Shutdown(context.WithoutCancel(t.Context()))) })
	opened := sessions.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)

	live := liveservice.New(nil, nil)
	var effects []controlservice.Effect
	coordination := controlservice.New(controlservice.Config{
		Runtime: live, Terminals: live, TrustedHack: live,
		Enqueue: func(effect controlservice.Effect) { effects = append(effects, effect) },
	})
	_, err = coordination.AddCharacter(domain.CharacterCreatePayload{
		Name: "Mara", Intelligence: 1, HackerPerkAvailable: false, ExpectedRevision: coordination.Revision(),
	})
	require.NoError(t, err)
	master, err := coordination.AddCharacter(domain.CharacterCreatePayload{
		Name: "Boone", Intelligence: 1, HackerPerkAvailable: false, ExpectedRevision: coordination.Revision(),
	})
	require.NoError(t, err)
	controllerCharacterID := master.Roster[0].ID
	observerCharacterID := master.Roster[1].ID
	master, err = coordination.StartBroadcast()
	require.NoError(t, err)
	connectionID := domain.ConnectionID("production-reset-controller")
	controller := coordination.CreateSession(connectionID)
	selected := coordination.SelectCharacter(controlservice.CharacterSelection{
		ConnectionID: connectionID, SessionID: controller.SessionID,
		RequestID: "production-reset-select", BroadcastID: master.Broadcast.ID,
		CharacterID: controllerCharacterID,
	})
	require.True(t, selected.Accepted)
	observerConnectionID := domain.ConnectionID("production-reset-observer")
	observer := coordination.CreateSession(observerConnectionID)
	selected = coordination.SelectCharacter(controlservice.CharacterSelection{
		ConnectionID: observerConnectionID, SessionID: observer.SessionID,
		RequestID: "production-reset-select-observer", BroadcastID: master.Broadcast.ID,
		CharacterID: observerCharacterID,
	})
	require.True(t, selected.Accepted)
	terminal := opened.Session.Terminals[0]
	_, err = coordination.RequestTerminalActivation(domain.TerminalTarget{
		TerminalID: terminal.ID, TerminalName: terminal.Name, Tree: terminal.Root,
		CommandStates: cloneCommandExecutionStates(terminal.CommandStates),
		HackLevel:     terminal.HackLevel, IntroText: terminal.IntroText,
	})
	require.NoError(t, err)
	shown := coordination.DispatchPlayerAction(connectionID, domain.RuntimeCommand{
		RequestID: "production-show-completed", BroadcastID: master.Broadcast.ID,
		TerminalID: terminal.ID, Kind: domain.RuntimeCommandNavigate,
		Action: "command", NodeID: "command-stable-1",
	})
	require.True(t, shown.Accepted)
	pending := coordination.Snapshot().PendingCommandExecution
	require.NotNil(t, pending)
	_, mutation, err := coordination.ResolveCommandExecution(t.Context(), pending.RequestID, domain.CommandExecutionApprove)
	require.NoError(t, err)
	require.Nil(t, mutation, "completed command approval must not write durable state")

	effects = nil
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions, Coordination: coordination,
		Events: &recordingEventSink{recorder: &callRecorder{}},
	})
	resetStarted := time.Now()
	result := app.ResetTerminalCommandStates(ResetTerminalCommandStatesPayload{TerminalID: terminal.ID})
	require.Less(t, time.Since(resetStarted), time.Second)
	require.True(t, result.OK, "ResetTerminalCommandStates() = %#v", result)
	require.Equal(t, uint64(1), result.Revision)
	require.Empty(t, result.Session.Terminals[0].CommandStates)
	require.Contains(t, result.Session.Terminals[1].CommandStates, "reserve-command")

	durableBytes, ok := fileSystem.File(target)
	require.True(t, ok)
	durable, err := domain.DecodeSession(durableBytes)
	require.NoError(t, err)
	require.Empty(t, durable.Terminals[0].CommandStates)
	require.Contains(t, durable.Terminals[1].CommandStates, "reserve-command")
	require.Equal(t, uint64(1), sessions.Snapshot().SavedRevision)

	var projection *domain.PublicLiveState
	for _, effect := range slices.Backward(effects) {
		if effect.Live != nil {
			projection = effect.Live
			break
		}
	}
	require.NotNil(t, projection)
	require.Equal(t, "Open doors", projection.Tree.Children[0].Name)
	require.Equal(t, "Enable alarm", projection.Tree.Children[1].Name)
	require.Nil(t, projection.Nav.CommandNodeID, "production reset retained the completed command result view")
	masterAfterReset := coordination.Snapshot()
	require.Len(t, masterAfterReset.Sessions, 2)
	require.ElementsMatch(t, []domain.PlayerRole{domain.PlayerRoleActive, domain.PlayerRoleObserver}, []domain.PlayerRole{
		masterAfterReset.Sessions[0].Role, masterAfterReset.Sessions[1].Role,
	})

	reopened := sessions.Open(t.Context())
	require.True(t, reopened.OK, "reopen reset session = %#v", reopened)
	require.Empty(t, reopened.Session.Terminals[0].CommandStates)
	require.Contains(t, reopened.Session.Terminals[1].CommandStates, "reserve-command")
}

func TestCommandStateResetFailureDoesNotPublishSessionOrPlayerState(t *testing.T) {
	recorder := &callRecorder{}
	sessions := &recordingCommandStateSession{
		recorder:       recorder,
		resetOneResult: sessionservice.CommandStateResult{Error: "could not persist command state"},
	}
	activeTerminalID := "terminal-stable-1"
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: sessions,
		Coordination: &recordingCoordinationService{state: &domain.MasterCoordinationState{
			Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1", ActiveTerminalID: &activeTerminalID},
		}},
		Events: &recordingEventSink{recorder: recorder}, Player: &recordingPlayerServer{recorder: recorder},
	})

	result := app.ResetCommandState(ResetCommandStatePayload{TerminalID: "terminal-stable-1", CommandID: "command-stable-1"})
	require.False(t, result.OK)
	require.NotEmpty(t, result.Error)
	require.Zero(t, result.Revision)
	require.Nil(t, result.Session)
	require.Equal(t, []string{"session:reset-command-state:terminal-stable-1:command-stable-1"}, recorder.Calls())
}

func TestTerminalSwitchBridgeReturnsDecisionShapeAndResolvesValidatedChoices(t *testing.T) {
	recorder := &callRecorder{}
	initial := &domain.MasterCoordinationState{
		Revision:  60,
		Broadcast: &domain.MasterBroadcastState{ID: "broadcast-decision", ActiveTerminalID: new("terminal-1")},
	}
	coordination := &recordingTerminalCoordinationService{
		state: initial,
		order: recorder, decisionRequired: true, nextSwitchID: "opaque-switch-1",
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: coordination, Events: &recordingEventSink{recorder: recorder},
	})
	tree := domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}}

	pending := app.RequestTerminalActivation(LiveTerminalPayload{
		TerminalID: "terminal-2", TerminalName: "Archive", Tree: tree, HackLevel: 2,
	})
	require.Falsef(t, !pending.OK || pending.Error != "" || pending.Status != "decision-required" || pending.SwitchID != "opaque-switch-1" || pending.State == nil,
		"decision-required activation = %#v", pending)
	require.Falsef(t, pending.State.PendingSwitch == nil || pending.State.PendingSwitch.SwitchID != pending.SwitchID || pending.State.Broadcast.ActiveTerminalID == nil || *pending.State.Broadcast.ActiveTerminalID != "terminal-1",
		"pending activation changed source or omitted switch metadata: %#v", pending.State)

	resolved := app.ResolveTerminalSwitch(TerminalSwitchDecisionPayload{
		SwitchID: pending.SwitchID, Decision: domain.TerminalSwitchPreserve,
	})
	require.Falsef(t, !resolved.OK || resolved.Error != "" || resolved.Status != "activated" || resolved.SwitchID != "" || resolved.State == nil || resolved.State.PendingSwitch != nil,
		"preserve resolution = %#v", resolved)
	{

		got := coordination.decisions
		require.Falsef(t, !cmp.Equal(got, []recordedTerminalDecision{{SwitchID: "opaque-switch-1", Decision: domain.TerminalSwitchPreserve}}),
			"coordinator decisions = %#v", got)
	}
	{

		got := recorder.Calls()
		require.Falsef(t, !cmp.Equal(got, []string{
			"coordinator:request-terminal-activation:terminal-2", "event:coordination-state",
			"coordinator:resolve-terminal-switch:opaque-switch-1:preserve", "event:coordination-state",
		}),
			"decision bridge ordering = %v", got)
	}

	for _, decision := range []domain.TerminalSwitchChoice{domain.TerminalSwitchDiscard, domain.TerminalSwitchCancel} {
		coordination.decisionRequired = true
		coordination.nextSwitchID = domain.SwitchID("opaque-" + string(decision))
		request := app.RequestTerminalClear()
		require.Falsef(t, request.Status != "decision-required" || request.SwitchID == "",
			"clear %s decision request = %#v", decision, request)

		result := app.ResolveTerminalSwitch(TerminalSwitchDecisionPayload{SwitchID: request.SwitchID, Decision: decision})
		wantStatus := "cleared"
		if decision == domain.TerminalSwitchCancel {
			wantStatus = "cancelled"
		}
		require.Falsef(t, !result.OK || result.Status != wantStatus || result.SwitchID != "",
			"resolve %s = %#v, want status %q", decision, result, wantStatus)

		if decision != domain.TerminalSwitchCancel {
			active := "terminal-1"
			coordination.state.Broadcast.ActiveTerminalID = &active
		}
	}
}

func TestTerminalSwitchBridgeRejectsInvalidAndStaleDecisionButKeepsTrustedForceSuccessEligible(t *testing.T) {
	initial := &domain.MasterCoordinationState{
		Revision:  70,
		Broadcast: &domain.MasterBroadcastState{ID: "broadcast-decision", ActiveTerminalID: new("terminal-1")},
		PendingSwitch: &domain.MasterPendingSwitch{
			SwitchID: "switch-current", BroadcastID: "broadcast-decision", SourceTerminalID: "terminal-1",
		},
	}
	coordination := &recordingTerminalCoordinationService{
		state:      initial,
		commandErr: errors.New("terminal switch decision is stale"),
		forceState: &domain.PublicHackState{Level: 2, AttemptsMax: 4, AttemptsLeft: 3},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: coordination})

	invalid := []TerminalSwitchDecisionPayload{
		{SwitchID: "", Decision: domain.TerminalSwitchPreserve},
		{SwitchID: "switch-current", Decision: ""},
		{SwitchID: "switch-current", Decision: "restart"},
	}
	for index, payload := range invalid {
		result := app.ResolveTerminalSwitch(payload)
		require.Falsef(t, result.OK || result.Error == "" || result.State == nil || result.State.Revision != 70,
			"invalid decision %d = %#v", index, result)

	}
	require.Falsef(t, len(coordination.decisions) != 0,
		"invalid decisions reached coordinator: %#v", coordination.decisions)

	stale := app.ResolveTerminalSwitch(TerminalSwitchDecisionPayload{SwitchID: "switch-old", Decision: domain.TerminalSwitchDiscard})
	require.Falsef(t, stale.OK || !strings.Contains(stale.Error, "stale") || stale.Status != "" || stale.SwitchID != "" || stale.State == nil || !cmp.Equal(stale.State, initial),
		"stale decision = %#v", stale)

	stale.State.Revision = 999
	require.False(t, app.GetRuntimeStatus().CoordinationState.Revision != 70,
		"stale switch result aliases replay state")

	forced := app.ForceHackSuccess()
	require.Falsef(t, !forced.OK || coordination.forceCalls != 1,
		"ForceHackSuccess() while decision pending = %#v, calls %d", forced, coordination.forceCalls)

}

func TestOpenURLAllowsOnlyHTTPAndHTTPS(t *testing.T) {
	browser := &testutil.FakeBrowser{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Browser: browser})

	for _, rawURL := range []string{
		"file:///tmp/session.json",
		"javascript:alert(1)",
		"mailto:overseer@example.test",
		"not a URL",
		"http://[::1",
	} {
		result := app.OpenURL(rawURL)
		assert.Falsef(t, result.OK || result.Error == "",
			"OpenURL(%q) = %#v, want structured rejection", rawURL, result)

	}

	for _, rawURL := range []string{
		"http://127.0.0.1:3690/",
		"https://players.example.test/session",
	} {
		{
			result := app.OpenURL(rawURL)
			assert.Falsef(t, !result.OK || result.Error != "",
				"OpenURL(%q) = %#v, want success", rawURL, result)
		}

	}
	{
		got, want := browser.URLs(), []string{
			"http://127.0.0.1:3690/",
			"https://players.example.test/session",
		}
		require.Falsef(t, !cmp.Equal(got, want),
			"browser URLs = %v, want %v", got, want)
	}

}

func TestBridgeCoordinatorActivationAndLifecycleCleanup(t *testing.T) {
	recorder := &callRecorder{}
	live := &recordingLiveService{}
	coordination := &recordingTerminalCoordinationService{
		state: &domain.MasterCoordinationState{
			Revision: 1, Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1"},
		},
		order: recorder,
	}
	events := &recordingEventSink{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Sessions: &recordingSessionService{recorder: recorder},
		Player: &recordingPlayerServer{
			recorder: recorder,
			info:     domain.ServerInfo{IP: "127.0.0.1", Port: 3690, URL: "http://127.0.0.1:3690"},
		},
		Desktop:      &recordingDesktop{recorder: recorder},
		Events:       events,
		Live:         live,
		Coordination: coordination,
	})
	{
		err := app.Start(t.Context())
		require.Falsef(t, err != nil,
			"Start() error = %v", err)
	}

	result := app.RequestTerminalActivation(LiveTerminalPayload{
		TerminalID:   "terminal-1",
		TerminalName: "Overseer",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{},
		},
		HackLevel: 1,
		IntroText: "ROBCO INDUSTRIES UNIFIED OPERATING SYSTEM",
	})
	require.Falsef(t, !result.OK || result.Error != "",
		"RequestTerminalActivation(valid) = %#v", result)

	records := events.Records()
	last := records[len(records)-1]
	require.Falsef(t, last.Name != coordinationStateEvent,
		"last bridge event = %#v, want coordination-state", last)

	coordinationState, ok := last.Payload.(*domain.MasterCoordinationState)
	require.Falsef(t, !ok || coordinationState == nil || coordinationState.Broadcast == nil || coordinationState.Broadcast.ActiveTerminalID == nil || *coordinationState.Broadcast.ActiveTerminalID != "terminal-1",
		"coordination-state payload = %#v", last.Payload)
	{

		err := app.Shutdown(t.Context())
		require.Falsef(t, err != nil,
			"Shutdown() error = %v", err)
	}
	require.Falsef(t, live.clearCalls != 1,
		"live Clear calls after shutdown = %d, want 1", live.clearCalls)

}

func TestResolveTerminalNavigationUsesOnlyExactPrivateDecisionAndPublishesOneNewRevision(t *testing.T) {
	controller := domain.LogicalSessionID("controller-1")
	coordination := &recordingCoordinationService{state: &domain.MasterCoordinationState{
		Revision:  8,
		Broadcast: &domain.MasterBroadcastState{ID: "broadcast-1", ControllerSessionID: &controller, ActiveTerminalID: new("terminal-a")},
		PendingTerminalNavigation: &domain.MasterPendingTerminalNavigation{
			RequestID: "navigation-1", BroadcastID: "broadcast-1", Direction: domain.TerminalNavigationForward,
			SourceTerminalID: "terminal-a", TargetTerminalID: "terminal-b",
		},
	}}
	events := &recordingEventSink{recorder: &callRecorder{}}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Coordination: coordination, Events: events})

	result := app.ResolveTerminalNavigation(TerminalNavigationDecisionPayload{RequestID: " navigation-1 ", Decision: domain.TerminalNavigationApprove})
	require.True(t, result.OK)
	require.Empty(t, result.Error)
	require.Equal(t, uint64(9), result.State.Revision)
	require.Nil(t, result.State.PendingTerminalNavigation)
	require.Equal(t, []recordedTerminalNavigationDecision{{RequestID: "navigation-1", Decision: domain.TerminalNavigationApprove}}, coordination.navigationDecisions)
	require.Len(t, events.Records(), 1)
	require.Equal(t, coordinationStateEvent, events.Records()[0].Name)

	invalid := app.ResolveTerminalNavigation(TerminalNavigationDecisionPayload{RequestID: "navigation-2", Decision: "discard"})
	require.False(t, invalid.OK)
	require.Len(t, coordination.navigationDecisions, 1, "invalid private enums must not reach the coordinator")
}

func TestForceHackSuccessPublishesSolvedStateWithoutSpendingAttempt(t *testing.T) {
	recorder := &callRecorder{}
	events := &recordingEventSink{recorder: recorder}
	live := &recordingLiveService{
		forceState: &domain.PublicHackState{
			Level: 2, AttemptsMax: 4, AttemptsLeft: 2, Solved: true,
			Patterns: []domain.PublicHackPattern{{ID: "opaque-generation-pattern", Row: 0, Start: 0, End: 1, Used: false}},
		},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Live: live, Player: &recordingPlayerServer{recorder: recorder}, Events: events,
	})
	{

		result := app.ForceHackSuccess()
		require.Falsef(t, !result.OK,
			"ForceHackSuccess() = %#v", result)
	}
	require.Falsef(t, live.forceCalls != 1,
		"ForceHackSuccess calls = %d, want 1", live.forceCalls)

	records := events.Records()
	require.Falsef(t, len(records) != 1,
		"hack-state event count = %d, want 1", len(records))

	state, ok := records[0].Payload.(*domain.PublicHackState)
	require.Falsef(t, !ok || state == nil || !state.Solved || state.AttemptsLeft != 2 || len(state.Patterns) != 1,
		"forced public state = %#v", records[0].Payload)

}

func TestForceHackSuccessRejectsIneligiblePuzzleWithoutPublication(t *testing.T) {
	recorder := &callRecorder{}
	live := &recordingLiveService{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Live: live, Player: &recordingPlayerServer{recorder: recorder}, Events: &recordingEventSink{recorder: recorder},
	})

	result := app.ForceHackSuccess()
	require.Falsef(t, result.OK || result.Error == "",
		"ForceHackSuccess() = %#v, want structured ineligible rejection", result)
	require.Falsef(t, live.forceCalls != 1 || len(recorder.Calls()) != 0,
		"ineligible force calls=%d publications=%v, want one validation and no publication", live.forceCalls, recorder.Calls())

}

func TestForceHackSuccessPrefersOrderedCoordinatorAndPublishesHackStatus(t *testing.T) {
	recorder := &callRecorder{}
	coordination := &recordingCoordinatedHackService{
		state: &domain.MasterCoordinationState{Revision: 8},
		forceState: &domain.PublicHackState{
			Level: 2, AttemptsMax: 4, AttemptsLeft: 2, Solved: true,
			Log: []string{"Exact private completion"},
		},
	}
	legacy := &recordingLiveService{forceState: &domain.PublicHackState{Solved: true}}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: coordination,
		Live:         legacy,
		Events:       &recordingEventSink{recorder: recorder},
	})
	{

		result := app.ForceHackSuccess()
		require.Falsef(t, !result.OK || result.Error != "",
			"ForceHackSuccess() = %#v, want ordered success", result)
	}
	require.Falsef(t, coordination.forceCalls != 1 || legacy.forceCalls != 0,
		"force calls coordinator=%d legacy=%d, want 1/0", coordination.forceCalls, legacy.forceCalls)

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.HackState == nil || !status.HackState.Solved || status.HackState.AttemptsLeft != 2,
		"ordered force status = %#v", status.HackState)

	records := recorder.Calls()
	require.Falsef(t, !cmp.Equal(records, []string{"event:hack-state"}),
		"ordered force publications = %v, want one hack-state event", records)

}

func TestForceHackSuccessDoesNotBypassCoordinatorRejection(t *testing.T) {
	recorder := &callRecorder{}
	coordination := &recordingCoordinatedHackService{
		state: &domain.MasterCoordinationState{Revision: 8},
	}
	legacy := &recordingLiveService{forceState: &domain.PublicHackState{Solved: true}}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: coordination,
		Live:         legacy,
		Events:       &recordingEventSink{recorder: recorder},
	})

	result := app.ForceHackSuccess()
	require.Falsef(t, result.OK || result.Error == "",
		"ForceHackSuccess() = %#v, want coordinator rejection", result)
	require.Falsef(t, coordination.forceCalls != 1 || legacy.forceCalls != 0 || len(recorder.Calls()) != 0,
		"rejected ordered force calls coordinator=%d legacy=%d events=%v", coordination.forceCalls, legacy.forceCalls, recorder.Calls())

}

func TestForceHackSuccessUsesProductionCoordinatorOwnedRuntime(t *testing.T) {
	liveService := liveservice.New(nil, nil)
	var effects []controlservice.Effect
	coordination := controlservice.New(controlservice.Config{
		Runtime: liveService, Terminals: liveService, TrustedHack: liveService,
		Enqueue: func(effect controlservice.Effect) { effects = append(effects, effect) },
	})
	{
		_, err := coordination.StartBroadcast()
		require.False(t, err != nil,
			err)
	}
	{

		_, err := coordination.RequestTerminalActivation(domain.TerminalTarget{
			TerminalID: "terminal-force-app", TerminalName: "Force App", HackLevel: 1, IntroText: "WELCOME",
			Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT"},
		})
		require.False(t, err != nil,
			err)
	}
	{

		legacy := liveService.Snapshot()
		require.Falsef(t, legacy != nil,
			"legacy live slot unexpectedly owns production runtime: %#v", legacy)
	}

	beforeRevision := coordination.Revision()
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		Coordination: coordination,
		Live:         liveService,
		Events:       &recordingEventSink{recorder: &callRecorder{}},
	})
	{

		result := app.ForceHackSuccess()
		require.Falsef(t, !result.OK || result.Error != "",
			"ForceHackSuccess() = %#v", result)
	}
	require.Falsef(t, coordination.Revision() != beforeRevision+1,
		"force revision = %d, want %d", coordination.Revision(), beforeRevision+1)

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.HackState == nil || !status.HackState.Solved || status.HackState.Failed,
		"app hack status = %#v", status.HackState)

	var published *domain.PublicLiveState
	for _, effect := range effects {
		if effect.Revision == coordination.Revision() && effect.Live != nil {
			published = effect.Live
		}
	}
	require.Falsef(t, published == nil || published.TerminalID != "terminal-force-app" || published.Hack == nil || !published.Hack.Solved,
		"coordinator publication = %#v", published)
	{

		legacy := liveService.Snapshot()
		require.Falsef(t, legacy != nil,
			"trusted app force populated legacy live slot: %#v", legacy)
	}
	{

		result := app.ForceHackSuccess()
		require.Falsef(t, result.OK || result.Error == "" || coordination.Revision() != beforeRevision+1,
			"repeated ForceHackSuccess() = %#v revision=%d", result, coordination.Revision())
	}

}

func TestPlayerCallbacksEmitAndRetainDetachedPublicStatus(t *testing.T) {
	recorder := &callRecorder{}
	events := &recordingEventSink{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Events: events})
	hackState := &domain.PublicHackState{
		Level: 3, AttemptsMax: 4, AttemptsLeft: 2,
		Log:      []string{"ENTRY DENIED"},
		Patterns: []domain.PublicHackPattern{{ID: "opaque-generation-pattern", Row: 0, Start: 0, End: 1}},
	}

	app.updateClientCount(6)
	app.updateHackState(hackState)
	hackState.AttemptsLeft = 0
	hackState.Log[0] = "MUTATED"
	hackState.Patterns[0].ID = "mutated"
	hackState.Patterns[0].Row = 99
	hackState.Patterns[0].Used = true
	{

		got, want := recorder.Calls(), []string{"event:client-count", "event:hack-state"}
		require.Falsef(t, !cmp.Equal(got, want),
			"player callback events = %v, want %v", got, want)
	}

	status := app.GetRuntimeStatus()
	require.Falsef(t, status.ClientCount != 6 || status.HackState == nil || status.HackState.AttemptsLeft != 2 || status.HackState.Log[0] != "ENTRY DENIED" || status.HackState.Patterns[0].ID != "opaque-generation-pattern" || status.HackState.Patterns[0].Row != 0 || status.HackState.Patterns[0].Used,
		"detached player callback status = %#v", status)

}

type callRecorder struct {
	mu    sync.Mutex
	calls []string
}

func countRecordedCall(calls []string, want string) int {
	count := 0
	for _, call := range calls {
		if call == want {
			count++
		}
	}
	return count
}

type contextCapturingPlayer struct {
	info              domain.ServerInfo
	startContext      context.Context
	contextErrAtStart error
}

func (player *contextCapturingPlayer) Start(ctx context.Context) (domain.ServerInfo, error) {
	player.startContext = ctx
	player.contextErrAtStart = ctx.Err()
	return player.info, nil
}

func (*contextCapturingPlayer) Stop(context.Context) error { return nil }

type contextCapturingDesktop struct {
	readyContext context.Context
}

func (desktop *contextCapturingDesktop) Ready(ctx context.Context) error {
	desktop.readyContext = ctx
	return nil
}

func (*contextCapturingDesktop) Close(context.Context) error { return nil }

type contextCapturingEvents struct {
	context context.Context
}

func (events *contextCapturingEvents) SetContext(ctx context.Context) { events.context = ctx }
func (*contextCapturingEvents) Emit(string, any) error                { return nil }

func (r *callRecorder) Add(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *callRecorder) Calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *callRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

type recordingPlayerServer struct {
	recorder     *callRecorder
	info         domain.ServerInfo
	startErr     error
	stopContexts []context.Context
}

type recordingSessionService struct {
	recorder         *callRecorder
	shutdownCalls    int
	shutdownContexts []context.Context
}

type loggingSessionCommands struct {
	recordingSessionService
	createResult sessionservice.SessionResult
	openResult   sessionservice.SessionResult
	copyResult   sessionservice.SessionResult
	saveResult   sessionservice.SaveResult
	active       sessionservice.ActiveSession
}

func (service *loggingSessionCommands) Create(context.Context) sessionservice.SessionResult {
	service.active.Session = service.createResult.Session
	return service.createResult
}

func (service *loggingSessionCommands) Open(context.Context) sessionservice.SessionResult {
	return service.openResult
}

func (service *loggingSessionCommands) CopyDemo(context.Context) sessionservice.SessionResult {
	return service.copyResult
}

func (service *loggingSessionCommands) Save(context.Context, domain.Session, uint64) sessionservice.SaveResult {
	return service.saveResult
}

func (service *loggingSessionCommands) Snapshot() sessionservice.ActiveSession {
	return service.active
}

type recordingCommandStateSession struct {
	recordingSessionService
	resetOneResult      sessionservice.CommandStateResult
	resetTerminalResult sessionservice.CommandStateResult
	resetOneCalls       [][2]string
	resetTerminalCalls  []string
}

func (service *recordingCommandStateSession) ResetCommandState(_ context.Context, terminalID, commandID string) sessionservice.CommandStateResult {
	service.resetOneCalls = append(service.resetOneCalls, [2]string{terminalID, commandID})
	if service.recorder != nil {
		service.recorder.Add("session:reset-command-state:" + terminalID + ":" + commandID)
	}
	return service.resetOneResult
}

func (service *recordingCommandStateSession) ResetTerminalCommandStates(_ context.Context, terminalID string) sessionservice.CommandStateResult {
	service.resetTerminalCalls = append(service.resetTerminalCalls, terminalID)
	if service.recorder != nil {
		service.recorder.Add("session:reset-terminal-command-states:" + terminalID)
	}
	return service.resetTerminalResult
}

func commandStateResetSessionFixture() domain.Session {
	return domain.Session{
		Version: 1,
		Name:    "Stable ID reset fixture",
		Terminals: []domain.Terminal{{
			ID:        "terminal-stable-1",
			Name:      "Overseer terminal",
			HackLevel: 0,
			Root: domain.ContentNode{
				ID: "root", Type: domain.NodeFolder, Name: "ROOT",
				Children: []domain.ContentNode{{
					ID: "command-stable-1", Type: domain.NodeCommand, Name: "Open doors", Text: "Doors opened",
					StateChange: &domain.StateChangeConfig{CompletedName: "Doors open", ConfirmationText: "Open the doors?"},
				}},
			},
			CommandStates: map[string]domain.CommandExecutionState{
				"command-stable-1": {CompletedName: "Doors open", ResultText: "Doors opened"},
			},
		}},
	}
}

type recordingPlayerConfigSession struct {
	recordingSessionService
	snapshot     sessionservice.ActiveSession
	associations []string
}

func (service *recordingPlayerConfigSession) Snapshot() sessionservice.ActiveSession {
	return service.snapshot
}

func (service *recordingPlayerConfigSession) AssociatePlayerConfig(_ context.Context, path string) sessionservice.SessionResult {
	service.associations = append(service.associations, path)
	if service.snapshot.Session == nil {
		return sessionservice.SessionResult{Error: "no active session"}
	}
	copy := *service.snapshot.Session
	copy.PlayerConfig = "players/shared.json"
	service.snapshot.Session = &copy
	return sessionservice.SessionResult{OK: true, FilePath: service.snapshot.Path, Session: &copy}
}

type recordingPlayerConfigService struct {
	next playerconfigservice.Result
}

func (service *recordingPlayerConfigService) Create(context.Context) playerconfigservice.Result {
	return service.next
}
func (service *recordingPlayerConfigService) Open(context.Context) playerconfigservice.Result {
	return service.next
}
func (service *recordingPlayerConfigService) LoadReferenced(string, string) playerconfigservice.Result {
	return service.next
}

type recordingPlayerConfigCoordination struct {
	recordingCoordinationService
	installs []string
}

type loggingPlayerConfigBroadcastCoordination struct {
	recordingPlayerConfigCoordination
	endState *domain.MasterCoordinationState
}

func (service *loggingPlayerConfigBroadcastCoordination) EndBroadcast() (*domain.MasterCoordinationState, error) {
	service.state = domain.CloneMasterCoordinationState(service.endState)
	return domain.CloneMasterCoordinationState(service.state), nil
}

func (service *recordingPlayerConfigCoordination) InstallPlayerConfig(handle domain.PlayerConfigHandle, roster []domain.CharacterRosterEntry) (*domain.MasterCoordinationState, error) {
	entry := handle.Name + ":"
	if len(roster) > 0 {
		entry += string(roster[0].ID)
	}
	service.installs = append(service.installs, entry)
	state := domain.CloneMasterCoordinationState(service.state)
	state.PlayerConfig = &domain.PlayerConfigMetadata{Status: "loaded", Name: handle.Name, FilePath: handle.Path, Version: handle.Version}
	state.Roster = make([]domain.MasterRosterEntry, len(roster))
	for index, character := range roster {
		state.Roster[index] = domain.MasterRosterEntry{ID: character.ID, Name: character.Name}
	}
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingPlayerConfigCoordination) ClearPlayerConfig() (*domain.MasterCoordinationState, error) {
	state := domain.CloneMasterCoordinationState(service.state)
	state.PlayerConfig = nil
	state.Roster = []domain.MasterRosterEntry{}
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingSessionService) Shutdown(ctx context.Context) error {
	service.shutdownCalls++
	service.shutdownContexts = append(service.shutdownContexts, ctx)
	if service.recorder != nil {
		service.recorder.Add("session:shutdown")
	}
	return nil
}

type recordingLiveService struct {
	setState    *domain.PublicLiveState
	updateState *domain.PublicLiveState
	forceState  *domain.PublicHackState
	setCalls    int
	updateCalls int
	clearCalls  int
	forceCalls  int
}

type recordingCoordinationService struct {
	state               *domain.MasterCoordinationState
	addState            *domain.MasterCoordinationState
	startState          *domain.MasterCoordinationState
	addErr              error
	startErr            error
	addPayloads         []domain.CharacterCreatePayload
	startCalls          int
	navigationDecisions []recordedTerminalNavigationDecision
}

type recordedTerminalGroupReplacement struct {
	ctx       context.Context
	candidate domain.TerminalGroupCandidate
}

type recordingTerminalGroupCoordinationService struct {
	recordingCoordinationService
	recorder         *callRecorder
	replacementState *domain.MasterCoordinationState
	mutation         *controlservice.TerminalGroupMutation
	err              error
	calls            []recordedTerminalGroupReplacement
}

func (service *recordingTerminalGroupCoordinationService) ReplaceTerminalGroups(
	ctx context.Context,
	candidate domain.TerminalGroupCandidate,
) (*domain.MasterCoordinationState, *controlservice.TerminalGroupMutation, error) {
	clonedCandidate := candidate
	clonedCandidate.TerminalGroups = make([]domain.TerminalGroup, len(candidate.TerminalGroups))
	for index, group := range candidate.TerminalGroups {
		clonedCandidate.TerminalGroups[index] = group
		clonedCandidate.TerminalGroups[index].TerminalIDs = append([]string(nil), group.TerminalIDs...)
	}
	service.calls = append(service.calls, recordedTerminalGroupReplacement{ctx: ctx, candidate: clonedCandidate})
	if service.recorder != nil {
		service.recorder.Add("coordinator:replace-terminal-groups")
	}
	if service.err != nil {
		return domain.CloneMasterCoordinationState(service.replacementState), service.mutation, service.err
	}
	service.state = domain.CloneMasterCoordinationState(service.replacementState)
	return domain.CloneMasterCoordinationState(service.replacementState), service.mutation, nil
}

type recordedTerminalNavigationDecision struct {
	RequestID string
	Decision  domain.TerminalNavigationDecision
}

type recordingCoordinatedHackService struct {
	recordingCoordinationService
	forceState *domain.PublicHackState
	forceCalls int
}

type recordingTerminalCoordinationService struct {
	recordingCoordinationService
	order            *callRecorder
	targets          []domain.TerminalTarget
	clearCalls       int
	updateCalls      int
	updateTree       domain.ContentNode
	updateIntro      *string
	commandErr       error
	decisionRequired bool
	nextSwitchID     domain.SwitchID
	decisions        []recordedTerminalDecision
	forceState       *domain.PublicHackState
	forceCalls       int
	resetTargets     []domain.TerminalTarget
}

type recordingBroadcastLifecycleService struct {
	recordingCoordinationService
	endState      *domain.MasterCoordinationState
	endErr        error
	endCalls      int
	shutdownCalls int
}

func (service *recordingBroadcastLifecycleService) EndBroadcast() (*domain.MasterCoordinationState, error) {
	service.endCalls++
	if service.endErr != nil {
		return domain.CloneMasterCoordinationState(service.state), service.endErr
	}
	service.state = domain.CloneMasterCoordinationState(service.endState)
	return domain.CloneMasterCoordinationState(service.state), nil
}

func (service *recordingBroadcastLifecycleService) Shutdown() { service.shutdownCalls++ }

type recordedTerminalDecision struct {
	SwitchID domain.SwitchID
	Decision domain.TerminalSwitchChoice
}

func (service *recordingTerminalCoordinationService) RequestTerminalActivation(target domain.TerminalTarget) (*domain.MasterCoordinationState, error) {
	service.targets = append(service.targets, target)
	if service.order != nil {
		service.order.Add("coordinator:request-terminal-activation:" + target.TerminalID)
	}
	if service.commandErr != nil {
		return domain.CloneMasterCoordinationState(service.state), service.commandErr
	}
	state := domain.CloneMasterCoordinationState(service.state)
	state.Revision++
	if service.decisionRequired {
		state.PendingSwitch = &domain.MasterPendingSwitch{
			SwitchID: service.nextSwitchID, BroadcastID: state.Broadcast.ID,
			SourceTerminalID: *state.Broadcast.ActiveTerminalID, TargetTerminalID: new(target.TerminalID),
		}
		service.state = state
		return domain.CloneMasterCoordinationState(state), nil
	}
	activeTerminalID := target.TerminalID
	state.Broadcast.ActiveTerminalID = &activeTerminalID
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingTerminalCoordinationService) RequestTerminalClear() (*domain.MasterCoordinationState, error) {
	service.clearCalls++
	if service.order != nil {
		service.order.Add("coordinator:request-terminal-clear")
	}
	if service.commandErr != nil {
		return domain.CloneMasterCoordinationState(service.state), service.commandErr
	}
	state := domain.CloneMasterCoordinationState(service.state)
	state.Revision++
	if service.decisionRequired {
		state.PendingSwitch = &domain.MasterPendingSwitch{
			SwitchID: service.nextSwitchID, BroadcastID: state.Broadcast.ID,
			SourceTerminalID: *state.Broadcast.ActiveTerminalID,
		}
		service.state = state
		return domain.CloneMasterCoordinationState(state), nil
	}
	state.Broadcast.ActiveTerminalID = nil
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingTerminalCoordinationService) ResetFailedHack(target domain.TerminalTarget) (*domain.MasterCoordinationState, error) {
	service.resetTargets = append(service.resetTargets, target)
	if service.order != nil {
		service.order.Add("coordinator:reset-failed-hack:" + target.TerminalID)
	}
	if service.commandErr != nil {
		return domain.CloneMasterCoordinationState(service.state), service.commandErr
	}
	state := domain.CloneMasterCoordinationState(service.state)
	state.Revision++
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingTerminalCoordinationService) ResolveTerminalSwitch(switchID domain.SwitchID, decision domain.TerminalSwitchChoice) (*domain.MasterCoordinationState, error) {
	if service.order != nil {
		service.order.Add("coordinator:resolve-terminal-switch:" + string(switchID) + ":" + string(decision))
	}
	if service.commandErr != nil {
		return domain.CloneMasterCoordinationState(service.state), service.commandErr
	}
	service.decisions = append(service.decisions, recordedTerminalDecision{SwitchID: switchID, Decision: decision})
	state := domain.CloneMasterCoordinationState(service.state)
	state.Revision++
	pending := state.PendingSwitch
	state.PendingSwitch = nil
	if decision != domain.TerminalSwitchCancel && pending != nil {
		state.Broadcast.ActiveTerminalID = pending.TargetTerminalID
	}
	service.state = state
	service.decisionRequired = false
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingTerminalCoordinationService) ForceHackSuccess() (*domain.PublicHackState, bool) {
	service.forceCalls++
	if service.forceState == nil {
		return nil, false
	}
	return clonePublicHackState(service.forceState), true
}

func (service *recordingTerminalCoordinationService) UpdateLiveTerminal(tree domain.ContentNode, introText *string) (*domain.MasterCoordinationState, error) {
	service.updateCalls++
	service.updateTree = tree
	if introText != nil {
		intro := *introText
		service.updateIntro = &intro
	}
	terminalID := ""
	if service.state != nil && service.state.Broadcast != nil && service.state.Broadcast.ActiveTerminalID != nil {
		terminalID = *service.state.Broadcast.ActiveTerminalID
	}
	if service.order != nil {
		service.order.Add("coordinator:update-live-terminal:" + terminalID)
	}
	if service.commandErr != nil {
		return domain.CloneMasterCoordinationState(service.state), service.commandErr
	}
	state := domain.CloneMasterCoordinationState(service.state)
	state.Revision++
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

type recordingCorrectionCoordinationService struct {
	recordingCoordinationService
	calls          []string
	updatePayloads []domain.CharacterUpdatePayload
	deletePayloads []domain.CharacterDeletePayload
	nextRevision   int
	failCommand    string
	commandErr     error
	order          *callRecorder
}

func (service *recordingCorrectionCoordinationService) correction(command string) (*domain.MasterCoordinationState, error) {
	if service.failCommand == command {
		return domain.CloneMasterCoordinationState(service.state), service.commandErr
	}
	state := domain.CloneMasterCoordinationState(service.state)
	state.Revision = uint64(20 + service.nextRevision)
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingCorrectionCoordinationService) UpdateCharacter(payload domain.CharacterUpdatePayload) (*domain.MasterCoordinationState, error) {
	service.updatePayloads = append(service.updatePayloads, payload)
	service.calls = append(service.calls, fmt.Sprintf("update-character:%s:%s:%d:%t:%d", payload.CharacterID, payload.Name, payload.Intelligence, payload.HackerPerkAvailable, payload.ExpectedRevision))
	return service.correction("update-character")
}

func (service *recordingCorrectionCoordinationService) DeleteCharacter(payload domain.CharacterDeletePayload) (*domain.MasterCoordinationState, error) {
	service.deletePayloads = append(service.deletePayloads, payload)
	service.calls = append(service.calls, fmt.Sprintf("delete-character:%s:%d", payload.CharacterID, payload.ExpectedRevision))
	return service.correction("delete-character")
}

func (service *recordingCorrectionCoordinationService) RenameLogicalSession(sessionID domain.LogicalSessionID, fallbackName string) (*domain.MasterCoordinationState, error) {
	service.calls = append(service.calls, fmt.Sprintf("rename-session:%s:%s", sessionID, fallbackName))
	return service.correction("rename-session")
}

func (service *recordingCorrectionCoordinationService) AssignCharacter(sessionID domain.LogicalSessionID, characterID domain.CharacterID) (*domain.MasterCoordinationState, error) {
	service.calls = append(service.calls, fmt.Sprintf("assign-character:%s:%s", sessionID, characterID))
	return service.correction("assign-character")
}

func (service *recordingCorrectionCoordinationService) ReleaseCharacter(sessionID domain.LogicalSessionID) (*domain.MasterCoordinationState, error) {
	service.calls = append(service.calls, "release-character:"+string(sessionID))
	return service.correction("release-character")
}

func (service *recordingCorrectionCoordinationService) MoveCharacter(characterID domain.CharacterID, toSessionID domain.LogicalSessionID) (*domain.MasterCoordinationState, error) {
	service.calls = append(service.calls, fmt.Sprintf("move-character:%s:%s", characterID, toSessionID))
	return service.correction("move-character")
}

func (service *recordingCorrectionCoordinationService) SetActiveController(sessionID domain.LogicalSessionID) (*domain.MasterCoordinationState, error) {
	service.calls = append(service.calls, "set-controller:"+string(sessionID))
	if service.order != nil {
		service.order.Add("coordinator:set-controller:" + string(sessionID))
	}
	if service.failCommand == "set-active-controller" {
		return domain.CloneMasterCoordinationState(service.state), service.commandErr
	}
	state := domain.CloneMasterCoordinationState(service.state)
	state.Revision++
	controller := sessionID
	state.Broadcast.ControllerSessionID = &controller
	for index := range state.Sessions {
		if state.Sessions[index].Character == nil {
			state.Sessions[index].Role = domain.PlayerRoleUnassigned
		} else if state.Sessions[index].ID == sessionID {
			state.Sessions[index].Role = domain.PlayerRoleActive
		} else {
			state.Sessions[index].Role = domain.PlayerRoleObserver
		}
	}
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingCoordinatedHackService) ForceHackSuccess() (*domain.PublicHackState, bool) {
	service.forceCalls++
	if service.forceState == nil {
		return nil, false
	}
	clone := *service.forceState
	clone.Log = append([]string(nil), service.forceState.Log...)
	clone.Columns = append([]domain.HackColumn(nil), service.forceState.Columns...)
	clone.Patterns = append([]domain.PublicHackPattern(nil), service.forceState.Patterns...)
	return &clone, true
}

func (service *recordingCoordinationService) Snapshot() *domain.MasterCoordinationState {
	return domain.CloneMasterCoordinationState(service.state)
}

func (service *recordingCoordinationService) AddCharacter(payload domain.CharacterCreatePayload) (*domain.MasterCoordinationState, error) {
	service.addPayloads = append(service.addPayloads, payload)
	if service.addErr != nil {
		return domain.CloneMasterCoordinationState(service.state), service.addErr
	}
	service.state = domain.CloneMasterCoordinationState(service.addState)
	return domain.CloneMasterCoordinationState(service.state), nil
}

func (service *recordingCoordinationService) StartBroadcast() (*domain.MasterCoordinationState, error) {
	service.startCalls++
	if service.startErr != nil {
		return domain.CloneMasterCoordinationState(service.state), service.startErr
	}
	service.state = domain.CloneMasterCoordinationState(service.startState)
	return domain.CloneMasterCoordinationState(service.state), nil
}

func (service *recordingCoordinationService) ResolveTerminalNavigation(requestID string, decision domain.TerminalNavigationDecision) (*domain.MasterCoordinationState, error) {
	service.navigationDecisions = append(service.navigationDecisions, recordedTerminalNavigationDecision{RequestID: requestID, Decision: decision})
	state := domain.CloneMasterCoordinationState(service.state)
	state.Revision++
	state.PendingTerminalNavigation = nil
	service.state = state
	return domain.CloneMasterCoordinationState(state), nil
}

func (service *recordingLiveService) Set(string, string, domain.ContentNode, int, string) *domain.PublicLiveState {
	service.setCalls++
	return clonePublicLiveStateForTest(service.setState)
}

func (service *recordingLiveService) Update(domain.ContentNode, *string) (*domain.PublicLiveState, bool) {
	service.updateCalls++
	return clonePublicLiveStateForTest(service.updateState), service.updateState != nil
}

func (service *recordingLiveService) Clear() {
	service.clearCalls++
}

func (service *recordingLiveService) Snapshot() *domain.PublicLiveState {
	return clonePublicLiveStateForTest(service.setState)
}

func (service *recordingLiveService) ForceHackSuccess() (*domain.PublicHackState, bool) {
	service.forceCalls++
	if service.forceState == nil {
		return nil, false
	}
	clone := *service.forceState
	return &clone, true
}

func clonePublicLiveStateForTest(state *domain.PublicLiveState) *domain.PublicLiveState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.Hack != nil {
		hackClone := *state.Hack
		clone.Hack = &hackClone
	}
	return &clone
}

func (server *recordingPlayerServer) Start(context.Context) (domain.ServerInfo, error) {
	server.recorder.Add("player:start")
	return server.info, server.startErr
}

func (server *recordingPlayerServer) Stop(ctx context.Context) error {
	server.stopContexts = append(server.stopContexts, ctx)
	server.recorder.Add("player:stop")
	return nil
}

func (server *recordingPlayerServer) PublishLive() {
	server.recorder.Add("player:publish-live")
}

func (server *recordingPlayerServer) PublishUpdate() {
	server.recorder.Add("player:publish-update")
}

func (server *recordingPlayerServer) PublishClear() {
	server.recorder.Add("player:publish-clear")
}

func (server *recordingPlayerServer) PublishHack() {
	server.recorder.Add("player:publish-hack")
}

type recordingEventSink struct {
	recorder *callRecorder
	err      error
	mu       sync.Mutex
	records  []eventRecord
}

type eventRecord struct {
	Name    string
	Payload any
}

func (sink *recordingEventSink) Emit(name string, payload any) error {
	sink.recorder.Add("event:" + name)
	sink.mu.Lock()
	sink.records = append(sink.records, eventRecord{Name: name, Payload: payload})
	sink.mu.Unlock()
	return sink.err
}

func (sink *recordingEventSink) Records() []eventRecord {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]eventRecord(nil), sink.records...)
}

type recordingDesktop struct {
	recorder *callRecorder
}

func (desktop *recordingDesktop) Ready(context.Context) error {
	desktop.recorder.Add("desktop:ready")
	return nil
}

func (desktop *recordingDesktop) Close(context.Context) error {
	desktop.recorder.Add("desktop:close")
	return nil
}

type recordingPublicAccessCore struct {
	recorder           *callRecorder
	snapshot           tunnelservice.PublicAccessSnapshot
	start              tunnelservice.PublicAccessResult
	stop               tunnelservice.PublicAccessResult
	reconfigureResults []tunnelservice.PublicAccessResult
	reconfigures       []recordedPublicAccessMutation
	starts             int
	stops              int
	shutdowns          int
	shutdownErrors     []error
	shutdownContexts   []context.Context
}

type recordedPublicAccessMutation struct {
	ExpectedRevision         uint64
	PersistVisibleOverrides  bool
	Preferences              tunnelservice.PublicAccessPreferences
	ProviderReplacementBytes int
	DeleteProviderToken      bool
	PasswordReplacementBytes int
	DeletePlayerPassword     bool
}

func (core *recordingPublicAccessCore) Initialize(context.Context) tunnelservice.PublicAccessSnapshot {
	core.recorder.Add("public:initialize")
	return core.snapshot
}

func (core *recordingPublicAccessCore) Snapshot() tunnelservice.PublicAccessSnapshot {
	return core.snapshot
}

func (core *recordingPublicAccessCore) Start(_ context.Context, _ uint64) tunnelservice.PublicAccessResult {
	core.recorder.Add("public:start")
	core.starts++
	core.snapshot = core.start.Snapshot
	return core.start
}

func (core *recordingPublicAccessCore) Stop(_ context.Context, _ uint64) tunnelservice.PublicAccessResult {
	core.recorder.Add("public:stop")
	core.stops++
	core.snapshot = core.stop.Snapshot
	return core.stop
}

func (core *recordingPublicAccessCore) Reconfigure(_ context.Context, mutation tunnelservice.PublicAccessMutation) tunnelservice.PublicAccessResult {
	core.recorder.Add("public:reconfigure")
	core.reconfigures = append(core.reconfigures, recordedPublicAccessMutation{
		ExpectedRevision: mutation.ExpectedRevision, PersistVisibleOverrides: mutation.PersistVisibleOverrides,
		Preferences:              mutation.Preferences,
		ProviderReplacementBytes: len(mutation.ProviderToken.Replacement), DeleteProviderToken: mutation.ProviderToken.Delete,
		PasswordReplacementBytes: len(mutation.PlayerPassword.Replacement), DeletePlayerPassword: mutation.PlayerPassword.Delete,
	})
	if len(core.reconfigureResults) == 0 {
		return tunnelservice.PublicAccessResult{Error: tunnelservice.ErrorProviderFailure.SafeMessage(), Snapshot: core.snapshot}
	}
	result := core.reconfigureResults[0]
	core.reconfigureResults = core.reconfigureResults[1:]
	core.snapshot = result.Snapshot
	return result
}

func (core *recordingPublicAccessCore) Shutdown(ctx context.Context) error {
	core.recorder.Add("public:shutdown")
	core.shutdowns++
	core.shutdownContexts = append(core.shutdownContexts, ctx)
	if len(core.shutdownErrors) > 0 {
		err := core.shutdownErrors[0]
		core.shutdownErrors = core.shutdownErrors[1:]
		return err
	}
	return nil
}

func TestEmbeddedPublicAccessIsExplicitAfterLocalReadinessAndPublishesOnlyReadyResult(t *testing.T) {
	recorder := &callRecorder{}
	preferences := tunnelservice.DefaultPublicAccessPreferences()
	preferences.Revision = 4
	disabled := tunnelservice.PublicAccessSnapshot{Preferences: preferences, Status: tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, SettingsRevision: 4}}
	ready := disabled
	ready.Status = tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleReady, Generation: 1, SettingsRevision: 4, PublicURL: "https://public.example"}
	core := &recordingPublicAccessCore{
		recorder: recorder, snapshot: disabled,
		start: tunnelservice.PublicAccessResult{OK: true, Snapshot: ready},
		stop:  tunnelservice.PublicAccessResult{OK: true, Snapshot: disabled},
	}
	player := &recordingPlayerServer{recorder: recorder, info: domain.ServerInfo{URL: "http://127.0.0.1:3690", LocalURL: "http://127.0.0.1:3690", Port: 3690}}
	events := &recordingEventSink{recorder: recorder}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Player: player, Events: events, PublicAccess: core})
	require.NoError(t, app.Start(t.Context()))
	assert.Equal(t, 0, core.starts)
	calls := recorder.Calls()
	assert.Less(t, slices.Index(calls, "player:start"), slices.Index(calls, "public:initialize"))

	result := app.StartPublicAccess(PublicAccessCommandPayload{ExpectedRevision: 4})
	require.True(t, result.OK, result.Error)
	assert.Equal(t, "https://public.example", result.Snapshot.Status.PublicURL)
	assert.Equal(t, 1, core.starts)
	records := events.Records()
	require.NotEmpty(t, records)
	assert.Equal(t, publicAccessStatusEvent, records[len(records)-1].Name)
	assert.Equal(t, "https://public.example", records[len(records)-1].Payload.(PublicAccessSnapshot).Status.PublicURL)

	stopped := app.StopPublicAccess(PublicAccessCommandPayload{ExpectedRevision: 4})
	require.True(t, stopped.OK, stopped.Error)
	assert.Empty(t, stopped.Snapshot.Status.PublicURL)
	assert.Equal(t, 1, core.stops)
}

func TestEmbeddedPublicAccessFailurePreservesAuthoritativeLocalServerInfo(t *testing.T) {
	recorder := &callRecorder{}
	preferences := tunnelservice.DefaultPublicAccessPreferences()
	failed := tunnelservice.PublicAccessSnapshot{
		Preferences: preferences,
		Status:      tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleFailed, Generation: 1, ErrorCategory: tunnelservice.ErrorNetworkUnavailable, ErrorMessage: tunnelservice.ErrorNetworkUnavailable.SafeMessage()},
	}
	core := &recordingPublicAccessCore{
		recorder: recorder,
		snapshot: tunnelservice.PublicAccessSnapshot{Preferences: preferences, Status: tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled}},
		start:    tunnelservice.PublicAccessResult{Error: tunnelservice.ErrorNetworkUnavailable.SafeMessage(), Snapshot: failed},
	}
	local := domain.ServerInfo{URL: "http://127.0.0.1:3690", LocalURL: "http://127.0.0.1:3690", Port: 3690}
	app := NewAppWithDependencies(t.Context(), AppDependencies{Player: &recordingPlayerServer{recorder: recorder, info: local}, Events: &recordingEventSink{recorder: recorder}, PublicAccess: core})
	require.NoError(t, app.Start(t.Context()))
	result := app.StartPublicAccess(PublicAccessCommandPayload{})
	require.False(t, result.OK)
	assert.Equal(t, "error", result.Snapshot.Status.State)
	assert.Empty(t, app.GetRuntimeStatus().StartupError)
	require.Eventually(t, func() bool {
		info := app.GetRuntimeStatus().ServerInfo
		return info != nil && *info == local
	}, time.Second, time.Millisecond)
}

func TestEmbeddedPublicAccessFailureMatrixKeepsLocalServerAuthoritative(t *testing.T) {
	categories := []tunnelservice.ErrorCategory{
		tunnelservice.ErrorProviderAuthentication,
		tunnelservice.ErrorNetworkUnavailable,
		tunnelservice.ErrorTimeout,
		tunnelservice.ErrorDomainUnavailable,
		tunnelservice.ErrorSecretStoreLocked,
		tunnelservice.ErrorSecretStoreDenied,
		tunnelservice.ErrorSecretStoreUnavailable,
		tunnelservice.ErrorProviderFailure,
	}
	for index, category := range categories {
		t.Run(fmt.Sprintf("category-%d", index+1), func(t *testing.T) {
			recorder := &callRecorder{}
			preferences := tunnelservice.DefaultPublicAccessPreferences()
			disabled := tunnelservice.PublicAccessSnapshot{
				Preferences: preferences,
				Status:      tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled},
			}
			failed := disabled
			failed.Status = tunnelservice.PublicAccessStatus{
				State: tunnelservice.LifecycleFailed, Generation: 1,
				ErrorCategory: category, ErrorMessage: category.SafeMessage(),
			}
			ready := disabled
			ready.Status = tunnelservice.PublicAccessStatus{
				State: tunnelservice.LifecycleReady, Generation: 2,
				PublicURL: "https://recovered.example",
			}
			core := &recordingPublicAccessCore{
				recorder: recorder, snapshot: disabled,
				start: tunnelservice.PublicAccessResult{Error: category.SafeMessage(), Snapshot: failed},
			}
			local := domain.ServerInfo{
				URL: "http://127.0.0.1:3690", LocalURL: "http://127.0.0.1:3690", Port: 3690,
			}
			app := NewAppWithDependencies(t.Context(), AppDependencies{
				Player: &recordingPlayerServer{recorder: recorder, info: local},
				Events: &recordingEventSink{recorder: recorder}, PublicAccess: core,
			})
			require.NoError(t, app.Start(t.Context()))
			failedResult := app.StartPublicAccess(PublicAccessCommandPayload{})
			require.False(t, failedResult.OK)
			assert.Equal(t, local, *app.GetRuntimeStatus().ServerInfo)

			core.start = tunnelservice.PublicAccessResult{OK: true, Snapshot: ready}
			recovered := app.StartPublicAccess(PublicAccessCommandPayload{})
			require.True(t, recovered.OK, recovered.Error)
			assert.Equal(t, "https://recovered.example", recovered.Snapshot.Status.PublicURL)
			runtimeInfo := app.GetRuntimeStatus().ServerInfo
			require.NotNil(t, runtimeInfo)
			assert.Equal(t, "https://recovered.example", runtimeInfo.URL)
			assert.Equal(t, local.URL, runtimeInfo.LocalURL)
			assert.True(t, runtimeInfo.Tunnel)
			assert.Equal(t, 1, countRecordedCall(recorder.Calls(), "player:start"))
		})
	}
}

type fallbackMatrixSettings struct {
	preferences tunnelservice.PublicAccessPreferences
}

func (settings *fallbackMatrixSettings) Load() (tunnelservice.PublicAccessPreferences, error) {
	return settings.preferences, nil
}

func (settings *fallbackMatrixSettings) Save(preferences tunnelservice.PublicAccessPreferences) error {
	settings.preferences = preferences
	return nil
}

type fallbackMatrixEndpoint struct {
	mu         sync.Mutex
	url        *url.URL
	done       chan struct{}
	doneOnce   sync.Once
	closeErrs  []error
	closeCalls int
	closed     bool
	service    *fallbackMatrixTunnel
}

func (endpoint *fallbackMatrixEndpoint) URL() *url.URL {
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	if endpoint.url == nil {
		return nil
	}
	copyURL := *endpoint.url
	return &copyURL
}

func (endpoint *fallbackMatrixEndpoint) Done() <-chan struct{} { return endpoint.done }

func (endpoint *fallbackMatrixEndpoint) Complete() {
	endpoint.doneOnce.Do(func() { close(endpoint.done) })
}

func (endpoint *fallbackMatrixEndpoint) Close(context.Context) error {
	endpoint.mu.Lock()
	index := endpoint.closeCalls
	endpoint.closeCalls++
	if index < len(endpoint.closeErrs) && endpoint.closeErrs[index] != nil {
		err := endpoint.closeErrs[index]
		endpoint.mu.Unlock()
		return err
	}
	if endpoint.closed {
		endpoint.mu.Unlock()
		return nil
	}
	endpoint.closed = true
	endpoint.url = nil
	endpoint.mu.Unlock()
	endpoint.Complete()
	endpoint.service.closed()
	return nil
}

type fallbackMatrixTunnel struct {
	mu        sync.Mutex
	endpoints []*fallbackMatrixEndpoint
	starts    int
	active    int
	maxActive int
}

func newFallbackMatrixTunnel(closeErrors ...error) *fallbackMatrixTunnel {
	service := &fallbackMatrixTunnel{}
	for index := range 2 {
		endpointURL, err := url.Parse(fmt.Sprintf("https://recovery-%d.example", index+1))
		if err != nil {
			panic(err)
		}
		endpoint := &fallbackMatrixEndpoint{url: endpointURL, done: make(chan struct{}), service: service}
		if index == 0 {
			endpoint.closeErrs = append([]error(nil), closeErrors...)
		}
		service.endpoints = append(service.endpoints, endpoint)
	}
	return service
}

func (service *fallbackMatrixTunnel) Start(_ context.Context, request tunnelservice.TunnelStartRequest) (tunnelservice.TunnelEndpoint, error) {
	defer request.Clear()
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.starts >= len(service.endpoints) {
		return nil, errors.New("synthetic exhausted schedule")
	}
	endpoint := service.endpoints[service.starts]
	service.starts++
	service.active++
	service.maxActive = max(service.maxActive, service.active)
	return endpoint, nil
}

func (service *fallbackMatrixTunnel) closed() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.active--
}

func TestUnexpectedPublicEndpointFailureRetainsCleanupOwnershipBeforeRetryWithoutAppRestart(t *testing.T) {
	recorder := &callRecorder{}
	local := domain.ServerInfo{URL: "http://127.0.0.1:3690", LocalURL: "http://127.0.0.1:3690", Port: 3690}
	settings := &fallbackMatrixSettings{preferences: tunnelservice.DefaultPublicAccessPreferences()}
	secrets := testutil.NewFakeSecretStore()
	require.NoError(t, secrets.Replace(t.Context(), tunnelservice.ProviderAccountToken, []byte("synthetic-account-input")))
	require.NoError(t, secrets.Replace(t.Context(), tunnelservice.PlayerBasicAuthPassword, []byte("synthetic-player-input")))
	service := newFallbackMatrixTunnel(errors.New("synthetic first close failure"), nil)
	ingresses := testutil.NewFakePublicIngressFactory()
	var app *App
	manager, err := tunnelservice.NewPublicAccessManager(tunnelservice.ManagerConfig{
		Settings: settings, Secrets: secrets, Tunnel: service,
		Ingress:     ingresses,
		UpstreamURL: "http://127.0.0.1:3690",
		Publish: func(snapshot tunnelservice.PublicAccessSnapshot) {
			if app != nil {
				app.acceptPublicAccessSnapshot(snapshot, true)
			}
		},
	})
	require.NoError(t, err)
	app = NewAppWithDependencies(t.Context(), AppDependencies{
		Player: &recordingPlayerServer{recorder: recorder, info: local},
		Events: &recordingEventSink{recorder: recorder}, PublicAccess: manager,
	})
	require.NoError(t, app.Start(t.Context()))
	t.Cleanup(func() { assert.Zero(t, ingresses.ActiveIngresses()) })
	t.Cleanup(func() { require.NoError(t, app.Shutdown(context.WithoutCancel(t.Context()))) })
	require.True(t, app.StartPublicAccess(PublicAccessCommandPayload{}).OK)
	service.endpoints[0].Complete()
	require.Eventually(t, func() bool {
		return manager.Snapshot().Status.State == tunnelservice.LifecycleFailed
	}, time.Second, time.Millisecond)
	require.Eventually(t, func() bool {
		info := app.GetRuntimeStatus().ServerInfo
		return info != nil && info.URL == local.URL && info.LocalURL == local.LocalURL && !info.Tunnel && info.TunnelError != ""
	}, time.Second, time.Millisecond)

	recovered := app.StartPublicAccess(PublicAccessCommandPayload{})
	require.True(t, recovered.OK, recovered.Error)
	service.mu.Lock()
	active, maximum := service.active, service.maxActive
	service.mu.Unlock()
	assert.Equal(t, 1, active)
	assert.Equal(t, 1, maximum, "replacement must not overlap an endpoint whose Close needs retry")
	assert.Equal(t, 1, countRecordedCall(recorder.Calls(), "player:start"))
}

func TestApplicationShutdownRetriesOnlyFailedPublicCleanupAfterReleasingLaterResources(t *testing.T) {
	recorder := &callRecorder{}
	preferences := tunnelservice.DefaultPublicAccessPreferences()
	core := &recordingPublicAccessCore{
		recorder: recorder,
		snapshot: tunnelservice.PublicAccessSnapshot{
			Preferences: preferences,
			Status: tunnelservice.PublicAccessStatus{
				State: tunnelservice.LifecycleReady, Generation: 1, PublicURL: "https://public.example",
			},
		},
		shutdownErrors: []error{errors.New("synthetic endpoint cleanup failure"), nil},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		PublicAccess: core,
		Player: &recordingPlayerServer{recorder: recorder, info: domain.ServerInfo{
			URL: "http://127.0.0.1:3690", LocalURL: "http://127.0.0.1:3690", Port: 3690,
		}},
		Sessions: &recordingSessionService{recorder: recorder},
		Desktop:  &recordingDesktop{recorder: recorder},
		Events:   &recordingEventSink{recorder: recorder},
	})
	require.NoError(t, app.Start(t.Context()))
	recorder.Reset()

	first := app.Shutdown(t.Context())
	require.Error(t, first)
	assert.NotContains(t, first.Error(), "https://public.example")
	assert.Equal(t, []string{
		"public:shutdown", "player:stop", "session:shutdown", "desktop:close",
	}, recorder.Calls())

	require.NoError(t, app.Shutdown(t.Context()))
	assert.Equal(t, []string{
		"public:shutdown", "player:stop", "session:shutdown", "desktop:close", "public:shutdown",
	}, recorder.Calls())
	assert.Equal(t, 2, core.shutdowns)
	assert.Equal(t, "stopped", app.lifecyclePhase())
}

func TestActivePublicAccessMutationsDelegateAsProtectedReconfigureWithoutMixedRevision(t *testing.T) {
	recorder := &callRecorder{}
	events := &recordingEventSink{recorder: recorder}
	preferences := tunnelservice.DefaultPublicAccessPreferences()
	preferences.EnabledPreference = true
	preferences.Revision = 7
	ready7 := tunnelservice.PublicAccessSnapshot{
		Preferences: preferences, ProviderTokenPresence: tunnelservice.SecretPresent, PlayerPasswordPresence: tunnelservice.SecretPresent,
		Status: tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleReady, Generation: 3, SettingsRevision: 7, PublicURL: "https://before.example"},
	}
	preferences8 := preferences
	preferences8.ReservedDomain = "after.example"
	preferences8.Username = "friends"
	preferences8.Revision = 8
	ready8 := tunnelservice.PublicAccessSnapshot{
		Preferences: preferences8, ProviderTokenPresence: tunnelservice.SecretPresent, PlayerPasswordPresence: tunnelservice.SecretPresent,
		Status: tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleReady, Generation: 5, SettingsRevision: 8, PublicURL: "https://after.example"},
	}
	preferences9 := preferences8
	preferences9.Revision = 9
	ready9 := ready8
	ready9.Preferences = preferences9
	ready9.Status = tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleReady, Generation: 7, SettingsRevision: 9, PublicURL: "https://after.example"}
	preferences10 := preferences9
	preferences10.Revision = 10
	preferences10.ProviderTokenPresentHint = false
	disabled10 := tunnelservice.PublicAccessSnapshot{
		Preferences: preferences10, ProviderTokenPresence: tunnelservice.SecretAbsent, PlayerPasswordPresence: tunnelservice.SecretPresent,
		Status: tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, Generation: 9, SettingsRevision: 10},
	}
	core := &recordingPublicAccessCore{
		recorder: recorder, snapshot: ready7,
		reconfigureResults: []tunnelservice.PublicAccessResult{
			{OK: true, Snapshot: ready8}, {OK: true, Snapshot: ready9}, {OK: true, Snapshot: disabled10},
		},
		start: tunnelservice.PublicAccessResult{
			Error: tunnelservice.ErrorCredentialMissing.SafeMessage(),
			Snapshot: tunnelservice.PublicAccessSnapshot{
				Preferences: preferences10, ProviderTokenPresence: tunnelservice.SecretAbsent, PlayerPasswordPresence: tunnelservice.SecretPresent,
				Status: tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleFailed, Generation: 10, SettingsRevision: 10, ErrorCategory: tunnelservice.ErrorCredentialMissing, ErrorMessage: tunnelservice.ErrorCredentialMissing.SafeMessage()},
			},
		},
	}
	app := NewAppWithDependencies(t.Context(), AppDependencies{PublicAccess: core, Events: events, PasswordEntropy: strings.NewReader(strings.Repeat("g", 32))})

	saved := app.SavePublicAccessSettings(SavePublicAccessSettingsPayload{
		ExpectedRevision: 7, EnabledPreference: true, ReservedDomain: "after.example", Username: "friends",
		ReplacementProviderToken: "synthetic-provider-rotation", ReplacementPlayerPassword: "synthetic-player-rotation",
	})
	require.True(t, saved.OK, saved.Error)
	assert.Equal(t, uint64(8), saved.Snapshot.Preferences.Revision)
	assert.Equal(t, uint64(8), saved.Snapshot.Status.SettingsRevision)
	assert.Equal(t, "https://after.example", saved.Snapshot.Status.PublicURL)
	require.Len(t, core.reconfigures, 1)
	assert.Equal(t, uint64(7), core.reconfigures[0].ExpectedRevision)
	assert.Equal(t, "after.example", core.reconfigures[0].Preferences.ReservedDomain)
	assert.Equal(t, "friends", core.reconfigures[0].Preferences.Username)
	assert.Positive(t, core.reconfigures[0].ProviderReplacementBytes)
	assert.Positive(t, core.reconfigures[0].PasswordReplacementBytes)
	assert.True(t, core.reconfigures[0].PersistVisibleOverrides)

	generated := app.GeneratePlayerPassword(PublicAccessCommandPayload{ExpectedRevision: 8})
	require.True(t, generated.OK, generated.Error)
	assert.NotEmpty(t, generated.GeneratedPassword)
	assert.Equal(t, uint64(9), generated.SettingsRevision)
	require.Len(t, core.reconfigures, 2)
	assert.Zero(t, core.reconfigures[1].ProviderReplacementBytes)
	assert.GreaterOrEqual(t, core.reconfigures[1].PasswordReplacementBytes, 16)
	assert.False(t, core.reconfigures[1].PersistVisibleOverrides)

	deleted := app.SavePublicAccessSettings(SavePublicAccessSettingsPayload{
		ExpectedRevision: 9, EnabledPreference: true, ReservedDomain: "after.example", Username: "friends", DeleteProviderToken: true,
	})
	require.True(t, deleted.OK, deleted.Error)
	assert.Equal(t, "absent", deleted.Snapshot.ProviderTokenPresence)
	assert.Equal(t, "stopped", deleted.Snapshot.Status.State)
	require.Len(t, core.reconfigures, 3)
	assert.True(t, core.reconfigures[2].DeleteProviderToken)
	assert.True(t, core.reconfigures[2].PersistVisibleOverrides)

	blocked := app.StartPublicAccess(PublicAccessCommandPayload{ExpectedRevision: 10})
	require.False(t, blocked.OK)
	assert.Equal(t, "credential_missing", blocked.Snapshot.Status.ErrorCategory)
	assert.Empty(t, blocked.Snapshot.Status.PublicURL)

	calls := recorder.Calls()
	assert.Less(t, slices.Index(calls, "public:reconfigure"), slices.Index(calls, "event:"+publicAccessStatusEvent))
}

func TestPartialPublicAccessMutationFailureUsesReconciledPresenceAndNeverRestartsMixedRevision(t *testing.T) {
	for _, test := range []struct {
		name             string
		providerPresence tunnelservice.SecretPresence
		passwordPresence tunnelservice.SecretPresence
		providerState    string
		passwordState    string
		category         tunnelservice.ErrorCategory
	}{
		{name: "second Keychain mutation fails", providerPresence: tunnelservice.SecretPresent, passwordPresence: tunnelservice.SecretAbsent, providerState: "present", passwordState: "absent", category: tunnelservice.ErrorSecretStoreDenied},
		{name: "settings commit fails after Keychain mutations", providerPresence: tunnelservice.SecretPresent, passwordPresence: tunnelservice.SecretPresent, providerState: "present", passwordState: "present", category: tunnelservice.ErrorSettingsCorrupt},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &callRecorder{}
			events := &recordingEventSink{recorder: recorder}
			preferences := tunnelservice.DefaultPublicAccessPreferences()
			preferences.Revision = 7
			failed := tunnelservice.PublicAccessSnapshot{
				Preferences: preferences, ProviderTokenPresence: test.providerPresence, PlayerPasswordPresence: test.passwordPresence,
				Status: tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleFailed, Generation: 5, SettingsRevision: 7, ErrorCategory: test.category, ErrorMessage: test.category.SafeMessage()},
			}
			core := &recordingPublicAccessCore{
				recorder: recorder,
				snapshot: tunnelservice.PublicAccessSnapshot{
					Preferences: preferences, ProviderTokenPresence: tunnelservice.SecretPresent, PlayerPasswordPresence: tunnelservice.SecretPresent,
					Status: tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleReady, Generation: 3, SettingsRevision: 7, PublicURL: "https://before.example"},
				},
				reconfigureResults: []tunnelservice.PublicAccessResult{{Error: test.category.SafeMessage(), Snapshot: failed}},
			}
			app := NewAppWithDependencies(t.Context(), AppDependencies{PublicAccess: core, Events: events})
			result := app.SavePublicAccessSettings(SavePublicAccessSettingsPayload{
				ExpectedRevision: 7, EnabledPreference: true, Username: "friends",
				ReplacementProviderToken: "synthetic-provider-rotation", ReplacementPlayerPassword: "synthetic-player-rotation",
			})
			require.False(t, result.OK)
			assert.Equal(t, test.category.SafeMessage(), result.Error)
			assert.Equal(t, uint64(7), result.Snapshot.Preferences.Revision)
			assert.Equal(t, uint64(7), result.Snapshot.Status.SettingsRevision)
			assert.Empty(t, result.Snapshot.Status.PublicURL)
			assert.Equal(t, test.providerState, result.Snapshot.ProviderTokenPresence)
			assert.Equal(t, test.passwordState, result.Snapshot.PlayerPasswordPresence)
			assert.Zero(t, core.starts)
			require.NotEmpty(t, events.Records())
			assert.Equal(t, result.Snapshot, events.Records()[len(events.Records())-1].Payload)
		})
	}
}

func TestPublicAccessCompositionUsesDirectExistingPlayerTarget(t *testing.T) {
	route := publicAccessCompositionRoute()
	assert.Equal(t, tunnelservice.PlayerUpstreamAddress, route.PlayerTarget)
	assert.Equal(t, "http://127.0.0.1:3690", route.UpstreamURL)
}

func TestPublicAccessEnvironmentOverrideCompositionIsDevelopmentOnlyAndNeverAutoStarts(t *testing.T) {
	settings := &fallbackMatrixSettings{preferences: tunnelservice.DefaultPublicAccessPreferences()}
	secrets := testutil.NewFakeSecretStore()
	lookupCalls := 0
	providerCanary := "generated-" + strings.Repeat("t", 32)
	passwordCanary := "generated-" + strings.Repeat("w", 16)
	lookup := func(name string) (string, bool) {
		lookupCalls++
		values := map[string]string{
			tunnelservice.DevelopmentNgrokAuthtokenEnvironment: providerCanary,
			tunnelservice.DevelopmentNgrokDomainEnvironment:    "override.example",
			tunnelservice.DevelopmentPlayerUsernameEnvironment: "override-players",
			tunnelservice.DevelopmentPlayerPasswordEnvironment: passwordCanary,
		}
		value, ok := values[name]
		return value, ok
	}

	developmentSettings, developmentSecrets := publicAccessStoresForProfile(settings, secrets, false, lookup)
	service := testutil.NewFakeTunnelService(testutil.NewFakeTunnelEndpoint("https://override.example"))
	ingresses := testutil.NewFakePublicIngressFactory()
	manager, err := tunnelservice.NewPublicAccessManager(tunnelservice.ManagerConfig{
		Settings: developmentSettings, Secrets: developmentSecrets, Tunnel: service,
		Ingress: ingresses, UpstreamURL: publicAccessCompositionRoute().UpstreamURL,
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.Zero(t, ingresses.ActiveIngresses()) })
	t.Cleanup(func() { require.NoError(t, manager.Shutdown(context.WithoutCancel(t.Context()))) })
	snapshot := manager.Initialize(t.Context())
	assert.Equal(t, "override.example", snapshot.Preferences.ReservedDomain)
	assert.Equal(t, "override-players", snapshot.Preferences.Username)
	assert.Equal(t, tunnelservice.SecretPresent, snapshot.ProviderTokenPresence)
	assert.Equal(t, tunnelservice.SecretPresent, snapshot.PlayerPasswordPresence)
	assert.Equal(t, tunnelservice.LifecycleDisabled, snapshot.Status.State)
	assert.Zero(t, service.StartCalls(), "environment loading must not auto-start public access")
	assert.Positive(t, lookupCalls)
	rawSnapshot, err := json.Marshal(snapshot)
	require.NoError(t, err)
	assert.NotContains(t, string(rawSnapshot), providerCanary)
	assert.NotContains(t, string(rawSnapshot), passwordCanary)
	underlyingProvider, err := secrets.Presence(t.Context(), tunnelservice.ProviderAccountToken)
	require.NoError(t, err)
	underlyingPassword, err := secrets.Presence(t.Context(), tunnelservice.PlayerBasicAuthPassword)
	require.NoError(t, err)
	assert.Equal(t, tunnelservice.SecretAbsent, underlyingProvider)
	assert.Equal(t, tunnelservice.SecretAbsent, underlyingPassword)

	app := NewAppWithDependencies(t.Context(), AppDependencies{
		PublicAccess: manager, PasswordEntropy: strings.NewReader(strings.Repeat("g", 32)),
	})
	generated := app.GeneratePlayerPassword(PublicAccessCommandPayload{ExpectedRevision: snapshot.Preferences.Revision})
	require.True(t, generated.OK, generated.Error)
	assert.NotEmpty(t, generated.GeneratedPassword)
	assert.Equal(t, "", settings.preferences.ReservedDomain, "Generate must not persist the environment domain")
	assert.Equal(t, "players", settings.preferences.Username, "Generate must not persist the environment username")
	assert.Equal(t, generated.SettingsRevision, settings.preferences.Revision)

	saved := app.SavePublicAccessSettings(SavePublicAccessSettingsPayload{
		ExpectedRevision: generated.SettingsRevision, ReservedDomain: "override.example", Username: "override-players",
	})
	require.True(t, saved.OK, saved.Error)
	assert.Equal(t, "override.example", settings.preferences.ReservedDomain)
	assert.Equal(t, "override-players", settings.preferences.Username)

	started := manager.Start(t.Context(), saved.Snapshot.Preferences.Revision)
	require.True(t, started.OK, started.Error)
	assert.Equal(t, tunnelservice.LifecycleReady, started.Snapshot.Status.State)
	assert.Equal(t, 1, service.StartCalls(), "only explicit Start may consume the override")
	underlyingProvider, err = secrets.Presence(t.Context(), tunnelservice.ProviderAccountToken)
	require.NoError(t, err)
	underlyingPassword, err = secrets.Presence(t.Context(), tunnelservice.PlayerBasicAuthPassword)
	require.NoError(t, err)
	assert.Equal(t, tunnelservice.SecretAbsent, underlyingProvider)
	assert.Equal(t, tunnelservice.SecretPresent, underlyingPassword)

	lookupCalls = 0
	productionSettings, productionSecrets := publicAccessStoresForProfile(settings, secrets, true, lookup)
	assert.Same(t, settings, productionSettings)
	assert.Same(t, secrets, productionSecrets)
	assert.Zero(t, lookupCalls, "packaged production must not consult the development environment")
}
