package live

import (
	"encoding/json"
	"maps"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/nav"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControllerPresentationIsRuntimeOwnedAndProjected(t *testing.T) {
	runtimeType := reflect.TypeFor[domain.TerminalRuntime]()
	runtimeField, ok := runtimeType.FieldByName("Presentation")
	require.True(t, ok, "TerminalRuntime must own controller presentation")
	require.Equal(t, "ControllerTerminalPresentation", runtimeField.Type.Name())

	publicType := reflect.TypeFor[domain.PublicLiveState]()
	publicField, ok := publicType.FieldByName("Presentation")
	require.True(t, ok, "PublicLiveState must project controller presentation")
	require.Equal(t, runtimeField.Type, publicField.Type)
}

func TestControllerPresentationRevalidatesAfterNavigationAndContentChanges(t *testing.T) {
	service := New(nil, nil)
	target := domain.TerminalTarget{
		TerminalID: "terminal-1", TerminalName: "Overseer",
		Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{
			{ID: "docs", Type: domain.NodeFolder, Name: "DOCS", Children: []domain.ContentNode{
				{ID: "report", Type: domain.NodeEntry, Name: "REPORT", Description: "SYSTEM NOMINAL"},
			}},
			{ID: "status", Type: domain.NodeEntry, Name: "STATUS", Description: "ALL SYSTEMS OPERATIONAL"},
		}},
	}
	runtime, projection := service.CreateRuntime(target)
	require.NotNil(t, runtime)
	require.Equal(t, domain.ControllerTerminalPresentationMenu, projection.Presentation.Kind)
	require.Equal(t, "docs", projection.Presentation.TargetID)

	projection, ok := service.Apply(runtime, domain.RuntimeCommand{
		Kind: domain.RuntimeCommandPresentation,
		Presentation: domain.ControllerTerminalPresentation{
			Kind: domain.ControllerTerminalPresentationMenu, ContextKey: projection.Presentation.ContextKey, TargetID: "status",
		},
	})
	require.True(t, ok)
	require.Equal(t, "status", projection.Presentation.TargetID)

	projection, ok = service.Apply(runtime, domain.RuntimeCommand{Kind: domain.RuntimeCommandNavigate, Action: "entry", NodeID: "status"})
	require.True(t, ok)
	require.Equal(t, domain.ControllerTerminalPresentationPage, projection.Presentation.Kind)
	require.Equal(t, uint32(0), projection.Presentation.PageIndex)

	projection, ok = service.Apply(runtime, domain.RuntimeCommand{
		Kind: domain.RuntimeCommandPresentation,
		Presentation: domain.ControllerTerminalPresentation{
			Kind: domain.ControllerTerminalPresentationPage, ContextKey: projection.Presentation.ContextKey, PageIndex: 3,
		},
	})
	require.True(t, ok)
	require.Equal(t, uint32(3), projection.Presentation.PageIndex)

	target.Tree.Children = target.Tree.Children[:1]
	projection = service.UpdateRuntime(runtime, target)
	require.Equal(t, domain.ControllerTerminalPresentationMenu, projection.Presentation.Kind)
	require.Equal(t, "docs", projection.Presentation.TargetID)
}

func TestSetSnapshotIsDetachedAndSecretFree(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	tree := testTree()
	command := &tree.Children[0].Children[1]
	command.TerminalTransition = &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-2"}
	command.Extra = map[string]json.RawMessage{"future": json.RawMessage(`{"enabled":true}`)}

	first := service.Set("terminal-1", "Overseer", tree, 1, "WELCOME")
	require.Falsef(t, first == nil || first.Hack == nil,
		"Set() = %#v, want live terminal with puzzle", first)

	tree.Name = "MUTATED INPUT"
	tree.Children[0].Children[1].TerminalTransition.TargetTerminalID = "mutated-input"
	first.Tree.Name = "MUTATED RESULT"
	first.Tree.Children[0].Children[1].TerminalTransition.TargetTerminalID = "mutated-result"
	first.Tree.Children[0].Children[1].Extra["future"][0] = '['
	first.Nav.Path[0] = "mutated"
	first.Hack.Log = append(first.Hack.Log, "private mutation")

	snapshot := service.Snapshot()
	require.False(t, snapshot == nil,
		"Snapshot() returned nil")
	require.Falsef(t, snapshot.Tree.Name != "ROOT" || !cmp.Equal(snapshot.Nav.Path, []string{"root"}) || len(snapshot.Hack.Log) != 0,
		"canonical state was mutated through a boundary: %#v", snapshot)
	projectedCommand := snapshot.Tree.Children[0].Children[1]
	require.NotNil(t, projectedCommand.TerminalTransition)
	assert.Equal(t, "terminal-2", projectedCommand.TerminalTransition.TargetTerminalID)
	assert.Equal(t, json.RawMessage(`{"enabled":true}`), projectedCommand.Extra["future"])

	raw, err := json.Marshal(snapshot)
	if err != nil {
		require.NoError(t, err)
	}
	for _, privateField := range []string{"secretWord", "wordsById"} {
		assert.Falsef(t, strings.Contains(string(raw), privateField),
			"public snapshot leaked %q: %s", privateField, raw)

	}
}

func TestUpdateRevalidatesNavigationAndPreservesPuzzle(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	service.Set("terminal-1", "Overseer", testTree(), 1, "OLD")
	{
		_, ok := service.ApplyNav("enter", "docs")
		require.False(t, !ok,
			"ApplyNav() rejected active live terminal")
	}
	{

		_, ok := service.ApplyNav("entry", "report")
		require.False(t, !ok,
			"ApplyNav() rejected active entry")
	}

	before := service.Snapshot()
	intro := "NEW"

	updated, ok := service.Update(treeWithoutReport(), &intro)
	require.False(t, !ok,
		"Update() rejected active live terminal")
	require.Falsef(t, updated.IntroText != intro || updated.Nav.Mode != "list" || updated.Nav.ViewEntryID != nil,
		"Update() did not revalidate navigation: %#v", updated)
	require.False(t, !cmp.Equal(updated.Hack, before.Hack),
		"Update() reset or changed the active puzzle")

}

