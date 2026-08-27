package hack

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

func TestGenerateBoardUsesLevelRules(t *testing.T) {
	tests := []struct {
		level      int
		wordLength int
		wordCount  int
	}{
		{level: 1, wordLength: 4, wordCount: 12},
		{level: 2, wordLength: 5, wordCount: 13},
		{level: 3, wordLength: 6, wordCount: 14},
		{level: 4, wordLength: 7, wordCount: 15},
		{level: 5, wordLength: 8, wordCount: 16},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("level_%d", test.level), func(t *testing.T) {
			words := &recordingWordSource{}
			hack := GenerateBoard("generation-level", test.level, newSequenceRandom(), words)

			if hack == nil {
				t.Fatal("GenerateBoard() returned nil")
			}
			if hack.Level != test.level || hack.WordLength != test.wordLength {
				t.Fatalf("level metadata = (%d, %d), want (%d, %d)", hack.Level, hack.WordLength, test.level, test.wordLength)
			}
			if words.length != test.wordLength || words.count != test.wordCount {
				t.Fatalf("PickWords() request = (%d, %d), want (%d, %d)", words.length, words.count, test.wordLength, test.wordCount)
			}
			if hack.AttemptsMax != 4 || hack.AttemptsLeft != 4 {
				t.Fatalf("attempts = %d/%d, want 4/4", hack.AttemptsLeft, hack.AttemptsMax)
			}
			if hack.Solved || hack.Failed || len(hack.UsedPatterns) != 0 || len(hack.Log) != 0 {
				t.Fatalf("new puzzle has progressed state: %#v", hack)
			}
			if len(hack.WordsByID) != test.wordCount {
				t.Fatalf("private lookup has %d entries, want %d candidates", len(hack.WordsByID), test.wordCount)
			}

			candidateCount := 0
			for _, candidate := range hack.WordsByID {
				candidateCount++
				if len(candidate.Text) != test.wordLength {
					t.Errorf("candidate %q has length %d, want %d", candidate.Text, len(candidate.Text), test.wordLength)
				}
			}
			if candidateCount != test.wordCount || candidateCount < 12 || candidateCount > 16 {
				t.Errorf("candidate count = %d, want %d in range 12..16", candidateCount, test.wordCount)
			}
			if !containsCandidate(hack, hack.SecretWord) {
				t.Errorf("secret word %q is not a visible candidate", hack.SecretWord)
			}

			if len(hack.Columns) != 2 {
				t.Fatalf("column count = %d, want 2", len(hack.Columns))
			}
			for columnIndex, column := range hack.Columns {
				if len(column.Text) != 16*12 {
					t.Errorf("column %d text length = %d, want 192", columnIndex, len(column.Text))
				}
				if len(column.Addresses) != 16 {
					t.Errorf("column %d address count = %d, want 16", columnIndex, len(column.Addresses))
				}
				for _, address := range column.Addresses {
					if len(address) != 6 || !strings.HasPrefix(address, "0x") {
						t.Errorf("column %d contains malformed address %q", columnIndex, address)
					}
				}
			}
		})
	}
}

