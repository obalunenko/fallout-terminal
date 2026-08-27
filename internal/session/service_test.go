package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testCampaignsRoot = testAbsolutePath("Volumes", "Campaigns")
	testLocations     = Locations{
		DocumentsDefault:   testAbsolutePath("Users", "test", "Documents", "Fallout Terminal", "Sessions"),
		BundledDemo:        testAbsolutePath("Applications", "Fallout Terminal.app", "Contents", "Resources", "sessions", "demo.json"),
		ApplicationSupport: testAbsolutePath("Users", "test", "Library", "Application Support", "com.vaulttec.fallout-terminal"),
	}
)

func testAbsolutePath(elements ...string) string {
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = `C:\`
	}
	return filepath.Join(append([]string{root}, elements...)...)
}

func TestCanceledSessionDialogsDoNotChangeStateOrFilesystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		invoke      func(context.Context, *Service) SessionResult
		wantKind    string
		wantDefault string
	}{
		{
			name:        "new session",
			invoke:      func(ctx context.Context, service *Service) SessionResult { return service.Create(ctx) },
			wantKind:    "save",
			wantDefault: filepath.Join(testLocations.DocumentsDefault, "session.json"),
		},
		{
			name:        "open session",
			invoke:      func(ctx context.Context, service *Service) SessionResult { return service.Open(ctx) },
			wantKind:    "open",
			wantDefault: testLocations.DocumentsDefault,
		},
		{
			name:        "copy bundled demo",
			invoke:      func(ctx context.Context, service *Service) SessionResult { return service.CopyDemo(ctx) },
			wantKind:    "save",
			wantDefault: filepath.Join(testLocations.DocumentsDefault, "demo.json"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fileSystem := testutil.NewFakeFileSystem()
			dialog := &testutil.FakeDialog{}
			service := NewService(NewStorage(fileSystem), dialog, testLocations)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

			result := test.invoke(t.Context(), service)
			assert.False(t, result.OK)
			assert.True(t, result.Canceled)
			assert.Empty(t, result.Error)
			assert.Empty(t, result.FilePath)
			assert.Nil(t, result.Session)
			assert.Equal(t, []testutil.DialogCall{{Kind: test.wantKind, DefaultPath: test.wantDefault}}, dialog.Calls())
			assertInactive(t, service.Snapshot())
			assert.Empty(t, fileSystem.MkdirCalls(), "cancellation created directories")
			assert.Empty(t, fileSystem.WriteCalls(), "cancellation wrote files")
		})
	}
}

func TestCreateUsesDocumentsSuggestionAndActivatesChosenPathAfterWrite(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "My Wasteland.json")
	dialog := &testutil.FakeDialog{SaveResult: target}
	service := NewService(NewStorage(fileSystem), dialog, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	result := service.Create(t.Context())
	require.True(t, result.OK, "Create() = %#v", result)
	assert.False(t, result.Canceled)
	assert.Empty(t, result.Error)
	assert.Equal(t, target, result.FilePath)
	require.NotNil(t, result.Session)
	assert.Equal(t, "My Wasteland", result.Session.Name)
	assert.Equal(t, 1, result.Session.Version)
	assert.Len(t, result.Session.Terminals, 1)
	require.NoError(t, domain.ValidateSession(*result.Session))
	written, ok := fileSystem.File(target)
	require.True(t, ok, "chosen target %q was not written", target)
	assert.True(t, bytes.HasSuffix(written, []byte("\n")), "created JSON must have a final newline")
	assert.NotContains(t, string(written), "\t", "created JSON must not contain tabs")
	snapshot := service.Snapshot()
	assert.Equal(t, target, snapshot.Path)
	require.NotNil(t, snapshot.Session)
	assert.Equal(t, "My Wasteland", snapshot.Session.Name)
	assertNoApplicationSupportWrites(t, fileSystem)
}

func TestCreateAddsSingletonGroupForStarterTerminal(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "grouped-new-session.json")
	service := NewService(
		NewStorage(fileSystem),
		&testutil.FakeDialog{SaveResult: target},
		testLocations,
	)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	created := service.Create(t.Context())
	require.True(t, created.OK, "Create() = %#v", created)
	require.NotNil(t, created.Session)
	assertCanonicalTerminalGroups(t, *created.Session)
	require.Len(t, created.Session.TerminalGroups, 1)
	assert.Equal(t, []string{created.Session.Terminals[0].ID}, created.Session.TerminalGroups[0].TerminalIDs)

	written, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, created.Session.TerminalGroups, written.TerminalGroups)
}

func TestOpenNormalizesLegacyTerminalsIntoStableSingletonGroupsWithoutWriting(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "legacy-singletons.json")
	raw := []byte(`{
  "version": 1,
  "name": "Legacy terminals",
  "terminals": [
    {
      "id": "legacy-a",
      "name": "Terminal",
      "hackLevel": 0,
      "introText": "",
      "root": {"id":"root","type":"folder","name":"ROOT","children":[]}
    },
    {
      "id": "legacy-b",
      "name": "Terminal",
      "hackLevel": 0,
      "introText": "",
      "root": {"id":"root","type":"folder","name":"ROOT","children":[]}
    }
  ]
}`)
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, raw)

	open := func(t *testing.T) domain.Session {
		t.Helper()
		service := NewService(
			NewStorage(fileSystem),
			&testutil.FakeDialog{OpenResult: target},
			testLocations,
		)
		opened := service.Open(t.Context())
		require.True(t, opened.OK, "Open() = %#v", opened)
		require.NotNil(t, opened.Session)
		require.NoError(t, service.Shutdown(context.WithoutCancel(t.Context())))

		return *opened.Session
	}

	first := open(t)
	assertCanonicalTerminalGroups(t, first)
	require.Len(t, first.TerminalGroups, 2)
	for index, terminal := range first.Terminals {
		assert.Equal(t, []string{terminal.ID}, first.TerminalGroups[index].TerminalIDs)
		assert.Contains(t, strings.ToLower(first.TerminalGroups[index].Name), strings.ToLower(terminal.Name))
	}
	assert.NotEqual(t, first.TerminalGroups[0].Name, first.TerminalGroups[1].Name,
		"duplicate terminal names require collision-safe group names")

	second := open(t)
	assert.Equal(t, first.TerminalGroups, second.TerminalGroups,
		"equivalent legacy opens must produce stable singleton identities")
	assert.Equal(t, raw, fileSystemFileData(t, fileSystem, target),
		"opening a legacy document must not persist normalization")
	assert.Empty(t, fileSystem.WriteCalls(), "opening a legacy document wrote normalized groups")
}

func TestSaveAndReopenPreservesThreeGroupsAndTenTerminalMemberships(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "ordered-groups.json")
	canonical := domain.Session{
		Version: 1,
		Name:    "Ordered groups",
		Terminals: []domain.Terminal{
			terminalForGroupTest("a", "Alpha"),
			terminalForGroupTest("b", "Beta"),
			terminalForGroupTest("c", "Gamma"),
			terminalForGroupTest("d", "Delta"),
			terminalForGroupTest("e", "Epsilon"),
			terminalForGroupTest("f", "Zeta"),
			terminalForGroupTest("g", "Eta"),
			terminalForGroupTest("h", "Theta"),
			terminalForGroupTest("i", "Iota"),
			terminalForGroupTest("j", "Kappa"),
		},
		TerminalGroups: []domain.TerminalGroup{
			{ID: "secondary", Name: "Secondary", TerminalIDs: []string{"c", "a", "f"}},
			{ID: "primary", Name: "Primary", TerminalIDs: []string{"b", "j", "d", "e"}},
			{ID: "archive", Name: "Archive", TerminalIDs: []string{"i", "h", "g"}},
		},
	}
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, mustEncodeSession(t, canonical))

	service := NewService(
		NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		testLocations,
	)
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	edited := domain.CloneSession(*opened.Session)
	edited.Name = "Ordered groups after save"
	saved := service.Save(t.Context(), edited, 1)
	require.True(t, saved.OK, "Save() = %#v", saved)
	require.NoError(t, service.Shutdown(context.WithoutCancel(t.Context())))

	restarted := NewService(
		NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		testLocations,
	)
	t.Cleanup(func() { _ = restarted.Shutdown(context.WithoutCancel(t.Context())) })
	reopened := restarted.Open(t.Context())
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.NotNil(t, reopened.Session)
	require.Len(t, reopened.Session.TerminalGroups, 3)
	require.Len(t, reopened.Session.Terminals, 10)
	assert.Equal(t, canonical.TerminalGroups, reopened.Session.TerminalGroups)
	assert.Equal(t, canonical.Terminals, reopened.Session.Terminals)
	assertCanonicalTerminalGroups(t, *reopened.Session)
}

func TestReplaceTerminalGroupsRepairsLegacyTransitionAndReopensExactCandidate(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "legacy-transition-repair.json")
	legacy := linkedSessionForTest()
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, mustEncodeSession(t, legacy))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)

	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	require.Len(t, opened.Session.TerminalGroups, 2)
	sourceGroup := terminalGroupByMember(t, opened.Session.TerminalGroups, "a")
	targetGroup := terminalGroupByMember(t, opened.Session.TerminalGroups, "b")
	candidate := []domain.TerminalGroup{{
		ID: sourceGroup.ID, Name: sourceGroup.Name, TerminalIDs: []string{"a", "b"},
	}}

	replaced := service.ReplaceTerminalGroups(t.Context(), candidate, 0)
	require.True(t, replaced.OK, "ReplaceTerminalGroups() = %#v", replaced)
	require.True(t, replaced.Changed)
	require.NotNil(t, replaced.Session)
	assert.Equal(t, candidate, replaced.Session.TerminalGroups)
	assert.NotEqual(t, targetGroup.ID, replaced.Session.TerminalGroups[0].ID)
	require.Equal(t, "go", replaced.Session.Terminals[0].Root.Children[0].ID)
	require.Equal(t, "b", replaced.Session.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID)
	require.NoError(t, service.Shutdown(context.WithoutCancel(t.Context())))

	restarted := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = restarted.Shutdown(context.WithoutCancel(t.Context())) })
	reopened := restarted.Open(t.Context())
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.NotNil(t, reopened.Session)
	assert.Equal(t, candidate, reopened.Session.TerminalGroups)
	assert.Equal(t, legacy.Terminals, reopened.Session.Terminals, "repair must preserve terminal content and command identity")
	transition, ok := restarted.LookupTerminalTransition("a", "go")
	require.True(t, ok, "repaired same-group transition must become eligible after reopen")
	assert.Equal(t, "b", transition.Target.TerminalID)
}

func TestReplaceTerminalGroupsClassifiesAndRepairsMultiLinkLegacyFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "session-05-cold-storage.json"))
	require.NoError(t, err)
	testReplaceTerminalGroupsClassifiesAndRepairsMultiLinkLegacyDocument(t, raw)
}

func TestReplaceTerminalGroupsClassifiesAndRepairsExactAuthoredMultiLinkLegacyDocument(t *testing.T) {
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
	testReplaceTerminalGroupsClassifiesAndRepairsMultiLinkLegacyDocument(t, raw)
}

func testReplaceTerminalGroupsClassifiesAndRepairsMultiLinkLegacyDocument(t *testing.T, raw []byte) {
	t.Helper()

	target := filepath.Join(testCampaignsRoot, "session-05-cold-storage.json")
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, raw)
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	require.Len(t, opened.Session.TerminalGroups, 3)
	beforeRevision := service.Snapshot().SavedRevision
	serviceGroup := terminalGroupByMember(t, opened.Session.TerminalGroups, "t-krel-service")
	emergencyGroup := terminalGroupByMember(t, opened.Session.TerminalGroups, "t-krel-emergency")
	repairServiceToAdmin := []domain.TerminalGroup{
		{
			ID: serviceGroup.ID, Name: serviceGroup.Name,
			TerminalIDs: []string{"t-krel-service", "t-krel-admin"},
		},
		emergencyGroup,
	}

	partial := service.ReplaceTerminalGroups(t.Context(), repairServiceToAdmin, 0)
	require.False(t, partial.OK)
	assert.Equal(t, beforeRevision, partial.Revision)
	assert.NotContains(t, partial.Error, `command "svc-access-admin"`)
	assert.Contains(t, partial.Error, `command "adm-emergency"`)
	require.NotNil(t, partial.Session)
	assert.Equal(t, opened.Session.TerminalGroups, partial.Session.TerminalGroups)
	assert.Equal(t, beforeRevision, service.Snapshot().SavedRevision)

	complete := []domain.TerminalGroup{{
		ID: serviceGroup.ID, Name: serviceGroup.Name,
		TerminalIDs: []string{"t-krel-service", "t-krel-admin", "t-krel-emergency"},
	}}
	replaced := service.ReplaceTerminalGroups(t.Context(), complete, 0)
	require.True(t, replaced.OK, "ReplaceTerminalGroups() = %#v", replaced)
	require.NotNil(t, replaced.Session)
	assert.Equal(t, beforeRevision+1, replaced.Revision)
	assert.Equal(t, complete, replaced.Session.TerminalGroups)
	assert.Equal(t, opened.Session.Terminals, replaced.Session.Terminals)
	assert.Equal(t, opened.Session.PlayerConfig, replaced.Session.PlayerConfig)
	durableRaw, err := fileSystem.ReadFile(target)
	require.NoError(t, err)
	durable, err := domain.DecodeSession(durableRaw)
	require.NoError(t, err)
	assert.Equal(t, complete, durable.TerminalGroups)
	assert.Equal(t, opened.Session.Terminals, durable.Terminals)
	assert.Equal(t, opened.Session.PlayerConfig, durable.PlayerConfig)
	require.NoError(t, service.Shutdown(context.WithoutCancel(t.Context())))

	restarted := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = restarted.Shutdown(context.WithoutCancel(t.Context())) })
	reopened := restarted.Open(t.Context())
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.NotNil(t, reopened.Session)
	assert.Equal(t, complete, reopened.Session.TerminalGroups)
	assert.Equal(t, opened.Session.Terminals, reopened.Session.Terminals)
	assert.Equal(t, opened.Session.PlayerConfig, reopened.Session.PlayerConfig)

	serviceToAdmin, ok := restarted.LookupTerminalTransition("t-krel-service", "svc-access-admin")
	require.True(t, ok)
	assert.Equal(t, "t-krel-admin", serviceToAdmin.Target.TerminalID)
	adminToEmergency, ok := restarted.LookupTerminalTransition("t-krel-admin", "adm-emergency")
	require.True(t, ok)
	assert.Equal(t, "t-krel-emergency", adminToEmergency.Target.TerminalID)
}

func TestGenericSaveNormalizesTerminalLifecycleAndPreservesCanonicalMembership(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "terminal-lifecycle-groups.json")
	canonical := domain.Session{
		Version: 1,
		Name:    "Terminal lifecycle",
		Terminals: []domain.Terminal{
			terminalForGroupTest("a", "Alpha"),
			terminalForGroupTest("b", "Beta"),
			terminalForGroupTest("deleted", "Deleted"),
		},
		TerminalGroups: []domain.TerminalGroup{
			{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b"}},
			{ID: "deleted-singleton", Name: "Deleted", TerminalIDs: []string{"deleted"}},
		},
	}
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, mustEncodeSession(t, canonical))
	service := NewService(
		NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		testLocations,
	)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)

	candidate := domain.CloneSession(*opened.Session)
	candidate.Terminals = append([]domain.Terminal(nil), candidate.Terminals[:2]...)
	candidate.Terminals = append(candidate.Terminals,
		terminalForGroupTest("created", "Created terminal"),
		terminalForGroupTest("imported", "Imported terminal"),
	)
	candidate.TerminalGroups = []domain.TerminalGroup{
		{ID: "stale-membership-a", Name: "Stale A", TerminalIDs: []string{"b", "created"}},
		{ID: "stale-membership-b", Name: "Stale B", TerminalIDs: []string{"a", "imported"}},
	}

	saved := service.Save(t.Context(), candidate, 1)
	require.True(t, saved.OK, "Save() = %#v", saved)
	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assertCanonicalTerminalGroups(t, *active.Session)

	route := terminalGroupByMember(t, active.Session.TerminalGroups, "a")
	assert.Equal(t, domain.TerminalGroup{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b"}}, route,
		"generic save replaced canonical membership or member order")
	newTerminalNames := map[string]string{
		"created":  "Created terminal",
		"imported": "Imported terminal",
	}
	for terminalID, terminalName := range newTerminalNames {
		group := terminalGroupByMember(t, active.Session.TerminalGroups, terminalID)
		assert.Equal(t, []string{terminalID}, group.TerminalIDs)
		assert.Contains(t, strings.ToLower(group.Name), strings.ToLower(terminalName))
		assert.NotContains(t, []string{"stale-membership-a", "stale-membership-b"}, group.ID)
	}
	for _, group := range active.Session.TerminalGroups {
		assert.NotContains(t, group.TerminalIDs, "deleted")
		assert.NotEqual(t, "deleted-singleton", group.ID)
	}

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, active.Session.TerminalGroups, persisted.TerminalGroups)
}

func TestStaleGenericSavePreservesLatestGroupsAcrossContractAndReopen(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "stale-groups-round-trip.json")
	initial := terminalGroupMutationTestSession([]domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
	})
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)

	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	stale := domain.CloneSession(*opened.Session)
	latestGroups := []domain.TerminalGroup{
		{ID: "outer", Name: "Outer", TerminalIDs: []string{"a", "c"}},
		{ID: "inner", Name: "Inner", TerminalIDs: []string{"b"}},
	}
	replaced := service.ReplaceTerminalGroups(t.Context(), latestGroups, 0)
	require.True(t, replaced.OK, "ReplaceTerminalGroups() = %#v", replaced)
	assert.Equal(t, uint64(1), replaced.Revision)

	stale.Name = "Authored after regrouping"
	stale.Terminals[0].IntroText = "Unrelated authored edit"
	saved := service.Save(t.Context(), stale, 2)
	require.True(t, saved.OK, "Save(stale groups) = %#v", saved)
	assert.Equal(t, uint64(2), saved.SavedRevision)
	require.NoError(t, service.Shutdown(context.WithoutCancel(t.Context())))

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, "Authored after regrouping", persisted.Name)
	assert.Equal(t, "Unrelated authored edit", persisted.Terminals[0].IntroText)
	assert.Equal(t, latestGroups, persisted.TerminalGroups,
		"a stale generic save must not restore its obsolete group set")
	semantic, err := SessionToProto(persisted)
	require.NoError(t, err)
	contractRoundTrip, err := SessionFromProto(semantic, persisted)
	require.NoError(t, err)
	assert.Equal(t, persisted, contractRoundTrip)

	restarted := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = restarted.Shutdown(context.WithoutCancel(t.Context())) })
	reopened := restarted.Open(t.Context())
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.NotNil(t, reopened.Session)
	assert.Equal(t, persisted, *reopened.Session)
}

func TestLegacyDormantLinkSurvivesTerminalCreateAndDelete(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "legacy-dormant-link-lifecycle.json")
	legacy := linkedSessionForTest()
	legacy.TerminalGroups = nil
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, mustEncodeSession(t, legacy))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	assertCanonicalTerminalGroups(t, *opened.Session)
	_, eligible := service.LookupTerminalTransition("a", "go")
	assert.False(t, eligible, "legacy cross-singleton link must stay dormant")

	created := domain.CloneSession(*opened.Session)
	created.Terminals = append(created.Terminals, terminalForGroupTest("created", "Created"))
	created.TerminalGroups = []domain.TerminalGroup{
		{ID: "stale-route", Name: "Stale route", TerminalIDs: []string{"a", "b", "created"}},
	}
	require.True(t, service.Save(t.Context(), created, 1).OK)
	afterCreate := service.Snapshot()
	require.NotNil(t, afterCreate.Session)
	assert.Equal(t, []string{"created"}, terminalGroupByMember(t, afterCreate.Session.TerminalGroups, "created").TerminalIDs)
	assertCanonicalTerminalGroups(t, *afterCreate.Session)

	deleted := domain.CloneSession(*afterCreate.Session)
	deleted.Terminals = append([]domain.Terminal(nil), deleted.Terminals[:2]...)
	require.True(t, service.Save(t.Context(), deleted, 2).OK)
	afterDelete := service.Snapshot()
	require.NotNil(t, afterDelete.Session)
	assertCanonicalTerminalGroups(t, *afterDelete.Session)
	assert.Len(t, afterDelete.Session.TerminalGroups, 2)
	assert.Equal(t, "b", afterDelete.Session.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID)
	_, eligible = service.LookupTerminalTransition("a", "go")
	assert.False(t, eligible, "ordinary lifecycle saves must not activate a dormant legacy link")

	persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, *afterDelete.Session, persisted)
}

func TestGenericSaveRejectsNewOrRetargetedCrossGroupTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*domain.Session)
	}{
		{
			name: "new transition",
			mutate: func(candidate *domain.Session) {
				candidate.Terminals[0].Root.Children = append(candidate.Terminals[0].Root.Children,
					domain.ContentNode{
						ID:   "skip-to-gamma",
						Type: domain.NodeCommand,
						Name: "Skip to Gamma",
						TerminalTransition: &domain.TerminalTransitionConfig{
							TargetTerminalID: "c",
						},
					},
				)
			},
		},
		{
			name: "retargeted transition",
			mutate: func(candidate *domain.Session) {
				candidate.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID = "c"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target := filepath.Join(testCampaignsRoot, strings.ReplaceAll(test.name, " ", "-")+".json")
			initial := linkedSessionForTest()
			initial.Terminals = append(initial.Terminals, terminalForGroupTest("c", "Gamma"))
			initial.TerminalGroups = []domain.TerminalGroup{
				{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b"}},
				{ID: "gamma", Name: "Gamma", TerminalIDs: []string{"c"}},
			}
			fileSystem := testutil.NewFakeFileSystem()
			fileSystem.SeedFile(target, mustEncodeSession(t, initial))
			service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

			opened := service.Open(t.Context())
			require.True(t, opened.OK, "Open() = %#v", opened)
			require.NotNil(t, opened.Session)
			candidate := domain.CloneSession(*opened.Session)
			test.mutate(&candidate)
			writesBefore := len(fileSystem.WriteCalls())

			result := service.Save(t.Context(), candidate, 1)

			assert.False(t, result.OK)
			assert.Contains(t, result.Error, "session is invalid")
			assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
			active := service.Snapshot()
			require.NotNil(t, active.Session)
			assert.Equal(t, initial, *active.Session)
			persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
			require.NoError(t, err)
			assert.Equal(t, initial, persisted)
		})
	}
}

func TestFailedCommandStateMutationRollsBackGroupedSessionAndRevision(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "grouped-command-state-rollback.json")
	initial := stateChangingSession("Grouped rollback")
	initial.TerminalGroups = []domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{"t1", "t2"}},
	}
	initialData := mustEncodeSession(t, initial)
	store := &failingMutationStore{
		path: target,
		data: initialData,
		err:  fmt.Errorf("injected grouped command-state failure"),
	}
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)

	result := service.ExecuteCommandState(t.Context(), "t1", "doors")
	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.Zero(t, result.Revision)
	assert.Nil(t, result.Session)
	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Equal(t, initial, *active.Session)
	assert.Zero(t, active.RequestedRevision)
	assert.Zero(t, active.SavedRevision)
	assert.Equal(t, SaveStateFailed, active.SaveState)
	assert.Equal(t, initialData, store.data)
}

func TestCoalescedGenericSavesKeepNewestContentAndCanonicalGroups(t *testing.T) {
	target := filepath.Join(testCampaignsRoot, "coalesced-grouped-saves.json")
	canonicalGroups := []domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b"}},
		{ID: "gamma", Name: "Gamma", TerminalIDs: []string{"c"}},
	}
	initial := terminalGroupMutationTestSession(canonicalGroups)
	store := newBlockingStore()
	store.seed(target, mustEncodeSession(t, initial))
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)

	type completion struct {
		revision uint64
		result   SaveResult
	}
	completions := make(chan completion, 3)
	startSave := func(revision uint64) {
		candidate := domain.CloneSession(*opened.Session)
		candidate.Name = fmt.Sprintf("revision-%d", revision)
		candidate.TerminalGroups = []domain.TerminalGroup{
			{ID: fmt.Sprintf("stale-%d", revision), Name: "Stale", TerminalIDs: []string{"c", "b", "a"}},
		}
		go func() {
			completions <- completion{revision: revision, result: service.Save(t.Context(), candidate, revision)}
		}()
	}

	startSave(1)
	select {
	case <-store.firstWriteStarted:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "first grouped revision did not begin writing")
	}
	t.Cleanup(store.release)
	for revision := uint64(2); revision <= 3; revision++ {
		startSave(revision)
		waitForRequestedRevision(t, service, revision)
	}
	store.release()

	for range 3 {
		select {
		case completed := <-completions:
			assert.True(t, completed.result.OK, "revision %d result = %#v", completed.revision, completed.result)
			assert.Equal(t, completed.revision, completed.result.RequestedRevision)
			assert.GreaterOrEqual(t, completed.result.SavedRevision, completed.revision)
		case <-time.After(2 * time.Second):
			require.FailNow(t, "grouped saves did not finish")
		}
	}

	persisted, err := domain.DecodeSession(store.file(target))
	require.NoError(t, err)
	assert.Equal(t, "revision-3", persisted.Name)
	assert.Equal(t, canonicalGroups, persisted.TerminalGroups)
	assertCanonicalTerminalGroups(t, persisted)
	active := service.Snapshot()
	assert.Equal(t, uint64(3), active.RequestedRevision)
	assert.Equal(t, uint64(3), active.SavedRevision)
	assert.Equal(t, SaveStateSaved, active.SaveState)
}

func TestReplaceTerminalGroupsAppliesDissolutionAndMoveCandidatesAtomically(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		initial   []domain.TerminalGroup
		candidate []domain.TerminalGroup
	}{
		{
			name: "dissolve group into content-preserving singletons",
			initial: []domain.TerminalGroup{
				{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
			},
			candidate: []domain.TerminalGroup{
				{ID: "singleton-a", Name: "Alpha", TerminalIDs: []string{"a"}},
				{ID: "singleton-b", Name: "Beta", TerminalIDs: []string{"b"}},
				{ID: "singleton-c", Name: "Gamma", TerminalIDs: []string{"c"}},
			},
		},
		{
			name: "move terminal between groups and preserve destination order",
			initial: []domain.TerminalGroup{
				{ID: "left", Name: "Left", TerminalIDs: []string{"a", "b"}},
				{ID: "right", Name: "Right", TerminalIDs: []string{"c"}},
			},
			candidate: []domain.TerminalGroup{
				{ID: "left", Name: "Left", TerminalIDs: []string{"a"}},
				{ID: "right", Name: "Right", TerminalIDs: []string{"c", "b"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fileSystem := testutil.NewFakeFileSystem()
			target := filepath.Join(testCampaignsRoot, strings.ReplaceAll(test.name, " ", "-")+".json")
			initial := terminalGroupMutationTestSession(test.initial)
			initialData := mustEncodeSession(t, initial)
			fileSystem.SeedFile(target, initialData)
			service := NewService(
				NewStorage(fileSystem),
				&testutil.FakeDialog{OpenResult: target},
				testLocations,
			)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
			opened := service.Open(t.Context())
			require.True(t, opened.OK, "Open() = %#v", opened)
			require.NotNil(t, opened.Session)

			result := service.ReplaceTerminalGroups(t.Context(), test.candidate, 0)
			require.True(t, result.OK, "ReplaceTerminalGroups() = %#v", result)
			assert.True(t, result.Changed)
			assert.Empty(t, result.Error)
			assert.Equal(t, uint64(1), result.Revision)
			require.NotNil(t, result.Session)
			assert.Equal(t, test.candidate, result.Session.TerminalGroups)
			assert.Equal(t, initial.Terminals, result.Session.Terminals,
				"group replacement changed terminal content")

			active := service.Snapshot()
			require.NotNil(t, active.Session)
			assert.Equal(t, uint64(1), active.RequestedRevision)
			assert.Equal(t, uint64(1), active.SavedRevision)
			assert.Equal(t, SaveStateSaved, active.SaveState)
			assert.Equal(t, test.candidate, active.Session.TerminalGroups)
			persisted, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
			require.NoError(t, err)
			assert.Equal(t, test.candidate, persisted.TerminalGroups)
			assert.NotEqual(t, initialData, fileSystemFileData(t, fileSystem, target))

			result.Session.TerminalGroups[0].TerminalIDs[0] = "mutated-result"
			assert.Equal(t, test.candidate, service.Snapshot().Session.TerminalGroups,
				"mutation result must be detached from active state")
		})
	}
}

func TestReplaceTerminalGroupsNoOpDoesNotAdvanceRevisionOrWrite(t *testing.T) {
	t.Parallel()

	groups := []domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
	}
	initial := terminalGroupMutationTestSession(groups)
	target := filepath.Join(testCampaignsRoot, "group-no-op.json")
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(
		NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		testLocations,
	)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	writesBefore := len(fileSystem.WriteCalls())

	result := service.ReplaceTerminalGroups(t.Context(), groups, 0)
	require.True(t, result.OK, "ReplaceTerminalGroups(no-op) = %#v", result)
	assert.False(t, result.Changed)
	assert.Empty(t, result.Error)
	assert.Zero(t, result.Revision)
	require.NotNil(t, result.Session)
	assert.Equal(t, initial, *result.Session)
	assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
	active := service.Snapshot()
	assert.Zero(t, active.RequestedRevision)
	assert.Zero(t, active.SavedRevision)
	assert.Equal(t, SaveStateSaved, active.SaveState)
}

func TestReplaceTerminalGroupsRejectsStaleAndDuplicateSubmissionsWithoutWriting(t *testing.T) {
	t.Parallel()

	initial := terminalGroupMutationTestSession([]domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
	})
	coupled := []domain.TerminalGroup{
		{ID: "front", Name: "Front", TerminalIDs: []string{"a", "b"}},
		{ID: "back", Name: "Back", TerminalIDs: []string{"c"}},
	}
	target := filepath.Join(testCampaignsRoot, "group-duplicate-submit.json")
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, mustEncodeSession(t, initial))
	service := NewService(
		NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		testLocations,
	)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)

	first := service.ReplaceTerminalGroups(t.Context(), coupled, 0)
	require.True(t, first.OK, "first ReplaceTerminalGroups() = %#v", first)
	require.True(t, first.Changed)
	require.Equal(t, uint64(1), first.Revision)
	writesAfterFirst := len(fileSystem.WriteCalls())

	for _, test := range []struct {
		name      string
		candidate []domain.TerminalGroup
	}{
		{name: "duplicate submit", candidate: coupled},
		{name: "different stale candidate", candidate: initial.TerminalGroups},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := service.ReplaceTerminalGroups(t.Context(), test.candidate, 0)
			assert.False(t, result.OK)
			assert.False(t, result.Changed)
			assert.NotEmpty(t, result.Error)
			assert.Equal(t, uint64(1), result.Revision)
			require.NotNil(t, result.Session)
			assert.Equal(t, coupled, result.Session.TerminalGroups)
			assert.Equal(t, writesAfterFirst, len(fileSystem.WriteCalls()))
		})
	}

	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Equal(t, coupled, active.Session.TerminalGroups)
	assert.Equal(t, uint64(1), active.SavedRevision)
}

func TestReplaceTerminalGroupsPersistenceFailureKeepsCanonicalSessionAndRevision(t *testing.T) {
	t.Parallel()

	initial := terminalGroupMutationTestSession([]domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
	})
	candidate := []domain.TerminalGroup{
		{ID: "left", Name: "Left", TerminalIDs: []string{"a"}},
		{ID: "right", Name: "Right", TerminalIDs: []string{"b", "c"}},
	}
	target := filepath.Join(testCampaignsRoot, "group-write-failure.json")
	initialData := mustEncodeSession(t, initial)
	store := &failingMutationStore{
		path: target,
		data: initialData,
		err:  fmt.Errorf("injected group replacement failure"),
	}
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)

	result := service.ReplaceTerminalGroups(t.Context(), candidate, 0)
	assert.False(t, result.OK)
	assert.False(t, result.Changed)
	assert.NotEmpty(t, result.Error)
	assert.Zero(t, result.Revision)
	require.NotNil(t, result.Session)
	assert.Equal(t, initial, *result.Session)
	assert.Equal(t, 1, store.writes)
	assert.Equal(t, initialData, store.data)
	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Equal(t, initial, *active.Session)
	assert.Zero(t, active.RequestedRevision)
	assert.Zero(t, active.SavedRevision)
	assert.Equal(t, SaveStateSaved, active.SaveState)

	result.Session.TerminalGroups[0].TerminalIDs[0] = "mutated-failure-result"
	assert.Equal(t, initial.TerminalGroups, service.Snapshot().Session.TerminalGroups,
		"failure result must be detached from canonical state")
}

func TestInvalidOpenRetainsPreviousActiveSessionAndPath(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	validPath := filepath.Join(testCampaignsRoot, "valid.json")
	invalidPath := filepath.Join(testCampaignsRoot, "invalid.json")
	validData, err := domain.EncodeSession(validSession("safe"))
	require.NoError(t, err)
	fileSystem.SeedFile(validPath, validData)
	fileSystem.SeedFile(invalidPath, []byte(`{"version":2,"name":"bad","terminals":[]}`))
	dialog := &testutil.FakeDialog{OpenResult: validPath}
	service := NewService(NewStorage(fileSystem), dialog, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	opened := service.Open(t.Context())
	require.True(t, opened.OK, "first Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	assert.Equal(t, validPath, opened.FilePath)
	dialog.OpenResult = invalidPath
	failed := service.Open(t.Context())
	assert.False(t, failed.OK)
	assert.False(t, failed.Canceled)
	assert.NotEmpty(t, failed.Error)
	assert.Nil(t, failed.Session)
	assert.NotContains(t, failed.Error, string(fileSystemFileData(t, fileSystem, invalidPath)))
	snapshot := service.Snapshot()
	assert.Equal(t, validPath, snapshot.Path)
	require.NotNil(t, snapshot.Session)
	assert.Equal(t, "safe", snapshot.Session.Name)
}

func TestOpenAndSavePreserveUnknownFieldsAtExplicitPath(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "forward-compatible.json")
	raw := []byte(`{
  "version": 1,
  "name": "before",
  "campaignNote": {"keep": true},
  "terminals": [{
    "id": "t1",
    "name": "Terminal",
    "hackLevel": 0,
    "introText": "",
    "terminalNote": 42,
    "root": {"id":"root","type":"folder","name":"ROOT","children":[],"nodeNote":[1,2]}
  }]
}`)
	fileSystem.SeedFile(target, raw)
	dialog := &testutil.FakeDialog{OpenResult: target}
	service := NewService(NewStorage(fileSystem), dialog, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	edited := *opened.Session
	edited.Name = "after"
	saved := service.Save(t.Context(), edited, 1)
	require.True(t, saved.OK, "Save() = %#v", saved)
	assert.Empty(t, saved.Error)
	assert.Equal(t, uint64(1), saved.RequestedRevision)
	assert.Equal(t, uint64(1), saved.SavedRevision)
	written := fileSystemFileData(t, fileSystem, target)
	for _, field := range []string{"campaignNote", "terminalNote", "nodeNote"} {
		assert.Contains(t, string(written), `"`+field+`"`)
	}
	decoded, err := domain.DecodeSession(written)
	require.NoError(t, err)
	assert.Equal(t, "after", decoded.Name)
	for _, rename := range fileSystem.RenameCalls() {
		assert.Equal(t, target, rename.NewPath)
	}
	assertNoApplicationSupportWrites(t, fileSystem)
}

func TestRealDemoCrossTerminalLinkSurvivesServiceSaveAndRejectsOnlyInvalidTargets(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../sessions/demo.json")
	require.NoError(t, err)
	target := filepath.Join(testCampaignsRoot, "demo-transition.json")
	fileSystem := testutil.NewFakeFileSystem()
	fileSystem.SeedFile(target, raw)
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { require.NoError(t, service.Shutdown(context.WithoutCancel(t.Context()))) })

	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	require.Len(t, opened.Session.Terminals, 2)
	require.Equal(t, "t_demo2", opened.Session.Terminals[0].Root.Children[4].TerminalTransition.TargetTerminalID)

	saved := service.Save(t.Context(), *opened.Session, 1)
	require.True(t, saved.OK, "Save() = %#v", saved)
	reopened := service.Open(t.Context())
	require.True(t, reopened.OK, "reopen = %#v", reopened)
	require.Len(t, reopened.Session.Terminals, 2)
	require.Equal(t, "t_demo2", reopened.Session.Terminals[0].Root.Children[4].TerminalTransition.TargetTerminalID)

	missing := cloneSession(*reopened.Session)
	missing.Terminals = missing.Terminals[:1]
	require.False(t, service.Save(t.Context(), missing, 2).OK)

	self := cloneSession(*reopened.Session)
	self.Terminals[0].Root.Children[4].TerminalTransition.TargetTerminalID = "t_demo1"
	require.False(t, service.Save(t.Context(), self, 2).OK)
}

func TestOpenAndSaveLegacyVersionOnePreservesOrdinaryContentWithoutAddingStateFields(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "legacy-ordinary.json")
	raw := []byte(`{
  "version": 1,
  "name": "Legacy ordinary",
  "futureSession": {"keep": true},
  "terminals": [{
    "id": "t1",
    "name": "Terminal",
    "hackLevel": 0,
    "introText": "",
    "futureTerminal": 17,
    "root": {
      "id": "root",
      "type": "folder",
      "name": "ROOT",
      "children": [{
        "id": "status",
        "type": "command",
        "name": "Read status",
        "text": "All systems nominal.",
        "futureCommand": "keep"
      }]
    }
  }]
}`)
	fileSystem.SeedFile(target, raw)
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	ordinary := opened.Session.Terminals[0].Root.Children[0]
	assert.Nil(t, ordinary.StateChange)
	assert.Empty(t, opened.Session.Terminals[0].CommandStates)

	saved := service.Save(t.Context(), *opened.Session, 1)
	require.True(t, saved.OK, "Save() = %#v", saved)
	assert.Equal(t, uint64(1), saved.RequestedRevision)
	assert.Equal(t, uint64(1), saved.SavedRevision)
	written := fileSystemFileData(t, fileSystem, target)
	for _, absent := range []string{`"stateChange"`, `"commandStates"`} {
		assert.NotContains(t, string(written), absent)
	}
	for _, extra := range []string{`"futureSession"`, `"futureTerminal"`, `"futureCommand"`} {
		assert.Contains(t, string(written), extra)
	}

	roundTrip, err := domain.DecodeSession(written)
	require.NoError(t, err)
	assert.Equal(t, 1, roundTrip.Version)
	assert.Equal(t, ordinary, roundTrip.Terminals[0].Root.Children[0])
}

func TestAssociatePlayerConfigPersistsRelativeReferenceAndKeepsActiveSession(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeFileSystem()
	sessionPath := testAbsolutePath("Campaigns", "Chapter One", "session.json")
	data, err := domain.EncodeSession(validSession("chapter one"))
	require.NoError(t, err)
	fs.SeedFile(sessionPath, data)
	service := NewService(NewStorage(fs), &testutil.FakeDialog{OpenResult: sessionPath}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)

	configPath := testAbsolutePath("Campaigns", "Players", "shared.json")
	result := service.AssociatePlayerConfig(t.Context(), configPath)
	require.True(t, result.OK, "AssociatePlayerConfig() = %#v", result)
	require.NotNil(t, result.Session)
	want := filepath.Join("..", "Players", "shared.json")
	assert.Equal(t, want, result.Session.PlayerConfig)

	written, err := domain.DecodeSession(fileSystemFileData(t, fs, sessionPath))
	require.NoError(t, err)
	assert.Equal(t, want, written.PlayerConfig)
	require.NotNil(t, service.Snapshot().Session)
	assert.Equal(t, want, service.Snapshot().Session.PlayerConfig)
}

func TestSaveWithoutActivePathFailsWithoutWriting(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	result := service.Save(t.Context(), validSession("orphan"), 1)
	assert.False(t, result.OK)
	assert.NotEmpty(t, result.Error)
	assert.Equal(t, uint64(1), result.RequestedRevision)
	assert.Zero(t, result.SavedRevision)
	assert.Empty(t, fileSystem.WriteCalls())
	assert.Empty(t, fileSystem.RenameCalls())
	assertInactive(t, service.Snapshot())
}

func TestCopyDemoRequiresExplicitDestinationAndActivatesWritableCopy(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	demo := stateChangingSession("demo")
	demo.PlayerConfig = "demo-players.json"
	completedState := domain.CommandExecutionState{
		CompletedName: "Двери открыты",
		ResultText:    "Доступ в сектор разрешён.",
	}
	demo.Terminals[0].CommandStates = map[string]domain.CommandExecutionState{"doors": completedState}
	demoData, err := domain.EncodeSession(demo)
	require.NoError(t, err)
	playerConfigData, err := domain.EncodePlayerConfig(domain.PlayerConfig{
		Version: 1,
		Name:    "demo-players",
		Roster: []domain.CharacterRosterEntry{
			{ID: "scout", Name: "Разведчик", Intelligence: 6, HackerPerkAvailable: false},
			{ID: "medic", Name: "Медик", Intelligence: 9, HackerPerkAvailable: true},
		},
	})
	require.NoError(t, err)
	fileSystem.SeedFile(testLocations.BundledDemo, demoData)
	bundledPlayerConfig := filepath.Join(filepath.Dir(testLocations.BundledDemo), demo.PlayerConfig)
	fileSystem.SeedFile(bundledPlayerConfig, playerConfigData)
	destination := filepath.Join(testCampaignsRoot, "demo-copy.json")
	dialog := &testutil.FakeDialog{SaveResult: destination}
	service := NewService(NewStorage(fileSystem), dialog, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })

	result := service.CopyDemo(t.Context())
	require.True(t, result.OK, "CopyDemo() = %#v", result)
	assert.False(t, result.Canceled)
	assert.Empty(t, result.Error)
	assert.Equal(t, destination, result.FilePath)
	require.NotNil(t, result.Session)
	assert.Equal(t, completedState, result.Session.Terminals[0].CommandStates["doors"])
	assert.Equal(t, demoData, fileSystemFileData(t, fileSystem, testLocations.BundledDemo))
	assert.Equal(t, playerConfigData, fileSystemFileData(t, fileSystem, bundledPlayerConfig))
	assert.Equal(t, demoData, fileSystemFileData(t, fileSystem, destination))
	destinationPlayerConfig := filepath.Join(filepath.Dir(destination), demo.PlayerConfig)
	assert.Equal(t, playerConfigData, fileSystemFileData(t, fileSystem, destinationPlayerConfig))
	snapshot := service.Snapshot()
	assert.Equal(t, destination, snapshot.Path)
	assert.NotNil(t, snapshot.Session)

	reset := service.ResetCommandState(t.Context(), "t1", "doors")
	require.True(t, reset.OK, "ResetCommandState() in writable demo copy = %#v", reset)
	assert.True(t, reset.Changed)
	assert.Equal(t, uint64(1), reset.Revision)
	require.NotNil(t, reset.Session)
	assert.NotContains(t, reset.Session.Terminals[0].CommandStates, "doors")
	assert.Equal(t, demoData, fileSystemFileData(t, fileSystem, testLocations.BundledDemo),
		"resetting the writable copy mutated the bundled demo")
	writtenCopy, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, destination))
	require.NoError(t, err)
	assert.NotContains(t, writtenCopy.Terminals[0].CommandStates, "doors")
}

func TestTwentyQueuedRevisionsFinishAtNewestAcceptedSession(t *testing.T) {
	store := newBlockingStore()
	target := filepath.Join(testCampaignsRoot, "ordered.json")
	store.seed(target, mustEncodeSession(t, validSession("initial")))
	dialog := &testutil.FakeDialog{OpenResult: target}
	service := NewService(store, dialog, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	result := service.Open(t.Context())
	require.True(t, result.OK, "Open() = %#v", result)

	type completion struct {
		revision uint64
		result   SaveResult
	}
	completions := make(chan completion, 20)
	startSave := func(revision uint64) {
		session := validSession(fmt.Sprintf("revision-%02d", revision))
		go func() {
			completions <- completion{revision: revision, result: service.Save(t.Context(), session, revision)}
		}()
	}

	startSave(1)
	select {
	case <-store.firstWriteStarted:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "first revision did not begin writing")
	}
	t.Cleanup(store.release)
	for revision := uint64(2); revision <= 20; revision++ {
		startSave(revision)
		waitForRequestedRevision(t, service, revision)
	}
	store.release()

	results := make(map[uint64]SaveResult, 20)
	for range 20 {
		select {
		case completion := <-completions:
			results[completion.revision] = completion.result
		case <-time.After(2 * time.Second):
			require.FailNowf(t, "saves did not complete", "only %d of 20 saves completed", len(results))
		}
	}
	for revision := uint64(1); revision <= 20; revision++ {
		result := results[revision]
		assert.True(t, result.OK, "revision %d result = %#v", revision, result)
		assert.Empty(t, result.Error, "revision %d", revision)
		assert.Equal(t, revision, result.RequestedRevision)
		assert.GreaterOrEqual(t, result.SavedRevision, revision)
		assert.LessOrEqual(t, result.SavedRevision, uint64(20))
	}
	assert.Equal(t, uint64(20), results[20].SavedRevision)
	final, err := domain.DecodeSession(store.file(target))
	require.NoError(t, err)
	assert.Equal(t, "revision-20", final.Name)
	snapshot := service.Snapshot()
	assert.Equal(t, target, snapshot.Path)
	assert.Equal(t, uint64(20), snapshot.RequestedRevision)
	assert.Equal(t, uint64(20), snapshot.SavedRevision)
	assert.Equal(t, SaveStateSaved, snapshot.SaveState)
}

func TestCommandStateMutationsAllocateMonotonicDocumentRevisions(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "state-mutations.json")
	fileSystem.SeedFile(target, mustEncodeSession(t, stateChangingSession("chapter")))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)

	doors := service.ExecuteCommandState(t.Context(), "t1", "doors")
	require.True(t, doors.OK, "ExecuteCommandState(doors) = %#v", doors)
	assert.True(t, doors.Changed)
	assert.Empty(t, doors.Error)
	assert.Equal(t, uint64(1), doors.Revision)
	require.NotNil(t, doors.Session)
	assert.Equal(t, domain.CommandExecutionState{CompletedName: "Двери открыты", ResultText: "Доступ в сектор разрешён."}, doors.Session.Terminals[0].CommandStates["doors"])

	alarm := service.ExecuteCommandState(t.Context(), "t1", "alarm")
	require.True(t, alarm.OK, "ExecuteCommandState(alarm) = %#v", alarm)
	assert.True(t, alarm.Changed)
	assert.Equal(t, uint64(2), alarm.Revision)
	one := service.ResetCommandState(t.Context(), "t1", "doors")
	require.True(t, one.OK, "ResetCommandState() = %#v", one)
	assert.True(t, one.Changed)
	assert.Equal(t, uint64(3), one.Revision)
	require.NotNil(t, one.Session)
	assert.NotContains(t, one.Session.Terminals[0].CommandStates, "doors")
	assert.Contains(t, one.Session.Terminals[0].CommandStates, "alarm")

	all := service.ResetTerminalCommandStates(t.Context(), "t1")
	require.True(t, all.OK, "ResetTerminalCommandStates() = %#v", all)
	assert.True(t, all.Changed)
	assert.Equal(t, uint64(4), all.Revision)
	require.NotNil(t, all.Session)
	assert.Empty(t, all.Session.Terminals[0].CommandStates)
	writesBeforeNoOp := len(fileSystem.WriteCalls())
	noOp := service.ResetTerminalCommandStates(t.Context(), "t1")
	require.True(t, noOp.OK, "idempotent ResetTerminalCommandStates() = %#v", noOp)
	assert.False(t, noOp.Changed)
	assert.Equal(t, uint64(4), noOp.Revision)
	require.NotNil(t, noOp.Session)
	assert.Equal(t, writesBeforeNoOp, len(fileSystem.WriteCalls()))
	snapshot := service.Snapshot()
	assert.Equal(t, uint64(4), snapshot.RequestedRevision)
	assert.Equal(t, uint64(4), snapshot.SavedRevision)
	assert.Equal(t, SaveStateSaved, snapshot.SaveState)
}

func TestStaleFullSavePreservesCanonicalFrozenStateAndAppliesAuthoredEdits(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "stale-save.json")
	fileSystem.SeedFile(target, mustEncodeSession(t, stateChangingSession("before")))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)
	stale := *opened.Session

	executed := service.ExecuteCommandState(t.Context(), "t1", "doors")
	require.True(t, executed.OK, "ExecuteCommandState() = %#v", executed)
	assert.True(t, executed.Changed)
	assert.Equal(t, uint64(1), executed.Revision)
	stale.Name = "after"
	stale.Terminals[0].Root.Children[0].Name = "Открыть гермодвери"
	stale.Terminals[0].Root.Children[0].Text = "Новый результат для следующего выполнения."
	stale.Terminals[0].Root.Children[0].StateChange.CompletedName = "Гермодвери открыты"
	stale.Terminals[0].CommandStates = map[string]domain.CommandExecutionState{
		"doors": {CompletedName: "ПОДДЕЛКА", ResultText: "ПОДДЕЛКА"},
	}

	saved := service.Save(t.Context(), stale, 2)
	require.True(t, saved.OK, "Save(stale) = %#v", saved)
	assert.Equal(t, uint64(2), saved.RequestedRevision)
	assert.Equal(t, uint64(2), saved.SavedRevision)
	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Equal(t, "after", active.Session.Name)
	command := active.Session.Terminals[0].Root.Children[0]
	assert.Equal(t, "Открыть гермодвери", command.Name)
	require.NotNil(t, command.StateChange)
	assert.Equal(t, "Гермодвери открыты", command.StateChange.CompletedName)
	wantFrozen := domain.CommandExecutionState{CompletedName: "Двери открыты", ResultText: "Доступ в сектор разрешён."}
	assert.Equal(t, wantFrozen, active.Session.Terminals[0].CommandStates["doors"])
	reopened, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Equal(t, wantFrozen, reopened.Terminals[0].CommandStates["doors"])
}

func TestFullSavePrunesFrozenStateWhenCommandIsDeleted(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "delete-command.json")
	fileSystem.SeedFile(target, mustEncodeSession(t, stateChangingSession("before")))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	result := service.ExecuteCommandState(t.Context(), "t1", "doors")
	require.True(t, result.OK, "ExecuteCommandState() = %#v", result)
	assert.Equal(t, uint64(1), result.Revision)

	candidate := *service.Snapshot().Session
	candidate.Terminals[0].Root.Children = append([]domain.ContentNode(nil), candidate.Terminals[0].Root.Children[1:]...)
	saveResult := service.Save(t.Context(), candidate, 2)
	require.True(t, saveResult.OK, "Save(with deletion) = %#v", saveResult)
	assert.Equal(t, uint64(2), saveResult.SavedRevision)
	active := service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Empty(t, active.Session.Terminals[0].CommandStates)
	written, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Empty(t, written.Terminals[0].CommandStates)
}

func TestTerminalCatalogReturnsDetachedCurrentSessionSnapshotsAndInvalidSaveIsAtomic(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "linked.json")
	linked := linkedSessionForTest()
	linked.TerminalGroups = []domain.TerminalGroup{
		{ID: "linked", Name: "Linked", TerminalIDs: []string{"a", "b"}},
	}
	fileSystem.SeedFile(target, mustEncodeSession(t, linked))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)

	lookup, ok := service.LookupTerminalTransition("a", "go")
	require.True(t, ok)
	assert.Equal(t, "GO", lookup.CommandName)
	assert.Equal(t, "b", lookup.Target.TerminalID)
	lookup.Target.Tree.Name = "MUTATED"
	again, ok := service.LookupTerminalTransition("a", "go")
	require.True(t, ok)
	assert.Equal(t, "ROOT", again.Target.Tree.Name, "catalog snapshots must be detached")

	before := service.Snapshot()
	writesBefore := len(fileSystem.WriteCalls())
	invalid := domain.CloneSession(*before.Session)
	invalid.Terminals = invalid.Terminals[:1]
	result := service.Save(t.Context(), invalid, before.RequestedRevision+1)
	assert.False(t, result.OK)
	assert.Equal(t, writesBefore, len(fileSystem.WriteCalls()))
	after := service.Snapshot()
	require.Equal(t, before.Session, after.Session)
}

func TestLookupTerminalTransitionRequiresCurrentSameGroupEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		groups    []domain.TerminalGroup
		sourceID  string
		commandID string
		mutate    func(*domain.Session)
		wantOK    bool
	}{
		{
			name: "same group",
			groups: []domain.TerminalGroup{
				{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
			},
			sourceID: "a", commandID: "go", wantOK: true,
		},
		{
			name: "cross group",
			groups: []domain.TerminalGroup{
				{ID: "source", Name: "Source", TerminalIDs: []string{"a", "c"}},
				{ID: "target", Name: "Target", TerminalIDs: []string{"b"}},
			},
			sourceID: "a", commandID: "go",
		},
		{
			name: "self target",
			groups: []domain.TerminalGroup{
				{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
			},
			sourceID: "a", commandID: "go",
			mutate: func(session *domain.Session) {
				session.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID = "a"
			},
		},
		{
			name: "missing target",
			groups: []domain.TerminalGroup{
				{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
			},
			sourceID: "a", commandID: "go",
			mutate: func(session *domain.Session) {
				session.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID = "missing"
			},
		},
		{
			name: "missing source",
			groups: []domain.TerminalGroup{
				{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
			},
			sourceID: "missing", commandID: "go",
		},
		{
			name: "missing command",
			groups: []domain.TerminalGroup{
				{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
			},
			sourceID: "a", commandID: "missing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			target := filepath.Join(testCampaignsRoot, strings.ReplaceAll(test.name, " ", "-")+"-catalog.json")
			fileSystem := testutil.NewFakeFileSystem()
			fileSystem.SeedFile(target, mustEncodeSession(t, terminalTransitionCatalogSession(test.groups)))
			service := NewService(
				NewStorage(fileSystem),
				&testutil.FakeDialog{OpenResult: target},
				testLocations,
			)
			t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
			require.True(t, service.Open(t.Context()).OK)
			if test.mutate != nil {
				service.mu.Lock()
				test.mutate(service.active.Session)
				service.mu.Unlock()
			}

			transition, ok := service.LookupTerminalTransition(test.sourceID, test.commandID)
			assert.Equal(t, test.wantOK, ok)
			if !test.wantOK {
				assert.Equal(t, domain.TerminalTransitionTarget{}, transition)
				return
			}
			assert.Equal(t, "a", transition.SourceTerminalID)
			assert.Equal(t, "Alpha", transition.SourceTerminalName)
			assert.Equal(t, "go", transition.CommandID)
			assert.Equal(t, "GO", transition.CommandName)
			assert.Equal(t, "b", transition.Target.TerminalID)
			assert.Equal(t, "Beta", transition.Target.TerminalName)
		})
	}
}

func TestLookupTerminalTransitionRejectsStaleRemovedLink(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "stale-terminal-link.json")
	fileSystem := testutil.NewFakeFileSystem()
	groups := []domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
	}
	fileSystem.SeedFile(target, mustEncodeSession(t, terminalTransitionCatalogSession(groups)))
	service := NewService(
		NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		testLocations,
	)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)
	_, ok := service.LookupTerminalTransition("a", "go")
	require.True(t, ok)

	candidate := domain.CloneSession(*service.Snapshot().Session)
	candidate.Terminals[0].Root.Children = []domain.ContentNode{}
	saved := service.Save(t.Context(), candidate, 1)
	require.True(t, saved.OK, "Save(remove transition) = %#v", saved)
	transition, ok := service.LookupTerminalTransition("a", "go")
	assert.False(t, ok)
	assert.Equal(t, domain.TerminalTransitionTarget{}, transition)
	terminal, terminalOK := service.LookupTerminal("b")
	assert.True(t, terminalOK)
	assert.Equal(t, "b", terminal.TerminalID)
}

func TestLookupTerminalTransitionReturnsDeeplyDetachedTarget(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "detached-terminal-transition.json")
	fileSystem := testutil.NewFakeFileSystem()
	groups := []domain.TerminalGroup{
		{ID: "route", Name: "Route", TerminalIDs: []string{"a", "b", "c"}},
	}
	fileSystem.SeedFile(target, mustEncodeSession(t, terminalTransitionCatalogSession(groups)))
	service := NewService(
		NewStorage(fileSystem),
		&testutil.FakeDialog{OpenResult: target},
		testLocations,
	)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	require.True(t, service.Open(t.Context()).OK)

	transition, ok := service.LookupTerminalTransition("a", "go")
	require.True(t, ok)
	transition.Target.Tree.Name = "MUTATED ROOT"
	transition.Target.Tree.Children[0].Name = "MUTATED COMMAND"
	transition.Target.Tree.Children[0].StateChange.CompletedName = "MUTATED COMPLETION"
	transition.Target.CommandStates["status"] = domain.CommandExecutionState{
		CompletedName: "MUTATED STATE",
		ResultText:    "MUTATED RESULT",
	}

	again, ok := service.LookupTerminalTransition("a", "go")
	require.True(t, ok)
	assert.Equal(t, "ROOT", again.Target.Tree.Name)
	require.Len(t, again.Target.Tree.Children, 1)
	assert.Equal(t, "Status", again.Target.Tree.Children[0].Name)
	require.NotNil(t, again.Target.Tree.Children[0].StateChange)
	assert.Equal(t, "Status read", again.Target.Tree.Children[0].StateChange.CompletedName)
	assert.Equal(t, domain.CommandExecutionState{
		CompletedName: "Status read",
		ResultText:    "All systems nominal.",
	}, again.Target.CommandStates["status"])
}

func TestStableIDAndFrozenStateRulesAcross100CompletedCommands(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	target := filepath.Join(testCampaignsRoot, "one-hundred-completed-commands.json")
	fileSystem.SeedFile(target, mustEncodeSession(t, stateChangingSessionWith100CompletedCommands()))
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = service.Shutdown(context.WithoutCancel(t.Context())) })
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)
	require.NotNil(t, opened.Session)

	candidate := *opened.Session
	moved := domain.ContentNode{ID: "moved-folder", Type: domain.NodeFolder, Name: "MOVED COMMANDS"}
	for index := 99; index >= 0; index-- {
		command := candidate.Terminals[0].Root.Children[index]
		command.Name = fmt.Sprintf("Renamed command %03d", index)
		command.Text = fmt.Sprintf("Next result %03d", index)
		command.StateChange.CompletedName = fmt.Sprintf("Next completed %03d", index)
		command.StateChange.ConfirmationText = fmt.Sprintf("Run renamed command %03d?", index)
		moved.Children = append(moved.Children, command)
	}
	candidate.Terminals[0].Root.Children = []domain.ContentNode{moved}

	saved := service.Save(t.Context(), candidate, 1)
	require.True(t, saved.OK, "Save(rename/move/delete 100 commands) = %#v", saved)
	assert.Equal(t, uint64(1), saved.SavedRevision)
	active := service.Snapshot()
	require.NotNil(t, active.Session)
	require.Len(t, active.Session.Terminals[0].CommandStates, 100)
	for index := range 100 {
		commandID := fmt.Sprintf("command-%03d", index)
		wantFrozen := domain.CommandExecutionState{
			CompletedName: fmt.Sprintf("Frozen completed %03d", index),
			ResultText:    fmt.Sprintf("Frozen result %03d", index),
		}
		assert.Equal(t, wantFrozen, active.Session.Terminals[0].CommandStates[commandID], "command %q", commandID)
	}

	for index := range 100 {
		commandID := fmt.Sprintf("command-%03d", index)
		reset := service.ResetCommandState(t.Context(), "t100", commandID)
		require.True(t, reset.OK, "ResetCommandState(%q) = %#v", commandID, reset)
		assert.True(t, reset.Changed, "command %q", commandID)
		executed := service.ExecuteCommandState(t.Context(), "t100", commandID)
		require.True(t, executed.OK, "ExecuteCommandState(%q) after reset = %#v", commandID, executed)
		assert.True(t, executed.Changed, "command %q", commandID)
		require.NotNil(t, executed.Session)
		wantNext := domain.CommandExecutionState{
			CompletedName: fmt.Sprintf("Next completed %03d", index),
			ResultText:    fmt.Sprintf("Next result %03d", index),
		}
		assert.Equal(t, wantNext, executed.Session.Terminals[0].CommandStates[commandID], "command %q", commandID)
	}
	active = service.Snapshot()
	require.NotNil(t, active.Session)
	require.Len(t, active.Session.Terminals[0].CommandStates, 100)

	replacements := domain.ContentNode{ID: "replacement-folder", Type: domain.NodeFolder, Name: "REPLACEMENTS"}
	for index := range 100 {
		replacements.Children = append(replacements.Children, domain.ContentNode{
			ID:   fmt.Sprintf("replacement-%03d", index),
			Type: domain.NodeCommand,
			Name: fmt.Sprintf("Replacement %03d", index),
			Text: fmt.Sprintf("Replacement result %03d", index),
			StateChange: &domain.StateChangeConfig{
				CompletedName:    fmt.Sprintf("Replacement completed %03d", index),
				ConfirmationText: fmt.Sprintf("Replace command %03d?", index),
			},
		})
	}
	deletedCandidate := *active.Session
	deletedCandidate.Terminals[0].Root.Children = []domain.ContentNode{replacements}
	deleteRevision := active.RequestedRevision + 1
	deleted := service.Save(t.Context(), deletedCandidate, deleteRevision)
	require.True(t, deleted.OK, "Save(delete all 100 commands and add replacements) = %#v", deleted)
	assert.Equal(t, deleteRevision, deleted.SavedRevision)
	active = service.Snapshot()
	require.NotNil(t, active.Session)
	assert.Empty(t, active.Session.Terminals[0].CommandStates)
	for index := range 100 {
		commandID := fmt.Sprintf("command-%03d", index)
		replacementID := fmt.Sprintf("replacement-%03d", index)
		assert.NotContains(t, active.Session.Terminals[0].CommandStates, commandID)
		assert.NotContains(t, active.Session.Terminals[0].CommandStates, replacementID)
	}

	reopened, err := domain.DecodeSession(fileSystemFileData(t, fileSystem, target))
	require.NoError(t, err)
	assert.Empty(t, reopened.Terminals[0].CommandStates)
}

func TestFailedCommandStateMutationKeepsPriorDocumentAndRevision(t *testing.T) {
	t.Parallel()

	target := filepath.Join(testCampaignsRoot, "failed-command-state.json")
	initial := mustEncodeSession(t, stateChangingSession("safe"))
	store := &failingMutationStore{path: target, data: initial, err: fmt.Errorf("injected atomic replacement failure")}
	service := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() = %#v", opened)

	for attempt := range 100 {
		result := service.ExecuteCommandState(t.Context(), "t1", "doors")
		assert.False(t, result.OK, "attempt %d", attempt)
		assert.False(t, result.Changed, "attempt %d", attempt)
		assert.NotEmpty(t, result.Error, "attempt %d", attempt)
		assert.Zero(t, result.Revision, "attempt %d", attempt)
		assert.Nil(t, result.Session, "attempt %d", attempt)
		assert.Equal(t, attempt+1, store.writes, "attempt %d", attempt)
		assert.Equal(t, initial, store.data, "attempt %d", attempt)
		active := service.Snapshot()
		assert.Zero(t, active.RequestedRevision, "attempt %d", attempt)
		assert.Zero(t, active.SavedRevision, "attempt %d", attempt)
		require.NotNil(t, active.Session)
		assert.Empty(t, active.Session.Terminals[0].CommandStates, "attempt %d", attempt)
	}
	require.NoError(t, service.Shutdown(t.Context()))

	restarted := NewService(store, &testutil.FakeDialog{OpenResult: target}, testLocations)
	t.Cleanup(func() { _ = restarted.Shutdown(context.WithoutCancel(t.Context())) })
	reopened := restarted.Open(t.Context())
	require.True(t, reopened.OK, "reopen after 100 failed mutations = %#v", reopened)
	require.NotNil(t, reopened.Session)
	assert.Empty(t, reopened.Session.Terminals[0].CommandStates)
	assert.Equal(t, initial, store.data)
}

func TestCompletedCommandStateSurvivesFreshProcessReopen(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "process-restart-session.json")
	initial := mustEncodeSession(t, stateChangingSession("process restart"))
	require.NoError(t, os.WriteFile(target, initial, 0o600))
	executable, err := os.Executable()
	require.NoError(t, err)

	for _, mode := range []string{"execute", "reopen"} {
		command := exec.CommandContext(t.Context(), executable, "-test.run=^TestCommandStateFreshProcessHelper$", "-test.v")
		command.Env = append(os.Environ(),
			"FALLOUT_COMMAND_STATE_PROCESS_MODE="+mode,
			"FALLOUT_COMMAND_STATE_PROCESS_PATH="+target,
		)
		output, runErr := command.CombinedOutput()
		require.NoError(t, runErr, "fresh process %q failed:\n%s", mode, output)
	}

	durable, err := domain.DecodeSession(mustReadFile(t, target))
	require.NoError(t, err)
	want := domain.CommandExecutionState{CompletedName: "Двери открыты", ResultText: "Доступ в сектор разрешён."}
	assert.Equal(t, want, durable.Terminals[0].CommandStates["doors"])
}

func TestCommandStateFreshProcessHelper(t *testing.T) {
	mode := os.Getenv("FALLOUT_COMMAND_STATE_PROCESS_MODE")
	if mode == "" {
		t.Skip("helper runs only in a fresh subprocess")
	}
	target := os.Getenv("FALLOUT_COMMAND_STATE_PROCESS_PATH")
	require.NotEmpty(t, target, "FALLOUT_COMMAND_STATE_PROCESS_PATH is empty")

	service := NewService(NewStorage(nil), &testutil.FakeDialog{OpenResult: target}, testLocations)
	opened := service.Open(t.Context())
	require.True(t, opened.OK, "Open() in %s process = %#v", mode, opened)
	require.NotNil(t, opened.Session)
	switch mode {
	case "execute":
		result := service.ExecuteCommandState(t.Context(), "t1", "doors")
		require.True(t, result.OK, "ExecuteCommandState() in fresh process = %#v", result)
		assert.True(t, result.Changed)
		require.NotNil(t, result.Session)
	case "reopen":
		want := domain.CommandExecutionState{CompletedName: "Двери открыты", ResultText: "Доступ в сектор разрешён."}
		assert.Equal(t, want, opened.Session.Terminals[0].CommandStates["doors"])
	default:
		require.Failf(t, "unknown helper mode", "%q", mode)
	}
	require.NoError(t, service.Shutdown(t.Context()), "mode %s", mode)
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "ReadFile(%q)", path)
	return data
}

func stateChangingSession(name string) domain.Session {
	return domain.Session{
		Version: 1,
		Name:    name,
		Terminals: []domain.Terminal{
			{
				ID: "t1", Name: "Terminal 1", Root: domain.ContentNode{
					ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
						{
							ID: "doors", Type: domain.NodeCommand, Name: "Открыть двери", Text: "Доступ в сектор разрешён.",
							StateChange: &domain.StateChangeConfig{CompletedName: "Двери открыты", ConfirmationText: "Открыть двери?"},
						},
						{
							ID: "alarm", Type: domain.NodeCommand, Name: "Отключить тревогу", Text: "Тревога отключена.",
							StateChange: &domain.StateChangeConfig{CompletedName: "Тревога отключена", ConfirmationText: "Отключить тревогу?"},
						},
					},
				},
			},
			{
				ID: "t2", Name: "Terminal 2", Root: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{}},
			},
		},
	}
}

func stateChangingSessionWith100CompletedCommands() domain.Session {
	terminal := domain.Terminal{
		ID: "t100", Name: "One hundred commands", HackLevel: 1,
		Root:          domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT"},
		CommandStates: make(map[string]domain.CommandExecutionState, 100),
	}
	for index := range 100 {
		commandID := fmt.Sprintf("command-%03d", index)
		terminal.Root.Children = append(terminal.Root.Children, domain.ContentNode{
			ID:   commandID,
			Type: domain.NodeCommand,
			Name: fmt.Sprintf("Initial command %03d", index),
			Text: fmt.Sprintf("Authored result %03d", index),
			StateChange: &domain.StateChangeConfig{
				CompletedName:    fmt.Sprintf("Authored completed %03d", index),
				ConfirmationText: fmt.Sprintf("Run command %03d?", index),
			},
		})
		terminal.CommandStates[commandID] = domain.CommandExecutionState{
			CompletedName: fmt.Sprintf("Frozen completed %03d", index),
			ResultText:    fmt.Sprintf("Frozen result %03d", index),
		}
	}
	return domain.Session{
		Version:   1,
		Name:      "Stable ID sample of 100 completed commands",
		Terminals: []domain.Terminal{terminal},
	}
}

func validSession(name string) domain.Session {
	return domain.Session{
		Version: 1,
		Name:    name,
		Terminals: []domain.Terminal{{
			ID:        "t1",
			Name:      "Terminal 1",
			HackLevel: 0,
			IntroText: "",
			Root: domain.ContentNode{
				ID:       "root",
				Type:     domain.NodeFolder,
				Name:     "ROOT",
				Children: []domain.ContentNode{},
			},
		}},
	}
}

func terminalForGroupTest(id, name string) domain.Terminal {
	return domain.Terminal{
		ID:        id,
		Name:      name,
		HackLevel: 0,
		IntroText: "",
		Root: domain.ContentNode{
			ID:       "root",
			Type:     domain.NodeFolder,
			Name:     "ROOT",
			Children: []domain.ContentNode{},
		},
	}
}

func terminalGroupMutationTestSession(groups []domain.TerminalGroup) domain.Session {
	return domain.Session{
		Version: 1,
		Name:    "Group mutation",
		Terminals: []domain.Terminal{
			terminalForGroupTest("a", "Alpha"),
			terminalForGroupTest("b", "Beta"),
			terminalForGroupTest("c", "Gamma"),
		},
		TerminalGroups: groups,
	}
}

func terminalTransitionCatalogSession(groups []domain.TerminalGroup) domain.Session {
	return domain.Session{
		Version: 1,
		Name:    "Terminal transition catalog",
		Terminals: []domain.Terminal{
			{
				ID: "a", Name: "Alpha",
				Root: domain.ContentNode{
					ID: "root", Type: domain.NodeFolder, Name: "ROOT",
					Children: []domain.ContentNode{{
						ID: "go", Type: domain.NodeCommand, Name: "GO",
						TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "b"},
					}},
				},
			},
			{
				ID: "b", Name: "Beta",
				Root: domain.ContentNode{
					ID: "root", Type: domain.NodeFolder, Name: "ROOT",
					Children: []domain.ContentNode{{
						ID: "status", Type: domain.NodeCommand, Name: "Status", Text: "All systems nominal.",
						StateChange: &domain.StateChangeConfig{
							CompletedName: "Status read", ConfirmationText: "Read status?",
						},
					}},
				},
				CommandStates: map[string]domain.CommandExecutionState{
					"status": {CompletedName: "Status read", ResultText: "All systems nominal."},
				},
			},
			terminalForGroupTest("c", "Gamma"),
		},
		TerminalGroups: groups,
	}
}

func assertCanonicalTerminalGroups(t *testing.T, session domain.Session) {
	t.Helper()

	terminalIDs := make(map[string]struct{}, len(session.Terminals))
	for _, terminal := range session.Terminals {
		terminalIDs[terminal.ID] = struct{}{}
	}
	groupIDs := make(map[string]struct{}, len(session.TerminalGroups))
	groupNames := make(map[string]struct{}, len(session.TerminalGroups))
	memberships := make(map[string]int, len(session.Terminals))
	for _, group := range session.TerminalGroups {
		require.NotEmpty(t, strings.TrimSpace(group.ID))
		require.NotEmpty(t, strings.TrimSpace(group.Name))
		require.NotEmpty(t, group.TerminalIDs)
		_, duplicateID := groupIDs[group.ID]
		require.False(t, duplicateID, "duplicate terminal group ID %q", group.ID)
		groupIDs[group.ID] = struct{}{}
		normalizedName := strings.ToLower(strings.TrimSpace(group.Name))
		_, duplicateName := groupNames[normalizedName]
		require.False(t, duplicateName, "duplicate normalized terminal group name %q", group.Name)
		groupNames[normalizedName] = struct{}{}
		for _, terminalID := range group.TerminalIDs {
			_, exists := terminalIDs[terminalID]
			require.True(t, exists, "group %q references missing terminal %q", group.ID, terminalID)
			memberships[terminalID]++
		}
	}
	require.Len(t, memberships, len(session.Terminals))
	for terminalID := range terminalIDs {
		require.Equal(t, 1, memberships[terminalID], "terminal %q must have exactly one group", terminalID)
	}
}

func terminalGroupByMember(t *testing.T, groups []domain.TerminalGroup, terminalID string) domain.TerminalGroup {
	t.Helper()

	for _, group := range groups {
		if slices.Contains(group.TerminalIDs, terminalID) {
			return group
		}
	}
	require.FailNowf(t, "terminal group not found", "terminal %q has no group in %#v", terminalID, groups)

	return domain.TerminalGroup{}
}

func assertInactive(t *testing.T, snapshot ActiveSession) {
	t.Helper()
	assert.Empty(t, snapshot.Path)
	assert.Nil(t, snapshot.Session)
	assert.Zero(t, snapshot.RequestedRevision)
	assert.Zero(t, snapshot.SavedRevision)
	assert.Equal(t, SaveStateIdle, snapshot.SaveState)
}

func assertNoApplicationSupportWrites(t *testing.T, fileSystem *testutil.FakeFileSystem) {
	t.Helper()
	for _, write := range fileSystem.WriteCalls() {
		assert.False(t, strings.HasPrefix(write.Path, testLocations.ApplicationSupport+string(filepath.Separator)), "session content written to Application Support: %q", write.Path)
	}
}

func fileSystemFileData(t *testing.T, fileSystem *testutil.FakeFileSystem, path string) []byte {
	t.Helper()
	data, ok := fileSystem.File(path)
	require.True(t, ok, "file %q does not exist", path)
	return data
}

func mustEncodeSession(t *testing.T, session domain.Session) []byte {
	t.Helper()
	data, err := domain.EncodeSession(session)
	require.NoError(t, err)
	return data
}

func waitForRequestedRevision(t *testing.T, service *Service, want uint64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if service.Snapshot().RequestedRevision >= want {
			return
		}
		runtime.Gosched()
	}
	require.GreaterOrEqual(t, service.Snapshot().RequestedRevision, want, "requested revision never reached; state = %#v", service.Snapshot())
}

type blockingStore struct {
	mu sync.Mutex

	files map[string][]byte

	firstWriteStarted chan struct{}
	releaseFirstWrite chan struct{}
	firstOnce         sync.Once
	releaseOnce       sync.Once
}

func newBlockingStore() *blockingStore {
	return &blockingStore{
		files:             make(map[string][]byte),
		firstWriteStarted: make(chan struct{}),
		releaseFirstWrite: make(chan struct{}),
	}
}

func (store *blockingStore) seed(path string, data []byte) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.files[path] = append([]byte(nil), data...)
}

func (store *blockingStore) Read(path string) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	data, ok := store.files[path]
	if !ok {
		return nil, fmt.Errorf("read %q: not found", path)
	}
	return append([]byte(nil), data...), nil
}

func (store *blockingStore) WriteAtomic(path string, data []byte) error {
	blocked := false
	store.firstOnce.Do(func() {
		blocked = true
		close(store.firstWriteStarted)
	})
	if blocked {
		<-store.releaseFirstWrite
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.files[path] = append([]byte(nil), data...)
	return nil
}

func (store *blockingStore) CopyAtomic(source, destination string) error {
	data, err := store.Read(source)
	if err != nil {
		return err
	}
	return store.WriteAtomic(destination, data)
}

func (store *blockingStore) file(path string) []byte {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]byte(nil), store.files[path]...)
}

func (store *blockingStore) release() {
	store.releaseOnce.Do(func() { close(store.releaseFirstWrite) })
}

type failingMutationStore struct {
	path   string
	data   []byte
	err    error
	writes int
}

func (store *failingMutationStore) Read(path string) ([]byte, error) {
	if path != store.path {
		return nil, fmt.Errorf("read unexpected path %q", path)
	}
	return append([]byte(nil), store.data...), nil
}

func (store *failingMutationStore) WriteAtomic(path string, data []byte) error {
	store.writes++
	if path != store.path {
		return fmt.Errorf("write unexpected path %q", path)
	}
	if store.err != nil {
		return store.err
	}
	store.data = append([]byte(nil), data...)
	return nil
}

func (store *failingMutationStore) CopyAtomic(source, destination string) error {
	return fmt.Errorf("unexpected copy from %q to %q", source, destination)
}