func TestApplyHackPatternReturnsDetachedAcceptedState(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	initial := service.Set("terminal-1", "Overseer", testTree(), 1, "WELCOME")
	require.Falsef(t, initial == nil || initial.Hack == nil || len(initial.Hack.Patterns) == 0,
		"Set() pattern state = %#v", initial)

	patternID := initial.Hack.Patterns[0].ID

	result, ok := activatePattern(service, patternID)
	require.Falsef(t, !ok || result == nil,
		"ApplyHackPattern(%q) = %#v, %t", patternID, result, ok)

	used := false
	for _, pattern := range result.Patterns {
		if pattern.ID == patternID {
			used = pattern.Used
		}
	}
	require.Falsef(t, !used,
		"accepted pattern %q was not marked used: %#v", patternID, result.Patterns)

	result.Patterns[0].ID = "mutated"
	snapshot := service.Snapshot()
	require.False(t, snapshot == nil || snapshot.Hack == nil || snapshot.Hack.Patterns[0].ID == "mutated",
		"public pattern projection mutated canonical state")

}

func TestClearAndAbsentActions(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	require.False(t, service.Snapshot() != nil,
		"new service unexpectedly has live state")
	{

		_, ok := service.Update(testTree(), nil)
		require.False(t, ok,
			"Update() succeeded without live state")
	}
	{

		_, ok := service.ApplyNav("back", "")
		require.False(t, ok,
			"ApplyNav() succeeded without live state")
	}
	{

		_, ok := service.ApplyHackGuess("A1")
		require.False(t, ok,
			"ApplyHackGuess() succeeded without a puzzle")
	}
	require.False(t, service.ApplyHackPattern("opaque-stale-pattern", nil),
		"ApplyHackPattern() succeeded without a puzzle")

	service.Set("terminal-1", "Overseer", testTree(), 0, "")
	service.Clear()
	service.Clear()
	require.False(t, service.Snapshot() != nil,
		"Clear() left stale live state")

}

func TestConcurrentTransitionsAndSnapshots(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	service.Set("terminal-1", "Overseer", testTree(), 1, "WELCOME")

	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for range 100 {
				switch worker % 4 {
				case 0:
					service.ApplyNav("enter", "docs")
					service.ApplyNav("back", "")
				case 1:
					service.ApplyHackGuess("missing")
				case 2:
					snapshot := service.Snapshot()
					if snapshot != nil && snapshot.Hack != nil && len(snapshot.Hack.Patterns) > 0 {
						service.ApplyHackPattern(snapshot.Hack.Patterns[0].ID, nil)
					}
				case 3:
					snapshot := service.Snapshot()
					if snapshot != nil {
						snapshot.Tree.Name = "external"
					}
				}
			}
		}(worker)
	}
	workers.Wait()

	snapshot := service.Snapshot()
	require.Falsef(t, snapshot == nil || snapshot.Tree.Name != "ROOT" || len(snapshot.Nav.Path) == 0 || snapshot.Nav.Path[0] != "root",
		"concurrent use corrupted canonical state: %#v", snapshot)

}

func TestConcurrentPatternUseAppliesOnceAndFreshSetResetsUsage(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	initial := service.Set("terminal-1", "Overseer", testTree(), 1, "WELCOME")
	require.Falsef(t, initial == nil || initial.Hack == nil || len(initial.Hack.Patterns) == 0,
		"Set() pattern state = %#v", initial)

	patternID := initial.Hack.Patterns[0].ID

	var accepted atomic.Int32
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			if service.ApplyHackPattern(patternID, nil) {
				accepted.Add(1)
			}
		})
	}
	workers.Wait()
	require.Falsef(t, accepted.Load() != 1,
		"accepted concurrent actions = %d, want 1", accepted.Load())

	beforeRejected := service.Snapshot()
	require.False(t, service.ApplyHackPattern(patternID, nil),
		"repeated pattern was accepted")
	{

		afterRejected := service.Snapshot()
		require.Falsef(t, !cmp.Equal(afterRejected, beforeRejected),
			"repeated pattern changed state\ngot: %#v\nwant: %#v", afterRejected, beforeRejected)
	}

	fresh := service.Set("terminal-1", "Overseer", testTree(), 1, "WELCOME")
	require.Falsef(t, fresh == nil || fresh.Hack == nil || len(fresh.Hack.Patterns) == 0,
		"fresh Set() pattern state = %#v", fresh)

	for _, pattern := range fresh.Hack.Patterns {
		require.Falsef(t, pattern.Used,
			"fresh puzzle retained used pattern %#v", pattern)

	}
}

func TestPatternGenerationRejectsStaleIDWithoutRandomnessOrPublication(t *testing.T) {
	random := newCountingRandom(1)
	service := New(random, fixedWords{})
	service.generationIDs = &sequenceGenerationIDs{values: []string{"generation-old", "generation-new"}}
	old := service.Set("terminal-1", "Overseer", testTree(), 1, "WELCOME")
	require.Falsef(t, old == nil || old.Hack == nil || len(old.Hack.Patterns) == 0,
		"old puzzle = %#v", old)

	staleID := old.Hack.Patterns[0].ID
	current := service.Set("terminal-1", "Overseer", testTree(), 1, "WELCOME")
	require.Falsef(t, current == nil || current.Hack == nil || len(current.Hack.Patterns) == 0 || current.Hack.Patterns[0].ID == staleID,
		"fresh generation did not replace opaque identities: old=%q current=%#v", staleID, current)

	beforeCalls := random.calls.Load()
	publications := atomic.Int32{}
	require.False(t, service.ApplyHackPattern(staleID, func(*domain.PublicHackState) { publications.Add(1) }),
		"stale generation pattern was accepted")
	require.Falsef(t, random.calls.Load() != beforeCalls || publications.Load() != 0,
		"stale request consumed RNG or published: calls=%d->%d publications=%d", beforeCalls, random.calls.Load(), publications.Load())
	{

		after := service.Snapshot()
		require.Falsef(t, !cmp.Equal(after, current),
			"stale request mutated current generation\ngot: %#v\nwant: %#v", after, current)
	}

}