func TestApplyGuessTransitions(t *testing.T) {
	t.Run("matching candidate solves without spending attempt", func(t *testing.T) {
		hack := testHackState()

		ApplyGuess(hack, "A1")

		if !hack.Solved || hack.Failed {
			t.Fatalf("result solved=%t failed=%t, want solved only", hack.Solved, hack.Failed)
		}
		if hack.AttemptsLeft != 4 {
			t.Fatalf("attempts left = %d, want 4", hack.AttemptsLeft)
		}
		assertLogContains(t, hack.Log, "> CODE", "> Точно!")
	})

	t.Run("wrong candidate reports likeness and spends attempt", func(t *testing.T) {
		hack := testHackState()

		ApplyGuess(hack, "A2")

		if hack.Solved || hack.Failed {
			t.Fatalf("result solved=%t failed=%t, want unfinished", hack.Solved, hack.Failed)
		}
		if hack.AttemptsLeft != 3 {
			t.Fatalf("attempts left = %d, want 3", hack.AttemptsLeft)
		}
		assertLogContains(t, hack.Log, "> CAVE", "> Отказ в доступе", "> 2/4 правильно.")
	})

	t.Run("filler clicks exhaust four attempts", func(t *testing.T) {
		hack := testHackState()

		for range 4 {
			ApplyGuess(hack, "0:4")
		}

		if !hack.Failed || hack.Solved {
			t.Fatalf("result solved=%t failed=%t, want failed only", hack.Solved, hack.Failed)
		}
		if hack.AttemptsLeft != 0 {
			t.Fatalf("attempts left = %d, want 0", hack.AttemptsLeft)
		}
		assertLogContains(t, hack.Log, "> !", "> 0/4 правильно.")
	})

	t.Run("unknown malformed out-of-range and in-word targets are ignored", func(t *testing.T) {
		targets := []string{"missing", "not:a-position", "2:0", "0:999", "0:0"}
		for _, target := range targets {
			t.Run(target, func(t *testing.T) {
				hack := testHackState()
				before := cloneHackState(t, hack)

				ApplyGuess(hack, target)

				if !reflect.DeepEqual(hack, before) {
					t.Fatalf("target %q mutated state\ngot:  %#v\nwant: %#v", target, hack, before)
				}
			})
		}
	})

	t.Run("terminal states ignore further guesses", func(t *testing.T) {
		for _, terminalState := range []struct {
			name   string
			mutate func(*domain.HackState)
		}{
			{name: "solved", mutate: func(h *domain.HackState) { h.Solved = true }},
			{name: "failed", mutate: func(h *domain.HackState) { h.Failed = true; h.AttemptsLeft = 0 }},
		} {
			t.Run(terminalState.name, func(t *testing.T) {
				hack := testHackState()
				terminalState.mutate(hack)
				before := cloneHackState(t, hack)

				ApplyGuess(hack, "A2")

				if !reflect.DeepEqual(hack, before) {
					t.Fatalf("terminal puzzle mutated\ngot:  %#v\nwant: %#v", hack, before)
				}
			})
		}
	})
}

func TestGeneratedBoardsAtEveryDifficultyContainThreeThroughSixValidPatterns(t *testing.T) {
	configuredLevels := []int{1, 2, 3, 4, 5}
	const boardsPerLevel = 200
	seenPairs := map[string]bool{}
	generated := 0
	for _, level := range configuredLevels {
		for iteration := range boardsPerLevel {
			generationID := fmt.Sprintf("generation-%d-%d", level, iteration)
			state := GenerateBoard(generationID, level, newSequenceRandom(), &recordingWordSource{})
			if state == nil {
				t.Fatalf("level %d iteration %d returned nil", level, iteration)
			}
			patterns := discoverPatternSpans(state.GenerationID, state.Columns)
			if len(patterns) < 3 || len(patterns) > 6 {
				t.Fatalf("level %d iteration %d patterns = %d, want 3..6", level, iteration, len(patterns))
			}
			camouflage := classifyFinalBoard(state)
			if !camouflage.publishable() {
				t.Fatalf("level %d iteration %d failed final-board camouflage: %#v", level, iteration, camouflage)
			}
			if len(camouflage.standaloneDelimiters) < len(patterns) {
				t.Fatalf("level %d iteration %d standalone delimiters = %d, want at least %d", level, iteration, len(camouflage.standaloneDelimiters), len(patterns))
			}
			if !camouflage.hasNonEmptyPattern || !camouflage.hasAlphabeticInterruptedSpan {
				t.Fatalf("level %d iteration %d missing non-empty pattern or alphabetic interruption: %#v", level, iteration, camouflage)
			}
			for category, rows := range map[string]map[int]struct{}{
				"candidate": camouflage.candidateRows,
				"pattern":   camouflage.patternRows,
				"decoy":     camouflage.decoyRows,
				"filler":    camouflage.ordinaryFillerRows,
			} {
				if len(rows) < 2 {
					t.Fatalf("level %d iteration %d %s rows = %v, want at least two", level, iteration, category, rows)
				}
			}
			if !rowIntervalsOverlapPairwise(camouflage.candidateRows, camouflage.patternRows, camouflage.decoyRows) {
				t.Fatalf("level %d iteration %d occupied-row intervals do not overlap: candidate=%v pattern=%v decoy=%v", level, iteration, camouflage.candidateRows, camouflage.patternRows, camouflage.decoyRows)
			}
			public := PublicState(state)
			if len(public.Patterns) != len(patterns) {
				t.Fatalf("level %d iteration %d public patterns = %d, production discovery = %d", level, iteration, len(public.Patterns), len(patterns))
			}
			for index := range patterns {
				if public.Patterns[index].ID != patternID(patterns[index].Identity) {
					t.Fatalf("level %d iteration %d public pattern %d does not match production discovery", level, iteration, index)
				}
			}
			generated++
			for _, pattern := range patterns {
				seenPairs[pattern.Pair] = true
				if pattern.Identity.GenerationID != generationID {
					t.Fatalf("pattern generation = %q, want %q", pattern.Identity.GenerationID, generationID)
				}
				text := state.Columns[pattern.ColumnIndex].Text[pattern.AbsoluteStart : pattern.AbsoluteEnd+1]
				if strings.IndexFunc(text[1:len(text)-1], isASCIIAlpha) >= 0 {
					t.Fatalf("pattern %#v contains alphabetic interior %q", pattern, text)
				}
			}
		}
	}
	wantGenerated := len(configuredLevels) * boardsPerLevel
	if generated != wantGenerated {
		t.Fatalf("generated boards = %d, want exactly %d across all configured difficulties", generated, wantGenerated)
	}
	for _, pair := range []string{"()", "[]", "{}", "<>"} {
		if !seenPairs[pair] {
			t.Errorf("generated boards never exposed pair %s", pair)
		}
	}
}