func TestAcceptedPatternPublishesOnceAfterMutationAndDuplicatePublishesNever(t *testing.T) {
	random := newCountingRandom(1)
	service := New(random, fixedWords{})
	service.generationIDs = &sequenceGenerationIDs{values: []string{"generation-atomic"}}
	initial := service.Set("terminal-1", "Overseer", testTree(), 1, "WELCOME")
	patternID := initial.Hack.Patterns[0].ID
	random.value.Store(99)
	beforeCalls := random.calls.Load()

	var published []*domain.PublicHackState
	accepted := service.ApplyHackPattern(patternID, func(state *domain.PublicHackState) {
		published = append(published, state)
		require.Falsef(t, !publicPatternIsUsed(state, patternID),
			"callback ran before used marking: %#v", state.Patterns)

	})
	require.Falsef(t, !accepted || len(published) != 1 || random.calls.Load() != beforeCalls+1,
		"accepted=%t publications=%d RNG=%d->%d, want true/1/+1", accepted, len(published), beforeCalls, random.calls.Load())

	published[0].Patterns[0].ID = "mutated-return"
	{
		snapshot := service.Snapshot()
		require.False(t, snapshot.Hack.Patterns[0].ID == "mutated-return",
			"callback projection retained a canonical reference")
	}

	beforeCalls = random.calls.Load()
	require.False(t, service.ApplyHackPattern(patternID, func(*domain.PublicHackState) { published = append(published, nil) }),
		"duplicate pattern was accepted")
	require.Falsef(t, len(published) != 1 || random.calls.Load() != beforeCalls,
		"duplicate request published or consumed RNG: publications=%d calls=%d->%d", len(published), beforeCalls, random.calls.Load())

}

func TestApplyHackPatternSerializesPublicationBeforeNextTransition(t *testing.T) {
	random := newCountingRandom(1)
	service := New(random, fixedWords{})
	service.generationIDs = &sequenceGenerationIDs{values: []string{"generation-ordered"}}
	initial := service.Set("terminal-1", "Overseer", testTree(), 1, "WELCOME")
	require.GreaterOrEqual(t, len(initial.Hack.Patterns), 2)
	firstPatternID := initial.Hack.Patterns[0].ID
	secondPatternID := initial.Hack.Patterns[1].ID
	random.value.Store(99)

	firstPublishStarted := make(chan struct{})
	releaseFirstPublish := make(chan struct{})
	publicationOrder := make(chan string, 2)
	firstDone := make(chan bool, 1)
	go func() {
		firstDone <- service.ApplyHackPattern(firstPatternID, func(*domain.PublicHackState) {
			close(firstPublishStarted)
			<-releaseFirstPublish
			publicationOrder <- firstPatternID
		})
	}()
	<-firstPublishStarted

	mutexHeld := !service.mu.TryLock()
	if !mutexHeld {
		service.mu.Unlock()
	}

	secondStarted := make(chan struct{})
	secondDone := make(chan bool, 1)
	go func() {
		close(secondStarted)
		secondDone <- service.ApplyHackPattern(secondPatternID, func(*domain.PublicHackState) {
			publicationOrder <- secondPatternID
		})
	}()
	<-secondStarted
	close(releaseFirstPublish)

	require.True(t, <-firstDone)
	require.True(t, <-secondDone)
	require.True(t, mutexHeld, "publication callback ran after releasing the live-service mutex")
	require.Equal(t, firstPatternID, <-publicationOrder)
	require.Equal(t, secondPatternID, <-publicationOrder)
}

func TestTerminalRuntimeLifecycleCreatesUpdatesAndProjectsDetachedCheckpoints(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	service.generationIDs = &sequenceGenerationIDs{values: []string{"runtime-generation-1", "runtime-generation-2"}}
	target := domain.TerminalTarget{
		TerminalID: "terminal-1", TerminalName: "Overseer", Tree: testTree(), HackLevel: 1, IntroText: "OLD",
	}
	runtime, created := service.CreateRuntime(target)
	require.Falsef(t, runtime == nil || created == nil || runtime.Hack == nil || runtime.Hack.GenerationID != "runtime-generation-1",
		"CreateRuntime() = runtime %#v projection %#v", runtime, created)
	require.Falsef(t, runtime.Lifecycle != domain.TerminalLifecycleActive || !cmp.Equal(runtime.Nav.Path, []string{"root"}),
		"fresh runtime lifecycle/nav = %#v", runtime)
	require.Falsef(t, created.Hack == nil || created.TerminalID != target.TerminalID,
		"fresh public projection = %#v", created)

	created.Tree.Name = "MUTATED PROJECTION"
	created.Nav.Path[0] = "mutated"
	created.Hack.Log = append(created.Hack.Log, "mutated")
	projected := service.ProjectRuntime(runtime)
	require.Falsef(t, projected.Tree.Name != "ROOT" || !cmp.Equal(projected.Nav.Path, []string{"root"}) || len(projected.Hack.Log) != 0,
		"projection aliases private runtime: %#v", projected)
	{

		_, ok := service.Apply(runtime, domain.RuntimeCommand{Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs"})
		require.False(t, !ok,
			"Apply() rejected created runtime")
	}
	{

		_, ok := service.Apply(runtime, domain.RuntimeCommand{Kind: domain.RuntimeCommandNavigate, Action: "entry", NodeID: "report"})
		require.False(t, !ok,
			"Apply() rejected created runtime entry")
	}

	privateHackBefore := cloneHackForLifecycleTest(runtime.Hack)
	updatedTarget := target
	updatedTarget.TerminalName = "Overseer Updated"
	updatedTarget.Tree = treeWithoutReport()
	updatedTarget.IntroText = "NEW"
	updated := service.UpdateRuntime(runtime, updatedTarget)
	require.Falsef(t, updated == nil || updated.TerminalName != "Overseer Updated" || updated.IntroText != "NEW" || updated.Nav.Mode != "list" || updated.Nav.ViewEntryID != nil,
		"UpdateRuntime() did not update metadata/revalidate nav: %#v", updated)
	require.False(t, !cmp.Equal(runtime.Hack, privateHackBefore) || runtime.Hack.GenerationID != "runtime-generation-1",
		"UpdateRuntime() regenerated or changed private puzzle")
	require.Falsef(t, service.generationIDs.(*sequenceGenerationIDs).next != 1,
		"UpdateRuntime() consumed a new generation: %d", service.generationIDs.(*sequenceGenerationIDs).next)

	second, _ := service.CreateRuntime(domain.TerminalTarget{
		TerminalID: "terminal-2", TerminalName: "Archive", Tree: testTree(), HackLevel: 1,
	})
	require.Falsef(t, second.Hack == nil || second.Hack.GenerationID != "runtime-generation-2",
		"second fresh runtime did not generate a fresh puzzle: %#v", second.Hack)

}

func TestTerminalRuntimeLifecyclePreservesExactPrivateCheckpointAndDiscardRegenerates(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	service.generationIDs = &sequenceGenerationIDs{values: []string{"checkpoint-generation-1", "checkpoint-generation-2"}}
	target := domain.TerminalTarget{
		TerminalID: "terminal-1", TerminalName: "Overseer", Tree: testTree(), HackLevel: 1, IntroText: "OLD",
	}
	runtime, initial := service.CreateRuntime(target)
	require.Falsef(t, runtime == nil || runtime.Hack == nil || initial == nil || initial.Hack == nil,
		"CreateRuntime() = runtime %#v projection %#v", runtime, initial)
	{

		_, ok := service.Apply(runtime, domain.RuntimeCommand{Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "docs"})
		require.False(t, !ok,
			"navigation into checkpoint was rejected")
	}
	{

		_, ok := service.Apply(runtime, domain.RuntimeCommand{Kind: domain.RuntimeCommandNavigate, Action: "entry", NodeID: "report"})
		require.False(t, !ok,
			"entry navigation into checkpoint was rejected")
	}

	wrongTarget := ""
	for id, candidate := range runtime.Hack.WordsByID {
		if candidate.Text != runtime.Hack.SecretWord {
			wrongTarget = id
			break
		}
	}
	require.False(t, wrongTarget == "",
		"generated puzzle has no non-secret candidate")
	{

		_, ok := service.Apply(runtime, domain.RuntimeCommand{Kind: domain.RuntimeCommandGuess, TargetID: wrongTarget})
		require.False(t, !ok,
			"wrong guess was rejected")
	}

	patternID := service.ProjectRuntime(runtime).Hack.Patterns[0].ID
	{
		_, ok := service.Apply(runtime, domain.RuntimeCommand{Kind: domain.RuntimeCommandActivatePattern, PatternID: patternID})
		require.False(t, !ok,
			"pattern use was rejected")
	}

	privateBefore := cloneHackForLifecycleTest(runtime.Hack)
	navBefore := cloneNav(runtime.Nav)
	service.SuspendRuntime(runtime)
	require.Falsef(t, runtime.Lifecycle != domain.TerminalLifecycleSuspended || !cmp.Equal(runtime.Hack, privateBefore) || !cmp.Equal(runtime.Nav, navBefore),
		"SuspendRuntime() changed exact checkpoint: %#v", runtime)

	latest := target
	latest.TerminalName = "Overseer Renamed"
	latest.IntroText = "LATEST"
	latest.Tree = treeWithoutReport()
	restored := service.ReactivateRuntime(runtime, latest)
	require.Falsef(t, restored == nil || runtime.Lifecycle != domain.TerminalLifecycleActive,
		"ReactivateRuntime() = %#v, lifecycle %q", restored, runtime.Lifecycle)
	require.Falsef(t, runtime.TerminalName != latest.TerminalName || runtime.IntroText != latest.IntroText || runtime.Tree.ID != latest.Tree.ID || len(runtime.Tree.Children) != 1 || len(runtime.Tree.Children[0].Children) != 1 || runtime.Tree.Children[0].Children[0].ID != "read",
		"reactivation did not apply latest authored content: %#v", runtime)
	require.Falsef(t, !cmp.Equal(runtime.Hack, privateBefore),
		"reactivation changed secret/generation/board/attempts/candidates/patterns/log/outcome:\n got %#v\nwant %#v", runtime.Hack, privateBefore)
	require.Falsef(t, runtime.Nav.Mode != "list" || runtime.Nav.ViewEntryID != nil || !cmp.Equal(runtime.Nav.Path, []string{"root", "docs"}),
		"reactivation did not revalidate navigation against refreshed content: %#v", runtime.Nav)
	require.Falsef(t, service.generationIDs.(*sequenceGenerationIDs).next != 1,
		"preserve consumed a fresh generation: %d", service.generationIDs.(*sequenceGenerationIDs).next)

	discarded, fresh := service.DiscardRuntime(latest)
	require.Falsef(t, discarded == nil || fresh == nil || discarded.Hack == nil,
		"DiscardRuntime() = runtime %#v projection %#v", discarded, fresh)
	require.Falsef(t, discarded.Hack.GenerationID != "checkpoint-generation-2" || discarded.Hack.GenerationID == privateBefore.GenerationID,
		"discard retained prior generation: old %q new %#v", privateBefore.GenerationID, discarded.Hack)
	require.Falsef(t, discarded.Hack.AttemptsLeft != discarded.Hack.AttemptsMax || discarded.Hack.Solved || discarded.Hack.Failed || len(discarded.Hack.Log) != 0 || len(discarded.Hack.UsedPatterns) != 0,
		"discard did not create a fresh puzzle: %#v", discarded.Hack)
	require.Falsef(t, !cmp.Equal(discarded.Nav, domain.NavState{Mode: "list", Path: []string{"root"}}),
		"discard did not reset navigation: %#v", discarded.Nav)
	require.

		// Generation identity remains private; the fresh public board proves the
		// replacement without exposing that identifier.
		False(t, fresh.Hack == nil,
			"discard projection omitted fresh public puzzle")

}

func TestTerminalRuntimeReactivationRetainsSolvedUnfinishedAndFailedHackAcrossTenVisits(t *testing.T) {
	t.Parallel()
	for _, outcome := range []struct {
		name   string
		solved bool
		failed bool
	}{
		{name: "solved", solved: true},
		{name: "unfinished"},
		{name: "failed", failed: true},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			t.Parallel()
			service := New(&constantRandom{}, fixedWords{})
			target := domain.TerminalTarget{TerminalID: "checkpoint", TerminalName: "Checkpoint", Tree: testTree(), HackLevel: 1}
			runtime, _ := service.CreateRuntime(target)
			require.NotNil(t, runtime.Hack)
			runtime.Hack.Solved = outcome.solved
			runtime.Hack.Failed = outcome.failed
			runtime.Hack.AttemptsLeft = 2
			runtime.Hack.Log = []string{"retained"}
			before := cloneHackForLifecycleTest(runtime.Hack)
			for visit := range 10 {
				service.SuspendRuntime(runtime)
				projection := service.ReactivateRuntime(runtime, target)
				require.NotNil(t, projection, "visit %d", visit)
				assert.Equal(t, before, runtime.Hack, "visit %d", visit)
			}
		})
	}
}