func TestPatternDiscoveryRetainsEmptyNonEmptyAndFirstCloserRules(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantSpans [][2]int
	}{
		{name: "adjacent empty", text: "()!!!!!!!!!!", wantSpans: [][2]int{{0, 1}}},
		{name: "non-empty punctuation", text: "[!%]!!!!!!!!", wantSpans: [][2]int{{0, 3}}},
		{name: "unmatched", text: "(!!!!!!!!!!!", wantSpans: nil},
		{name: "mismatched", text: "[!)!!!!!!!!!", wantSpans: nil},
		{name: "alphabetic interior", text: "{AB}!!!!!!!!", wantSpans: nil},
		{name: "first compatible closer", text: "<!!>>!!!!!!!", wantSpans: [][2]int{{0, 3}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			patterns := discoverPatternSpans("generation-fixture", []domain.HackColumn{{Text: test.text}})
			got := make([][2]int, len(patterns))
			for index, pattern := range patterns {
				got[index] = [2]int{pattern.Identity.Start, pattern.Identity.End}
			}
			if len(got) != len(test.wantSpans) || len(got) > 0 && !reflect.DeepEqual(got, test.wantSpans) {
				t.Fatalf("discoverPatternSpans(%q) = %v, want %v", test.text, got, test.wantSpans)
			}
		})
	}
}

func TestFinalBoardClassificationCountsAccidentalPatternsAndOnlyStandaloneDecoys(t *testing.T) {
	state := &domain.HackState{
		GenerationID: "generation-accidental",
		Columns: []domain.HackColumn{{
			Text:  "([])!]!!!!!!",
			Words: []domain.HackWord{},
		}},
	}

	camouflage := classifyFinalBoard(state)
	if len(camouflage.patterns) != 2 {
		t.Fatalf("accidental patterns = %#v, want both outer and inner discoveries", camouflage.patterns)
	}
	if len(camouflage.standaloneDelimiters) != 1 {
		t.Fatalf("standalone delimiters = %#v, want only the delimiter outside both valid ranges", camouflage.standaloneDelimiters)
	}
	if camouflage.standaloneDelimiters[0].offset != 5 {
		t.Fatalf("standalone delimiter offset = %d, want 5", camouflage.standaloneDelimiters[0].offset)
	}
}