func TestResetFailedHackReplacesOnlyEligibleRuntimeFromLatestTarget(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	service.generationIDs = &sequenceGenerationIDs{values: []string{"failed-generation", "retry-generation"}}
	original := domain.TerminalTarget{
		TerminalID: "terminal-1", TerminalName: "Overseer", Tree: testTree(), HackLevel: 1, IntroText: "OLD",
	}
	runtime, _ := service.CreateRuntime(original)
	oldTargetID := ""
	for targetID := range runtime.Hack.WordsByID {
		oldTargetID = targetID
		break
	}
	runtime.Hack.AttemptsLeft = 0
	runtime.Hack.Failed = true
	runtime.Hack.Log = []string{"TERMINAL LOCKED"}

	latest := original
	latest.TerminalName = "Overseer Renamed"
	latest.Tree = treeWithoutReport()
	latest.HackLevel = 2
	latest.IntroText = "LATEST"
	replacement, projection := service.ResetFailedHack(runtime, latest)
	require.Falsef(t, replacement == nil || projection == nil || replacement.Hack == nil,
		"ResetFailedHack() = runtime %#v projection %#v", replacement, projection)
	require.Falsef(t, replacement.Hack.GenerationID != "retry-generation" || replacement.Hack.Level != 2 || replacement.Hack.AttemptsLeft != replacement.Hack.AttemptsMax || replacement.Hack.Failed || replacement.Hack.Solved || len(replacement.Hack.Log) != 0,
		"replacement puzzle = %#v, want fresh level-2 generation", replacement.Hack)
	require.Falsef(t, replacement.TerminalID != latest.TerminalID || replacement.TerminalName != latest.TerminalName || replacement.IntroText != latest.IntroText || replacement.Tree.ID != latest.Tree.ID || len(replacement.Tree.Children) != len(latest.Tree.Children),
		"replacement authored state = %#v, want latest target %#v", replacement, latest)
	require.Falsef(t, projection.Hack == nil || projection.Hack.AttemptsLeft != projection.Hack.AttemptsMax || projection.Hack.Failed,
		"replacement projection = %#v", projection)
	{

		_, reused := replacement.Hack.WordsByID[oldTargetID]
		require.Falsef(t, reused,
			"fresh generation reused stale candidate identity %q", oldTargetID)
	}

	freshBeforeStaleAction := cloneRuntimeForLifecycleTest(replacement)
	{
		_, accepted := service.Apply(replacement, domain.RuntimeCommand{Kind: domain.RuntimeCommandGuess, TargetID: oldTargetID})
		require.Falsef(t, accepted || !cmp.Equal(replacement, freshBeforeStaleAction),
			"stale generation action was accepted or mutated replacement: accepted=%t", accepted)
	}

	for name, mutate := range map[string]func(*domain.TerminalRuntime, *domain.TerminalTarget){
		"unfinished": func(candidate *domain.TerminalRuntime, _ *domain.TerminalTarget) {
			candidate.Hack.Failed = false
			candidate.Hack.AttemptsLeft = 1
		},
		"solved":         func(candidate *domain.TerminalRuntime, _ *domain.TerminalTarget) { candidate.Hack.Solved = true },
		"wrong terminal": func(_ *domain.TerminalRuntime, target *domain.TerminalTarget) { target.TerminalID = "terminal-2" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneRuntimeForLifecycleTest(runtime)
			target := latest
			mutate(candidate, &target)
			before := cloneRuntimeForLifecycleTest(candidate)
			beforeGeneration := service.generationIDs.(*sequenceGenerationIDs).next
			got, public := service.ResetFailedHack(candidate, target)
			require.Falsef(t, got != nil || public != nil || !cmp.Equal(candidate, before) || service.generationIDs.(*sequenceGenerationIDs).next != beforeGeneration,
				"ineligible reset = runtime %#v projection %#v candidate %#v", got, public, candidate)

		})
	}
}

func cloneRuntimeForLifecycleTest(state *domain.TerminalRuntime) *domain.TerminalRuntime {
	if state == nil {
		return nil
	}
	clone := *state
	clone.Tree = domain.CloneContentNode(state.Tree)
	clone.Nav = cloneNav(state.Nav)
	clone.Hack = cloneHackForLifecycleTest(state.Hack)
	return &clone
}

func cloneHackForLifecycleTest(state *domain.HackState) *domain.HackState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.WordsByID = make(map[string]domain.HackCandidate, len(state.WordsByID))
	maps.Copy(clone.WordsByID, state.WordsByID)
	clone.UsedPatterns = make(map[domain.HackPatternIdentity]struct{}, len(state.UsedPatterns))
	for identity := range state.UsedPatterns {
		clone.UsedPatterns[identity] = struct{}{}
	}
	if state.Log != nil {
		clone.Log = make([]string, len(state.Log))
		copy(clone.Log, state.Log)
	}
	if state.Columns != nil {
		clone.Columns = make([]domain.HackColumn, len(state.Columns))
		copy(clone.Columns, state.Columns)
	}
	for index := range clone.Columns {
		if state.Columns[index].Addresses != nil {
			clone.Columns[index].Addresses = make([]string, len(state.Columns[index].Addresses))
			copy(clone.Columns[index].Addresses, state.Columns[index].Addresses)
		}
		if state.Columns[index].Words != nil {
			clone.Columns[index].Words = make([]domain.HackWord, len(state.Columns[index].Words))
			copy(clone.Columns[index].Words, state.Columns[index].Words)
		}
	}
	return &clone
}

func activatePattern(service *Service, patternID string) (*domain.PublicHackState, bool) {
	var result *domain.PublicHackState
	ok := service.ApplyHackPattern(patternID, func(state *domain.PublicHackState) { result = state })
	return result, ok
}

func publicPatternIsUsed(state *domain.PublicHackState, patternID string) bool {
	if state == nil {
		return false
	}
	for _, pattern := range state.Patterns {
		if pattern.ID == patternID {
			return pattern.Used
		}
	}
	return false
}

type sequenceGenerationIDs struct {
	values []string
	next   int
}

func (source *sequenceGenerationIDs) Next() string {
	if source.next >= len(source.values) {
		return "generation-overflow"
	}
	value := source.values[source.next]
	source.next++
	return value
}

type countingRandom struct {
	value atomic.Int32
	calls atomic.Int64
}

func newCountingRandom(value int32) *countingRandom {
	random := &countingRandom{}
	random.value.Store(value)
	return random
}

func (random *countingRandom) Intn(limit int) int {
	random.calls.Add(1)
	return int(random.value.Load()) % limit
}

type constantRandom struct{}

func (*constantRandom) Intn(limit int) int {
	if limit <= 1 {
		return 0
	}
	return 1
}

type fixedWords struct{}

func (fixedWords) PickWords(length, count int) []string {
	pools := map[int][]string{
		4: {"CODE", "CAVE", "DUST", "IRON", "GATE", "BOLT", "RAMP", "CORE", "FUSE", "GRID", "LAMP", "MASK", "NODE", "PIPE", "RING", "RUST"},
		5: {"ALLOY", "ARMOR", "ATLAS", "BASIN", "BLAST", "BRICK", "CABLE", "CACHE", "CARGO", "CLIFF", "CLOCK", "CRANE", "CRATE", "CREEK", "DRAIN", "DRONE"},
	}
	return append([]string(nil), pools[length][:count]...)
}