func TestDelimiterAndInterruptedSpanTargetsUseOrdinaryGuessRules(t *testing.T) {
	t.Run("invalid delimiter categories use ordinary filler behavior", func(t *testing.T) {
		for _, target := range []struct {
			name  string
			text  string
			id    string
			glyph string
		}{
			{name: "standalone closer", text: "]!!!!!!!!!!!", id: "1:0", glyph: "]"},
			{name: "unmatched opening", text: "(!!!!!!!!!!!", id: "1:0", glyph: "("},
			{name: "mismatched opening", text: "[)!!!!!!!!!!", id: "1:0", glyph: "["},
			{name: "mismatched closer", text: "[)!!!!!!!!!!", id: "1:1", glyph: ")"},
			{name: "later compatible closer", text: "<!!>>!!!!!!!", id: "1:4", glyph: ">"},
		} {
			t.Run(target.name, func(t *testing.T) {
				state := testHackState()
				state.Columns[1].Text = target.text

				ApplyGuess(state, target.id)

				if state.AttemptsLeft != 3 || state.Solved || state.Failed {
					t.Fatalf("target %s did not retain ordinary filler behavior: %#v", target.id, state)
				}
				assertLogContains(t, state.Log, "> "+target.glyph, "> Отказ в доступе", "> 0/4 правильно.")
			})
		}
	})

	t.Run("non-opening cells in a valid span remain individual filler targets", func(t *testing.T) {
		for _, target := range []struct {
			name  string
			id    string
			glyph string
		}{
			{name: "interior", id: "0:1", glyph: "!"},
			{name: "closing delimiter", id: "0:3", glyph: ")"},
		} {
			t.Run(target.name, func(t *testing.T) {
				state := patternTestState()

				ApplyGuess(state, target.id)

				if state.AttemptsLeft != 3 || state.Solved || state.Failed {
					t.Fatalf("target %s did not retain ordinary filler behavior: %#v", target.id, state)
				}
				assertLogContains(t, state.Log, "> "+target.glyph, "> Отказ в доступе", "> 0/4 правильно.")
			})
		}
	})

	t.Run("current pattern opening remains reserved", func(t *testing.T) {
		state := patternTestState()
		before := cloneHackState(t, state)

		ApplyGuess(state, "0:0")

		if !reflect.DeepEqual(state, before) {
			t.Fatalf("pattern opening fell through to ordinary filler behavior\ngot:  %#v\nwant: %#v", state, before)
		}
	})

	t.Run("candidate inside alphabetic interrupted span uses ordinary guess rules", func(t *testing.T) {
		state := testHackState()
		state.Columns[0].Text = "(CODE)!CAVE!DUST!IRON!"
		state.Columns[0].Words[0].Start = 1
		state.WordsByID["A1"] = domain.HackCandidate{Text: "CAVE"}
		state.SecretWord = "IRON"

		ApplyGuess(state, "A1")

		if state.AttemptsLeft != 3 || state.Solved || state.Failed {
			t.Fatalf("interrupted-span candidate did not retain ordinary guess behavior: %#v", state)
		}
		assertLogContains(t, state.Log, "> CAVE", "> Отказ в доступе")
	})
}

func TestApplyPatternUsesExactOutcomeBuckets(t *testing.T) {
	dudRemovals := 0
	restores := 0
	for roll := range 100 {
		state := patternTestState()
		state.AttemptsLeft = 1
		beforeCandidates := len(state.WordsByID)
		random := &recordingRandom{values: []int{roll, 0}}
		if !ApplyPattern(state, firstPatternID(t, state), random) {
			t.Fatalf("roll %d rejected valid pattern", roll)
		}
		if random.calls == 0 || random.limits[0] != 100 {
			t.Fatalf("roll %d first RNG call = %v, want outcome Intn(100)", roll, random.limits)
		}
		if len(state.WordsByID) == beforeCandidates-1 {
			dudRemovals++
			if !containsCandidate(state, state.SecretWord) || state.AttemptsLeft != 1 {
				t.Fatalf("roll %d corrupted dud-removal state: %#v", roll, state)
			}
		} else if state.AttemptsLeft == state.AttemptsMax {
			restores++
		} else {
			t.Fatalf("roll %d produced no defined effect: %#v", roll, state)
		}
	}
	if dudRemovals != 80 || restores != 20 {
		t.Fatalf("outcomes = %d dud removals/%d restores, want 80/20", dudRemovals, restores)
	}
}

func TestApplyPatternIsOneUseAndRestoresWhenNoDudRemains(t *testing.T) {
	state := patternTestState()
	state.AttemptsLeft = 2
	delete(state.WordsByID, "A2")
	state.Columns[0].Words = state.Columns[0].Words[:1]
	state.Columns[0].Text = state.Columns[0].Text[:9] + "...."
	patternID := firstPatternID(t, state)
	random := &recordingRandom{values: []int{0}}

	if !ApplyPattern(state, patternID, random) {
		t.Fatal("valid pattern was rejected")
	}
	if random.calls != 1 || !reflect.DeepEqual(random.limits, []int{100}) {
		t.Fatalf("no-dud fallback RNG calls = %v, want one outcome draw", random.limits)
	}
	if state.AttemptsLeft != state.AttemptsMax {
		t.Fatalf("attempts = %d, want restored to %d", state.AttemptsLeft, state.AttemptsMax)
	}
	after := cloneHackState(t, state)
	rejectedRandom := &recordingRandom{values: []int{0}}
	if ApplyPattern(state, patternID, rejectedRandom) {
		t.Fatal("used pattern was accepted twice")
	}
	if rejectedRandom.calls != 0 {
		t.Fatalf("used pattern consumed %d RNG values, want zero", rejectedRandom.calls)
	}
	if !reflect.DeepEqual(state, after) {
		t.Fatalf("repeated pattern mutated state\ngot: %#v\nwant: %#v", state, after)
	}
}

func TestPatternDiscoveryHandlesStackedAndInvalidSpans(t *testing.T) {
	stacked := discoverPatternSpans("generation-stacked", []domain.HackColumn{{Text: "((!!)", Words: []domain.HackWord{}}})
	wantIDs := []string{
		patternID(domain.HackPatternIdentity{GenerationID: "generation-stacked", Row: 0, Start: 0, End: 4}),
		patternID(domain.HackPatternIdentity{GenerationID: "generation-stacked", Row: 0, Start: 1, End: 4}),
	}
	if got := patternIDs(stacked); !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("stacked pattern IDs = %v, want %v", got, wantIDs)
	}
	if stacked[0].Identity.End != stacked[1].Identity.End || stacked[0].Identity.Start == stacked[1].Identity.Start {
		t.Fatalf("shared closer patterns are not distinct complete coordinate pairs: %#v", stacked)
	}
	firstClose := discoverPatternSpans("generation-first-close", []domain.HackColumn{{Text: "(!!))"}})
	if len(firstClose) != 1 || firstClose[0].Identity.End != 3 {
		t.Fatalf("first-compatible-close discovery = %#v, want one span ending at 3", firstClose)
	}

	invalid := []struct {
		name string
		text string
	}{
		{name: "alphabetic interior", text: "(A)"},
		{name: "mismatched closer", text: "[)"},
		{name: "cross row", text: "!!!!!!!!!!!()"},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if patterns := discoverPatternSpans("generation-invalid", []domain.HackColumn{{Text: test.text}}); len(patterns) != 0 {
				t.Fatalf("discoverPatternSpans(%q) = %#v, want none", test.text, patterns)
			}
		})
	}
}

func TestChangedCloserCreatesNewIdentityAndUsedPairStaysUnavailable(t *testing.T) {
	state := &domain.HackState{
		GenerationID: "generation-changing-closer",
		Level:        1,
		WordLength:   4,
		AttemptsMax:  4,
		AttemptsLeft: 2,
		SecretWord:   "CODE",
		WordsByID:    map[string]domain.HackCandidate{"A1": {Text: "CODE"}},
		UsedPatterns: map[domain.HackPatternIdentity]struct{}{},
		Columns:      []domain.HackColumn{{Text: "(!)!"}},
	}

	oldIdentity := discoverPatternSpans(state.GenerationID, state.Columns)[0].Identity
	oldID := patternID(oldIdentity)
	if !ApplyPattern(state, oldID, &constantRandom{value: 99}) {
		t.Fatal("initial coordinate pair was rejected")
	}

	state.Columns[0].Text = "(!!)"
	changed := discoverPatternSpans(state.GenerationID, state.Columns)
	if len(changed) != 1 || changed[0].Identity.End == oldIdentity.End {
		t.Fatalf("changed closer discovery = %#v, want a new coordinate pair", changed)
	}
	changedID := patternID(changed[0].Identity)
	if !ApplyPattern(state, changedID, &constantRandom{value: 99}) {
		t.Fatal("new coordinate pair was not independently available")
	}

	state.Columns[0].Text = "(!)!"
	public := PublicState(state)
	if len(public.Patterns) != 1 || public.Patterns[0].ID != oldID || !public.Patterns[0].Used {
		t.Fatalf("rediscovered old coordinate pair = %#v, want permanently used", public.Patterns)
	}
	random := &recordingRandom{values: []int{0}}
	if ApplyPattern(state, oldID, random) || random.calls != 0 {
		t.Fatalf("rediscovered used pair accepted or consumed RNG: calls=%d", random.calls)
	}
}