func testTree() domain.ContentNode {
	return domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{
			{
				ID: "docs", Type: domain.NodeFolder, Name: "DOCS",
				Children: []domain.ContentNode{
					{ID: "report", Type: domain.NodeEntry, Name: "REPORT", Description: "Report"},
					{ID: "read", Type: domain.NodeCommand, Name: "READ", Text: "Reading"},
				},
			},
		},
	}
}

func treeWithoutReport() domain.ContentNode {
	tree := testTree()
	tree.Children[0].Children = tree.Children[0].Children[1:]
	return tree
}

func TestEffectiveTreeAppliesFrozenEntryChangesIndependently(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		states      map[string]domain.CommandExecutionState
		description string
	}{
		{
			name:        "authored values",
			description: "POWER: OFFLINE\n\nCOOLING: OFFLINE\n\nSTATUS: NOMINAL",
		},
		{
			name: "power completed",
			states: map[string]domain.CommandExecutionState{
				"restore-power": entryContentStateForTest("power", "POWER: ONLINE"),
			},
			description: "POWER: ONLINE\n\nCOOLING: OFFLINE\n\nSTATUS: NOMINAL",
		},
		{
			name: "cooling completed",
			states: map[string]domain.CommandExecutionState{
				"restore-cooling": entryContentStateForTest("cooling", "COOLING: ONLINE"),
			},
			description: "POWER: OFFLINE\n\nCOOLING: ONLINE\n\nSTATUS: NOMINAL",
		},
		{
			name: "both completed",
			states: map[string]domain.CommandExecutionState{
				"restore-power":   entryContentStateForTest("power", "POWER: ONLINE"),
				"restore-cooling": entryContentStateForTest("cooling", "COOLING: ONLINE"),
			},
			description: "POWER: ONLINE\n\nCOOLING: ONLINE\n\nSTATUS: NOMINAL",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tree := entryContentTargetForTest().Tree
			projected := effectiveTree(tree, test.states)
			entry := findContentNode(projected, "reactor-status")
			require.NotNil(t, entry)
			assert.Equal(t, test.description, entry.Description)
		})
	}
}

func TestEffectiveTreeComposesBlocksInAuthoredOrderWithTwoNewlines(t *testing.T) {
	t.Parallel()

	tree := entryContentTargetForTest().Tree
	states := map[string]domain.CommandExecutionState{
		"restore-status":  entryContentStateForTest("status", "THIRD"),
		"restore-power":   entryContentStateForTest("power", "FIRST"),
		"restore-cooling": entryContentStateForTest("cooling", "SECOND"),
	}

	projected := effectiveTree(tree, states)
	entry := findContentNode(projected, "reactor-status")
	require.NotNil(t, entry)
	assert.Equal(t, "FIRST\n\nSECOND\n\nTHIRD", entry.Description)
}

func TestEffectiveTreeProjectionIsDetachedFromAuthoredAndRuntimeState(t *testing.T) {
	t.Parallel()

	service := New(nil, nil)
	target := entryContentTargetForTest()
	target.CommandStates = map[string]domain.CommandExecutionState{
		"restore-power": entryContentStateForTest("power", "POWER: ONLINE"),
	}
	runtime, first := service.CreateRuntime(target)
	require.NotNil(t, runtime)
	require.NotNil(t, first)
	require.Equal(t, "POWER: ONLINE\n\nCOOLING: OFFLINE\n\nSTATUS: NOMINAL", first.Tree.Children[0].Description)

	first.Tree.Children[0].Description = "MUTATED DESCRIPTION"
	first.Tree.Children[0].Blocks[0].InitialText = "MUTATED BLOCK"
	first.Tree.Children[2].StateChange.EntryContentChange.CompletedText = "MUTATED CONFIG"

	second := service.ProjectRuntime(runtime)
	require.NotNil(t, second)
	assert.Equal(t, "POWER: ONLINE\n\nCOOLING: OFFLINE\n\nSTATUS: NOMINAL", second.Tree.Children[0].Description)
	assert.Equal(t, "POWER: OFFLINE", second.Tree.Children[0].Blocks[0].InitialText)
	assert.Equal(t, "POWER: ONLINE", second.Tree.Children[2].StateChange.EntryContentChange.CompletedText)
	assert.Empty(t, runtime.Tree.Children[0].Description)
	assert.Equal(t, "POWER: OFFLINE", runtime.Tree.Children[0].Blocks[0].InitialText)
	assert.Equal(t, "POWER: ONLINE", runtime.CommandStates["restore-power"].EntryContentChange.CompletedText)
	assert.Empty(t, target.Tree.Children[0].Description)
	assert.Equal(t, "POWER: OFFLINE", target.Tree.Children[0].Blocks[0].InitialText)
	assert.Equal(t, "POWER: ONLINE", target.CommandStates["restore-power"].EntryContentChange.CompletedText)
}

func TestEffectiveTreePreservesLegacyDescriptions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		description string
	}{
		{name: "empty", description: ""},
		{name: "text", description: "LEGACY STATUS"},
		{name: "whitespace", description: "  exact whitespace\nremains  "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			legacy := domain.ContentNode{
				ID: "legacy", Type: domain.NodeEntry, Name: "LEGACY", Description: test.description,
			}
			projected := effectiveTree(legacy, nil)
			assert.Equal(t, test.description, projected.Description)
			assert.Nil(t, projected.Blocks)
		})
	}
}

func TestStateChangingCommandProjectsInitialAndFrozenCompletedContent(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	target := stateChangingTarget()

	initialRuntime, initial := service.CreateRuntime(target)
	require.NotNil(t, initialRuntime)
	require.NotNil(t, initial)
	initialCommand := findContentNode(initial.Tree, "doors")
	require.NotNil(t, initialCommand)
	assert.Equal(t, "Открыть двери", initialCommand.Name)
	assert.Equal(t, "Доступ в сектор разрешён.", initialCommand.Text)

	target.CommandStates = map[string]domain.CommandExecutionState{
		"doors": {CompletedName: "Двери открыты", ResultText: "Проход разблокирован."},
	}
	completedRuntime, completed := service.CreateRuntime(target)
	require.NotNil(t, completedRuntime)
	require.NotNil(t, completed)
	completedCommand := findContentNode(completed.Tree, "doors")
	require.NotNil(t, completedCommand)
	assert.Equal(t, "Двери открыты", completedCommand.Name)
	assert.Equal(t, "Проход разблокирован.", completedCommand.Text)

	// The authored source remains detached and retains the values for the next
	// execution after a master reset.
	authoredCommand := findContentNode(target.Tree, "doors")
	require.NotNil(t, authoredCommand)
	assert.Equal(t, "Открыть двери", authoredCommand.Name)
	assert.Equal(t, "Доступ в сектор разрешён.", authoredCommand.Text)
}

func TestPendingCommandBlocksEverySharedRuntimeAction(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	runtime, _ := service.CreateRuntime(stateChangingTarget())
	require.NotNil(t, runtime)
	runtime.CommandExecution = &domain.CommandExecutionPresentation{
		Phase:     domain.CommandExecutionPhasePending,
		CommandID: "doors",
	}
	before := cloneTerminalRuntimeForTest(runtime)

	commands := []domain.RuntimeCommand{
		{Kind: domain.RuntimeCommandNavigate, Action: "back"},
		{Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "doors"},
		{Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "doors"},
		{Kind: domain.RuntimeCommandGuess, TargetID: "guess"},
		{Kind: domain.RuntimeCommandActivatePattern, PatternID: "pattern"},
	}
	for _, command := range commands {
		projection, accepted := service.Apply(runtime, command)
		assert.False(t, accepted, "Apply(%+v) accepted while command approval was pending", command)
		assert.Nil(t, projection)
		assert.Equal(t, before, runtime)
	}
}

func TestUniversalApprovalPresentationKeepsEveryCommandModeOutOfRuntime(t *testing.T) {
	for _, test := range []struct {
		name      string
		target    domain.TerminalTarget
		completed bool
	}{
		{
			name: "ordinary",
			target: domain.TerminalTarget{
				TerminalID: "terminal-ordinary", TerminalName: "Diagnostics",
				Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{{
					ID: "command", Type: domain.NodeCommand, Name: "RUN", Text: "DONE",
				}}},
			},
		},
		{name: "initial state-changing", target: stateChangingTarget()},
		{name: "completed state-changing", target: stateChangingTarget(), completed: true},
		{
			name: "terminal transition",
			target: domain.TerminalTarget{
				TerminalID: "terminal-transition", TerminalName: "Transit",
				Tree: domain.ContentNode{ID: "root", Type: domain.NodeFolder, Name: "ROOT", Children: []domain.ContentNode{{
					ID: "command", Type: domain.NodeCommand, Name: "GO",
					TerminalTransition: &domain.TerminalTransitionConfig{TargetTerminalID: "terminal-b"},
				}}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := New(&constantRandom{}, fixedWords{})
			target := test.target
			commandID := target.Tree.Children[0].ID
			if test.completed {
				target.CommandStates = map[string]domain.CommandExecutionState{
					commandID: {CompletedName: "DONE", ResultText: "DONE"},
				}
			}
			runtime, _ := service.CreateRuntime(target)
			require.NotNil(t, runtime)
			runtime.CommandExecution = &domain.CommandExecutionPresentation{
				Phase: domain.CommandExecutionPhasePending, CommandID: commandID,
			}
			before := cloneTerminalRuntimeForTest(runtime)

			projection, accepted := service.Apply(runtime, domain.RuntimeCommand{
				Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: commandID,
			})
			assert.False(t, accepted)
			assert.Nil(t, projection)
			assert.Equal(t, before, runtime)
		})
	}
}

func TestRejectedCommandAcceptsOnlyBackAndClearsPresentation(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	runtime, _ := service.CreateRuntime(stateChangingTarget())
	require.NotNil(t, runtime)
	runtime.CommandExecution = &domain.CommandExecutionPresentation{
		Phase:     domain.CommandExecutionPhaseRejected,
		CommandID: "doors",
	}

	for _, command := range []domain.RuntimeCommand{
		{Kind: domain.RuntimeCommandNavigate, Action: "enter", NodeID: "doors"},
		{Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "doors"},
	} {
		projection, accepted := service.Apply(runtime, command)
		assert.False(t, accepted)
		assert.Nil(t, projection)
		require.NotNil(t, runtime.CommandExecution)
	}

	projection, accepted := service.Apply(runtime, domain.RuntimeCommand{
		Kind: domain.RuntimeCommandNavigate, Action: "back",
	})
	require.True(t, accepted)
	require.NotNil(t, projection)
	assert.Nil(t, runtime.CommandExecution)
	assert.Nil(t, projection.CommandExecution)
	assert.Equal(t, nav.Default(), projection.Nav)
}

func TestReactivateRuntimeClearsRejectedCommandAndPreservesSolvedHack(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	target := stateChangingTarget()
	target.HackLevel = 1
	runtime, _ := service.CreateRuntime(target)
	require.NotNil(t, runtime)
	require.NotNil(t, runtime.Hack)
	runtime.Hack.Solved = true
	runtime.Hack.Log = append(runtime.Hack.Log, "ACCESS GRANTED")
	runtime.CommandExecution = &domain.CommandExecutionPresentation{
		Phase:     domain.CommandExecutionPhaseRejected,
		CommandID: "doors",
	}
	service.SuspendRuntime(runtime)
	wantHack := cloneHackForLifecycleTest(runtime.Hack)

	projection := service.ReactivateRuntime(runtime, target)

	require.NotNil(t, projection)
	assert.Equal(t, domain.TerminalLifecycleActive, runtime.Lifecycle)
	assert.Equal(t, wantHack, runtime.Hack)
	assert.Nil(t, runtime.CommandExecution)
	assert.Nil(t, projection.CommandExecution)
}

func TestCompletedCommandRepeatsFrozenResultWithoutChangingSnapshot(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	target := stateChangingTarget()
	target.CommandStates = map[string]domain.CommandExecutionState{
		"doors": {CompletedName: "Двери открыты", ResultText: "Проход разблокирован."},
	}
	runtime, _ := service.CreateRuntime(target)
	require.NotNil(t, runtime)

	for attempt := range 100 {
		projection, accepted := service.Apply(runtime, domain.RuntimeCommand{
			Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "doors",
		})
		require.True(t, accepted, "completed repeat %d", attempt)
		require.NotNil(t, projection, "completed repeat %d", attempt)
		require.NotNil(t, projection.Nav.CommandNodeID, "completed repeat %d", attempt)
		assert.Equal(t, "doors", *projection.Nav.CommandNodeID, "completed repeat %d", attempt)
		command := findContentNode(projection.Tree, "doors")
		require.NotNil(t, command, "completed repeat %d", attempt)
		assert.Equal(t, "Двери открыты", command.Name, "completed repeat %d", attempt)
		assert.Equal(t, "Проход разблокирован.", command.Text, "completed repeat %d", attempt)
		assert.Equal(t, target.CommandStates, runtime.CommandStates, "completed repeat %d", attempt)
	}
}