func TestDudRemovalRevealsDynamicPatternImmediately(t *testing.T) {
	state := &domain.HackState{
		GenerationID: "generation-dynamic",
		Level:        1, WordLength: 4, AttemptsMax: 4, AttemptsLeft: 4, SecretWord: "CODE",
		WordsByID:    map[string]domain.HackCandidate{"A1": {Text: "DUST"}, "B1": {Text: "CODE"}},
		UsedPatterns: map[domain.HackPatternIdentity]struct{}{},
		Columns: []domain.HackColumn{
			{Text: "(DUST)!!!!!!", Words: []domain.HackWord{{ID: "A1", Start: 1, Length: 4}}},
			{Text: "[]CODE!!!!!!", Words: []domain.HackWord{{ID: "B1", Start: 2, Length: 4}}},
		},
	}
	initial := discoverPatternSpans(state.GenerationID, state.Columns)
	if len(initial) != 1 || initial[0].Identity.Row != 1 || initial[0].Identity.Start != 0 || initial[0].Identity.End != 1 {
		t.Fatalf("initial patterns = %#v", initial)
	}
	if !ApplyPattern(state, patternID(initial[0].Identity), &constantRandom{value: 0}) {
		t.Fatal("available pattern was rejected")
	}
	postDud := discoverPatternSpans(state.GenerationID, state.Columns)
	if len(postDud) != 2 || postDud[0].Identity.Row != 0 || postDud[0].Identity.End != 5 || postDud[1].Identity.Row != 1 {
		t.Fatalf("post-dud patterns = %#v, want dynamic and used spans", postDud)
	}
	public := PublicState(state)
	if public == nil || len(public.Patterns) != 2 || public.Patterns[0].Used || !public.Patterns[1].Used {
		t.Fatalf("public dynamic/used pattern state = %#v", public)
	}
}

func TestGeneratedBoardHasNoPlayerAdministratorEntry(t *testing.T) {
	hack := GenerateBoard("generation-no-admin", 1, newSequenceRandom(), &recordingWordSource{})
	if hack == nil {
		t.Fatal("GenerateBoard() returned nil")
	}
	for id, candidate := range hack.WordsByID {
		if candidate.Text == "SUCCESS" {
			t.Fatalf("private lookup retained administrator candidate %q", id)
		}
	}
	for _, column := range hack.Columns {
		if strings.Contains(column.Text, "SUCCESS") {
			t.Fatalf("public board retained administrator entry: %q", column.Text)
		}
	}
}

func TestForceSuccessOnlyCompletesEligiblePuzzle(t *testing.T) {
	t.Run("active puzzle", func(t *testing.T) {
		hack := testHackState()

		ForceSuccess(hack)

		if !hack.Solved || hack.Failed || hack.AttemptsLeft != 4 {
			t.Fatalf("forced puzzle state = %#v", hack)
		}
		assertLogContains(t, hack.Log, "> CODE", "> Точно!")
	})

	t.Run("nil and terminal puzzles are no-op", func(t *testing.T) {
		ForceSuccess(nil)

		for _, terminalState := range []struct {
			name   string
			mutate func(*domain.HackState)
		}{
			{name: "solved", mutate: func(h *domain.HackState) { h.Solved = true }},
			{name: "failed", mutate: func(h *domain.HackState) { h.Failed = true; h.AttemptsLeft = 0 }},
		} {
			t.Run(terminalState.name, func(t *testing.T) {
				hack := testHackState()
				terminalState.mutate(hack)
				before := cloneHackState(t, hack)

				ForceSuccess(hack)

				if !reflect.DeepEqual(hack, before) {
					t.Fatalf("terminal puzzle mutated\ngot:  %#v\nwant: %#v", hack, before)
				}
			})
		}
	})
}