func TestCompletedCommandResultAcceptsBackWithoutChangingSnapshot(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	target := stateChangingTarget()
	target.CommandStates = map[string]domain.CommandExecutionState{
		"doors": {CompletedName: "Двери открыты", ResultText: "Проход разблокирован."},
	}
	runtime, _ := service.CreateRuntime(target)
	require.NotNil(t, runtime)

	projection, accepted := service.Apply(runtime, domain.RuntimeCommand{
		Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "doors",
	})
	require.True(t, accepted)
	require.NotNil(t, projection)
	require.NotNil(t, projection.Nav.CommandNodeID)

	projection, accepted = service.Apply(runtime, domain.RuntimeCommand{
		Kind: domain.RuntimeCommandNavigate, Action: "back",
	})
	require.True(t, accepted)
	require.NotNil(t, projection)
	assert.Nil(t, projection.Nav.CommandNodeID)
	assert.Equal(t, nav.Default(), projection.Nav)
	assert.Equal(t, target.CommandStates, runtime.CommandStates)
}

func TestOrdinaryCommandKeepsLegacyResultPathWithoutExecutionPresentation(t *testing.T) {
	service := New(&constantRandom{}, fixedWords{})
	target := domain.TerminalTarget{
		TerminalID: "terminal-ordinary", TerminalName: "Diagnostics",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: "diagnostic", Type: domain.NodeCommand,
				Name: "RUN DIAGNOSTIC", Text: "SYSTEM NOMINAL",
			}},
		},
	}
	runtime, initial := service.CreateRuntime(target)
	require.NotNil(t, runtime)
	require.NotNil(t, initial)
	require.Nil(t, runtime.CommandExecution)
	require.Empty(t, runtime.CommandStates)

	for range 2 {
		projection, accepted := service.Apply(runtime, domain.RuntimeCommand{
			Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: "diagnostic",
		})
		require.True(t, accepted)
		require.NotNil(t, projection)
		require.Nil(t, projection.CommandExecution)
		require.NotNil(t, projection.Nav.CommandNodeID)
		require.Equal(t, "diagnostic", *projection.Nav.CommandNodeID)
		command := findContentNode(projection.Tree, "diagnostic")
		require.NotNil(t, command)
		require.Equal(t, "RUN DIAGNOSTIC", command.Name)
		require.Equal(t, "SYSTEM NOMINAL", command.Text)
		require.Empty(t, runtime.CommandStates)
	}
}

func stateChangingTarget() domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: "terminal-1", TerminalName: "Overseer",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{{
				ID: "doors", Type: domain.NodeCommand, Name: "Открыть двери",
				Text: "Доступ в сектор разрешён.",
				StateChange: &domain.StateChangeConfig{
					CompletedName: "Двери разблокированы", ConfirmationText: "Открыть двери?",
				},
			}},
		},
	}
}

func entryContentTargetForTest() domain.TerminalTarget {
	return domain.TerminalTarget{
		TerminalID: "terminal-reactor", TerminalName: "Reactor",
		Tree: domain.ContentNode{
			ID: "root", Type: domain.NodeFolder, Name: "ROOT",
			Children: []domain.ContentNode{
				{
					ID: "reactor-status", Type: domain.NodeEntry, Name: "REACTOR STATUS",
					Blocks: []domain.EntryContentBlock{
						{ID: "power", InitialText: "POWER: OFFLINE"},
						{ID: "cooling", InitialText: "COOLING: OFFLINE"},
						{ID: "status", InitialText: "STATUS: NOMINAL"},
					},
				},
				{
					ID: "legacy-status", Type: domain.NodeEntry, Name: "LEGACY STATUS", Description: "UNCHANGED",
				},
				{
					ID: "restore-power", Type: domain.NodeCommand, Name: "RESTORE POWER", Text: "Power restored.",
					StateChange: &domain.StateChangeConfig{
						CompletedName: "POWER RESTORED", ConfirmationText: "Restore power?",
						EntryContentChange: &domain.EntryContentChange{BlockID: "power", CompletedText: "POWER: ONLINE"},
					},
				},
				{
					ID: "restore-cooling", Type: domain.NodeCommand, Name: "RESTORE COOLING", Text: "Cooling restored.",
					StateChange: &domain.StateChangeConfig{
						CompletedName: "COOLING RESTORED", ConfirmationText: "Restore cooling?",
						EntryContentChange: &domain.EntryContentChange{BlockID: "cooling", CompletedText: "COOLING: ONLINE"},
					},
				},
				{
					ID: "restore-status", Type: domain.NodeCommand, Name: "RESTORE STATUS", Text: "Status restored.",
					StateChange: &domain.StateChangeConfig{
						CompletedName: "STATUS RESTORED", ConfirmationText: "Restore status?",
						EntryContentChange: &domain.EntryContentChange{BlockID: "status", CompletedText: "STATUS: RESTORED"},
					},
				},
			},
		},
	}
}

func entryContentStateForTest(blockID, completedText string) domain.CommandExecutionState {
	return domain.CommandExecutionState{
		CompletedName: "COMPLETED",
		ResultText:    "DONE",
		EntryContentChange: &domain.EntryContentChange{
			BlockID: blockID, CompletedText: completedText,
		},
	}
}

func findContentNode(root domain.ContentNode, nodeID string) *domain.ContentNode {
	if root.ID == nodeID {
		return &root
	}
	for _, child := range root.Children {
		if found := findContentNode(child, nodeID); found != nil {
			return found
		}
	}
	return nil
}

func cloneTerminalRuntimeForTest(runtime *domain.TerminalRuntime) *domain.TerminalRuntime {
	if runtime == nil {
		return nil
	}
	clone := *runtime
	clone.Tree = domain.CloneContentNode(runtime.Tree)
	clone.Nav = cloneNav(runtime.Nav)
	if runtime.CommandExecution != nil {
		presentation := *runtime.CommandExecution
		clone.CommandExecution = &presentation
	}
	if runtime.CommandStates != nil {
		clone.CommandStates = make(map[string]domain.CommandExecutionState, len(runtime.CommandStates))
		maps.Copy(clone.CommandStates, runtime.CommandStates)
	}
	return &clone
}