func TestPublicStateExcludesPrivatePuzzleFields(t *testing.T) {
	if got := PublicState(nil); got != nil {
		t.Fatalf("PublicState(nil) = %#v, want nil", got)
	}

	hack := patternTestState()
	public := PublicState(hack)
	if public == nil {
		t.Fatal("PublicState() returned nil")
	}
	if public.Level != hack.Level || public.WordLength != hack.WordLength || public.AttemptsLeft != hack.AttemptsLeft {
		t.Fatalf("public state dropped gameplay fields: %#v", public)
	}

	raw, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secretWord", "wordsById"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("public JSON contains private value %q: %s", secret, raw)
		}
	}
	before := cloneHackState(t, hack)
	if len(public.Patterns) == 0 {
		t.Fatal("public state omitted the current valid pattern")
	}
	public.Patterns[0].ID = "mutated"
	public.Patterns[0].Used = true
	public.Columns[0].Text = "mutated"
	public.Columns[0].Words[0].ID = "mutated"
	if !reflect.DeepEqual(hack, before) {
		t.Fatalf("mutating public projection changed canonical state\ngot: %#v\nwant: %#v", hack, before)
	}
}

func TestRejectedPatternsDoNotConsumeRandomness(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*domain.HackState) string
	}{
		{name: "invalid", mutate: func(*domain.HackState) string { return "not-a-server-pattern" }},
		{name: "terminal", mutate: func(state *domain.HackState) string {
			state.Solved = true
			return firstPatternID(t, state)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := patternTestState()
			random := &recordingRandom{values: []int{0}}
			if ApplyPattern(state, test.mutate(state), random) {
				t.Fatal("rejected pattern was accepted")
			}
			if random.calls != 0 {
				t.Fatalf("rejected pattern consumed %d RNG values, want zero", random.calls)
			}
		})
	}
}

type sequenceRandom struct {
	next int
}

func newSequenceRandom() *sequenceRandom {
	return &sequenceRandom{next: 11}
}

func (r *sequenceRandom) Intn(limit int) int {
	value := r.next % limit
	r.next += 37
	return value
}

type constantRandom struct {
	value int
}

type recordingRandom struct {
	values []int
	calls  int
	limits []int
}

func (r *recordingRandom) Intn(limit int) int {
	r.limits = append(r.limits, limit)
	value := 0
	if r.calls < len(r.values) {
		value = r.values[r.calls]
	}
	r.calls++
	return value % limit
}

func (r *constantRandom) Intn(limit int) int {
	return r.value % limit
}

type recordingWordSource struct {
	length int
	count  int
}

func (s *recordingWordSource) PickWords(length, count int) []string {
	s.length = length
	s.count = count
	pool := wordsByLength[length]
	return append([]string(nil), pool[:count]...)
}

var wordsByLength = map[int][]string{
	4: {"RUIN", "PALM", "IRON", "GATE", "BOLT", "RAMP", "CORE", "DUST", "FUSE", "GRID", "LAMP", "MASK", "NODE", "PIPE", "RING", "RUST"},
	5: {"ALLOY", "ARMOR", "ATLAS", "BASIN", "BLAST", "BRICK", "CABLE", "CACHE", "CARGO", "CLIFF", "CLOCK", "CRANE", "CRATE", "CREEK", "DRAIN", "DRONE"},
	6: {"ANCHOR", "BASALT", "BEACON", "BUNKER", "CAVERN", "CIPHER", "CONVOY", "COURSE", "DEBRIS", "ENGINE", "FILTER", "FLIGHT", "FUNGUS", "GARDEN", "GIRDER", "HARBOR"},
	7: {"ANDROID", "ARCHIVE", "ARSENAL", "ARTICLE", "BATTERY", "BEDROCK", "BOMBARD", "BREAKER", "CAPSULE", "CHAMBER", "CIRCUIT", "COOLANT", "CORRODE", "CRUMBLE", "CRYSTAL", "DOSSIER"},
	8: {"CONCRETE", "DISTANCE", "ELECTRIC", "CHEMICAL", "GENERATE", "HOSPITAL", "INDUSTRY", "JUNCTION", "KEYSTONE", "LOCATION", "MOUNTAIN", "NAVIGATE", "OVERLOAD", "PIPELINE", "QUANTITY", "RADIATOR"},
}

func testHackState() *domain.HackState {
	return &domain.HackState{
		GenerationID: "generation-guess",
		Level:        1,
		WordLength:   4,
		AttemptsMax:  4,
		AttemptsLeft: 4,
		SecretWord:   "CODE",
		WordsByID: map[string]domain.HackCandidate{
			"A1": {Text: "CODE"},
			"A2": {Text: "CAVE"},
			"A3": {Text: "DUST"},
			"A4": {Text: "IRON"},
		},
		UsedPatterns: map[domain.HackPatternIdentity]struct{}{},
		Columns: []domain.HackColumn{
			{
				Addresses: []string{"0xC000"},
				Text:      "CODE!CAVE!DUST!IRON!",
				Words: []domain.HackWord{
					{ID: "A1", Start: 0, Length: 4},
					{ID: "A2", Start: 5, Length: 4},
					{ID: "A3", Start: 10, Length: 4},
					{ID: "A4", Start: 15, Length: 4},
				},
			},
			{Addresses: []string{"0xD000"}, Text: "!!!!!!!!!!!!", Words: []domain.HackWord{}},
		},
	}
}

func patternTestState() *domain.HackState {
	return &domain.HackState{
		GenerationID: "generation-pattern",
		Level:        1,
		WordLength:   4,
		AttemptsMax:  4,
		AttemptsLeft: 4,
		SecretWord:   "CODE",
		WordsByID:    map[string]domain.HackCandidate{"A1": {Text: "CODE"}, "A2": {Text: "DUST"}},
		UsedPatterns: map[domain.HackPatternIdentity]struct{}{},
		Log:          []string{},
		Columns: []domain.HackColumn{
			{
				Addresses: []string{"0xC000"},
				Text:      "(!!)CODE!DUST",
				Words: []domain.HackWord{
					{ID: "A1", Start: 4, Length: 4},
					{ID: "A2", Start: 9, Length: 4},
				},
			},
			{Addresses: []string{"0xD000"}, Text: "!!!!!!!!!!!!", Words: []domain.HackWord{}},
		},
	}
}

func cloneHackState(t *testing.T, source *domain.HackState) *domain.HackState {
	t.Helper()
	clone := *source
	if source.Log != nil {
		clone.Log = append([]string{}, source.Log...)
	}
	clone.WordsByID = make(map[string]domain.HackCandidate, len(source.WordsByID))
	maps.Copy(clone.WordsByID, source.WordsByID)
	clone.UsedPatterns = make(map[domain.HackPatternIdentity]struct{}, len(source.UsedPatterns))
	for id := range source.UsedPatterns {
		clone.UsedPatterns[id] = struct{}{}
	}
	clone.Columns = make([]domain.HackColumn, len(source.Columns))
	for i, column := range source.Columns {
		clone.Columns[i] = column
		clone.Columns[i].Addresses = append([]string(nil), column.Addresses...)
		if column.Words != nil {
			clone.Columns[i].Words = append([]domain.HackWord{}, column.Words...)
		}
	}
	return &clone
}

func patternIDs(patterns []domain.HackPattern) []string {
	ids := make([]string, len(patterns))
	for index, pattern := range patterns {
		ids[index] = patternID(pattern.Identity)
	}
	return ids
}

func firstPatternID(t *testing.T, state *domain.HackState) string {
	t.Helper()
	patterns := discoverPatternSpans(state.GenerationID, state.Columns)
	if len(patterns) == 0 {
		t.Fatal("test state has no valid pattern")
	}
	return patternID(patterns[0].Identity)
}

func containsCandidate(hack *domain.HackState, text string) bool {
	for _, candidate := range hack.WordsByID {
		if candidate.Text == text {
			return true
		}
	}
	return false
}

func assertLogContains(t *testing.T, log []string, lines ...string) {
	t.Helper()
	for _, line := range lines {
		found := slices.Contains(log, line)
		if !found {
			t.Errorf("log %q does not contain %q", log, line)
		}
	}
}
