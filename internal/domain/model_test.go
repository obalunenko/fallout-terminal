package domain

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeEncodeSessionV1Fixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../testutil/testdata/session-v1.json")
	require.NoError(t, err)

	session, err := DecodeSession(raw)
	require.NoError(t, err)
	assert.Equal(t, 1, session.Version)
	assert.NotEmpty(t, session.Terminals)

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	assert.True(t, bytes.HasSuffix(encoded, []byte("\n")))

	roundTrip, err := DecodeSession(encoded)
	require.NoError(t, err)
	assert.Equal(t, session, roundTrip)
}

func TestDecodeStateChangingSessionV1Fixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../testutil/testdata/session-v1-state-changing.json")
	require.NoError(t, err)
	session, err := DecodeSession(raw)
	require.NoError(t, err)
	assert.Equal(t, 1, session.Version)
	require.Len(t, session.Terminals, 2)
	security := session.Terminals[0]
	require.Len(t, security.CommandStates, 2)
	snapshot, ok := security.CommandStates["n_doors"]
	require.True(t, ok)
	assert.Equal(t, "Гермодвери открыты", snapshot.CompletedName)
	assert.NotEmpty(t, snapshot.ResultText)
	stateChange := security.Root.Children[0].Children[1].StateChange
	require.NotNil(t, stateChange)
	assert.Equal(t, "Тревога отключена", stateChange.CompletedName)
	power := session.Terminals[1]
	require.Len(t, power.CommandStates, 1)
	_, ok = power.CommandStates["n_backup_power"]
	require.True(t, ok)

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	roundTrip, err := DecodeSession(encoded)
	require.NoError(t, err)
	assert.Equal(t, session, roundTrip)
}

func TestUnknownFieldsRoundTrip(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "version": 1,
  "name": "Extras",
  "campaignNote": {"keep": true},
  "terminals": [{
    "id": "t1",
    "name": "Terminal",
    "hackLevel": 0,
    "introText": "",
    "terminalNote": 42,
    "root": {
      "id": "root",
      "type": "folder",
      "name": "ROOT",
      "nodeNote": [1, 2],
      "children": []
    }
  }]
}`)

	session, err := DecodeSession(raw)
	require.NoError(t, err)
	encoded, err := EncodeSession(session)
	require.NoError(t, err)

	for _, field := range []string{"campaignNote", "terminalNote", "nodeNote"} {
		assert.Contains(t, string(encoded), `"`+field+`"`)
	}
}

func TestLegacySessionNormalizesSingletonGroupsAndCloneDetachesOrder(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "version": 1,
  "name": "Legacy",
  "terminals": [
    {"id":"a","name":"Terminal","hackLevel":0,"introText":"","root":{"id":"root","type":"folder","name":"ROOT","children":[]}},
    {"id":"b","name":"Terminal","hackLevel":0,"introText":"","root":{"id":"root","type":"folder","name":"ROOT","children":[]}}
  ]
}`)

	session, err := DecodeSession(raw)
	require.NoError(t, err)
	require.Len(t, session.TerminalGroups, 2)
	assert.Equal(t, []string{"a"}, session.TerminalGroups[0].TerminalIDs)
	assert.Equal(t, []string{"b"}, session.TerminalGroups[1].TerminalIDs)
	assert.NotEqual(t, session.TerminalGroups[0].ID, session.TerminalGroups[1].ID)
	assert.NotEqual(t, session.TerminalGroups[0].Name, session.TerminalGroups[1].Name)

	clone := CloneSession(session)
	clone.TerminalGroups[0].TerminalIDs[0] = "changed"
	assert.Equal(t, "a", session.TerminalGroups[0].TerminalIDs[0])

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"terminalGroups"`)
}

func TestTerminalTransitionRoundTripTreatsConfigAsKnownAndDetachesClone(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "version": 1,
  "name": "Linked terminals",
  "terminals": [
    {"id":"a","name":"A","hackLevel":0,"introText":"","root":{"id":"root","type":"folder","name":"ROOT","children":[{"id":"go","type":"command","name":"GO","terminalTransition":{"targetTerminalId":"b"},"futureNode":true}]}},
    {"id":"b","name":"B","hackLevel":0,"introText":"","root":{"id":"root","type":"folder","name":"ROOT","children":[]}}
  ]
}`)
	session, err := DecodeSession(raw)
	require.NoError(t, err)
	transition := session.Terminals[0].Root.Children[0].TerminalTransition
	require.NotNil(t, transition)
	assert.Equal(t, "b", transition.TargetTerminalID)
	assert.NotContains(t, session.Terminals[0].Root.Children[0].Extra, "terminalTransition")

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"terminalTransition"`)
	assert.Contains(t, string(encoded), `"futureNode"`)

	clone := CloneSession(session)
	clone.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID = "changed"
	assert.Equal(t, "b", session.Terminals[0].Root.Children[0].TerminalTransition.TargetTerminalID)
}

func TestCloneContentNodeDeeplyDetachesNestedValues(t *testing.T) {
	t.Parallel()

	node := ContentNode{
		ID:    "root",
		Type:  NodeFolder,
		Name:  "ROOT",
		Extra: map[string]json.RawMessage{"future": json.RawMessage(`{"enabled":true}`)},
		Children: []ContentNode{{
			ID:                 "command",
			Type:               NodeCommand,
			StateChange:        &StateChangeConfig{CompletedName: "Done", ConfirmationText: "Proceed?"},
			TerminalTransition: &TerminalTransitionConfig{TargetTerminalID: "terminal-b"},
			Children:           []ContentNode{},
		}},
	}

	clone := CloneContentNode(node)
	clone.Extra["future"][0] = '['
	clone.Children[0].StateChange.CompletedName = "Changed"
	clone.Children[0].TerminalTransition.TargetTerminalID = "terminal-c"
	clone.Children = append(clone.Children, ContentNode{ID: "other"})

	assert.Equal(t, json.RawMessage(`{"enabled":true}`), node.Extra["future"])
	assert.Equal(t, "Done", node.Children[0].StateChange.CompletedName)
	assert.Equal(t, "terminal-b", node.Children[0].TerminalTransition.TargetTerminalID)
	assert.Len(t, node.Children, 1)
	assert.NotNil(t, clone.Children[0].Children)
	assert.Empty(t, clone.Children[0].Children)
	assert.Nil(t, CloneContentNode(ContentNode{}).Children)
}

func TestCommandBehaviorDiscriminatesOrdinaryStateChangeTransitionAndInvalid(t *testing.T) {
	t.Parallel()

	stateChange := &StateChangeConfig{CompletedName: "Done", ConfirmationText: "Proceed?"}
	transition := &TerminalTransitionConfig{TargetTerminalID: "b"}
	tests := []struct {
		name string
		node ContentNode
		want CommandBehavior
	}{
		{name: "ordinary", node: ContentNode{Type: NodeCommand}, want: CommandBehaviorOrdinary},
		{name: "state change", node: ContentNode{Type: NodeCommand, StateChange: stateChange}, want: CommandBehaviorStateChange},
		{name: "terminal transition", node: ContentNode{Type: NodeCommand, TerminalTransition: transition}, want: CommandBehaviorTerminalTransition},
		{name: "invalid dual config", node: ContentNode{Type: NodeCommand, StateChange: stateChange, TerminalTransition: transition}, want: CommandBehaviorInvalid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, test.node.Behavior())
		})
	}
}

func TestDecodeSessionRejectsMalformedDualCommandBehavior(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "version": 1,
  "name": "Invalid behavior",
  "terminals": [
    {"id":"a","name":"A","hackLevel":0,"introText":"","root":{"id":"root","type":"folder","name":"ROOT","children":[{"id":"go","type":"command","name":"GO","text":"Done","stateChange":{"completedName":"Done","confirmationText":"Proceed?"},"terminalTransition":{"targetTerminalId":"b"}}]}},
    {"id":"b","name":"B","hackLevel":0,"introText":"","root":{"id":"root","type":"folder","name":"ROOT","children":[]}}
  ]
}`)

	_, err := DecodeSession(raw)
	require.ErrorContains(t, err, "cannot contain both stateChange and terminalTransition")
}

func TestStateChangingCommandRoundTripPreservesFrozenSnapshotAndUnknownFields(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "version": 1,
  "name": "Stateful extras",
  "campaignNote": {"keep": true},
  "terminals": [{
    "id": "t_security",
    "name": "Security",
    "hackLevel": 0,
    "introText": "",
    "terminalNote": 42,
    "root": {
      "id": "root",
      "type": "folder",
      "name": "ROOT",
      "children": [{
        "id": "n_doors",
        "type": "command",
        "name": "Open doors",
        "text": "Doors opened.",
        "stateChange": {
          "completedName": "Doors open",
          "confirmationText": "Open the doors?"
        },
        "nodeNote": [1, 2]
      }]
    },
    "commandStates": {
      "n_doors": {
        "completedName": "Doors were opened",
        "resultText": "Access to the sector was granted."
      }
    }
  }]
}`)

	session, err := DecodeSession(raw)
	require.NoError(t, err)
	command := &session.Terminals[0].Root.Children[0]
	require.NotNil(t, command.StateChange)
	assert.Equal(t, StateChangeConfig{
		CompletedName:    "Doors open",
		ConfirmationText: "Open the doors?",
	}, *command.StateChange)
	snapshot, ok := session.Terminals[0].CommandStates["n_doors"]
	require.True(t, ok)
	assert.Equal(t, CommandExecutionState{
		CompletedName: "Doors were opened",
		ResultText:    "Access to the sector was granted.",
	}, snapshot)

	// The durable snapshot is a frozen record of the first successful execution,
	// not a view over the command's subsequently edited authored fields.
	command.StateChange.CompletedName = "New authored title"
	command.Text = "New authored result"
	assert.Equal(t, snapshot, session.Terminals[0].CommandStates["n_doors"])

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	for _, field := range []string{
		"stateChange", "completedName", "confirmationText", "commandStates", "resultText",
		"campaignNote", "terminalNote", "nodeNote",
	} {
		assert.Contains(t, string(encoded), `"`+field+`"`)
	}

	decoded, err := DecodeSession(encoded)
	require.NoError(t, err)
	gotSnapshot := decoded.Terminals[0].CommandStates["n_doors"]
	assert.Equal(t, snapshot, gotSnapshot)
}

func TestLegacyVersionOneSessionDefaultsToOrdinaryCommandsWithoutSnapshots(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "version": 1,
  "name": "Legacy",
  "terminals": [{
    "id": "t1",
    "name": "Terminal",
    "hackLevel": 0,
    "introText": "",
    "root": {
      "id": "root",
      "type": "folder",
      "name": "ROOT",
      "children": [{
        "id": "ordinary",
        "type": "command",
        "name": "Read status",
        "text": "All systems nominal."
      }]
    }
  }]
}`)

	session, err := DecodeSession(raw)
	require.NoError(t, err)
	assert.Nil(t, session.Terminals[0].Root.Children[0].StateChange)
	assert.Empty(t, session.Terminals[0].CommandStates)

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	for _, absent := range []string{`"stateChange"`, `"commandStates"`} {
		assert.NotContains(t, string(encoded), absent)
	}

	roundTrip, err := DecodeSession(encoded)
	require.NoError(t, err)
	assert.Equal(t, 1, roundTrip.Version)
	wantOrdinary := ContentNode{
		ID: "ordinary", Type: NodeCommand, Name: "Read status", Text: "All systems nominal.",
	}
	assert.Equal(t, wantOrdinary, roundTrip.Terminals[0].Root.Children[0])
}

func TestVersionOneSessionNeverPersistsRuntimeHackAggregate(t *testing.T) {
	t.Parallel()

	session := Session{
		Version: 1,
		Name:    "Runtime boundary",
		Terminals: []Terminal{{
			ID: "terminal-1", Name: "Overseer", HackLevel: 3, IntroText: "WELCOME",
			Root: ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}},
		}},
	}
	// This aggregate deliberately contains every category of process-local
	// hacking state. It is not part of Session or Terminal and therefore has no
	// path into the version-1 document.
	runtime := &HackState{
		GenerationID: "generation-runtime-only",
		Level:        3, WordLength: 6, AttemptsMax: 4, AttemptsLeft: 2,
		SecretWord: "CIPHER",
		WordsByID:  map[string]HackCandidate{"A1": {Text: "CIPHER"}, "A2": {Text: "BUNKER"}},
		UsedPatterns: map[HackPatternIdentity]struct{}{{
			GenerationID: "generation-runtime-only", Row: 4, Start: 2, End: 7,
		}: {}},
		Log:     []string{"Ложное слово удалено."},
		Columns: []HackColumn{{Text: "..CIPHER....", Words: []HackWord{{ID: "A1", Start: 2, Length: 6}}}},
	}
	require.NotEmpty(t, runtime.GenerationID)
	assert.NotEqual(t, runtime.AttemptsMax, runtime.AttemptsLeft)

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	for _, forbidden := range []string{
		"generationId", "generation-runtime-only", "patterns", "usedPatterns",
		"removedDuds", "attemptsMax", "attemptsLeft", "outcomes", "unlocked",
		"puzzleSeed", "secretWord", "wordsById", "CIPHER", "Ложное слово удалено.",
	} {
		assert.NotContains(t, string(encoded), forbidden)
	}

	decoded, err := DecodeSession(encoded)
	require.NoError(t, err)
	reencoded, err := EncodeSession(decoded)
	require.NoError(t, err)
	assert.Equal(t, encoded, reencoded)
}

func TestRuntimeCoordinationProjectionClonesDetachDeeply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		projection          any
		clone               func(any) any
		requiredFields      map[string]reflect.Kind
		forbiddenJSONFields []string
	}{
		{
			name:       "master coordination state",
			projection: &MasterCoordinationState{},
			clone: func(value any) any {
				return CloneMasterCoordinationState(value.(*MasterCoordinationState))
			},
			requiredFields: map[string]reflect.Kind{
				"Roster":        reflect.Slice,
				"Sessions":      reflect.Slice,
				"Broadcast":     reflect.Pointer,
				"PendingSwitch": reflect.Pointer,
			},
			forbiddenJSONFields: []string{
				"browserToken", "connectionId", "connectionIds", "requestResults",
				"secretWord", "wordsById", "usedPatterns", "hack",
			},
		},
		{
			name:       "personalized player state",
			projection: &PlayerState{},
			clone: func(value any) any {
				return ClonePlayerState(value.(*PlayerState))
			},
			requiredFields: map[string]reflect.Kind{
				"Character":        reflect.Pointer,
				"BroadcastID":      reflect.String,
				"ActiveTerminalID": reflect.String,
				"Roster":           reflect.Slice,
			},
			forbiddenJSONFields: []string{
				"browserToken", "sessions", "claimedBySessionId", "connected",
				"connectionId", "connectionIds", "requestResults", "pendingSwitch",
				"secretWord", "wordsById", "usedPatterns", "hack",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			projection := reflect.ValueOf(test.projection).Elem()
			for fieldName, wantKind := range test.requiredFields {
				field := projection.FieldByName(fieldName)
				require.True(t, field.IsValid(), "%T is missing required projection field %s", test.projection, fieldName)
				assert.Equal(t, wantKind, field.Kind(), "%T.%s", test.projection, fieldName)
			}

			seedProjection(t, projection, test.name, 0)
			before, err := json.Marshal(test.projection)
			require.NoError(t, err)
			var publicProjection any
			require.NoError(t, json.Unmarshal(before, &publicProjection))
			for _, forbidden := range test.forbiddenJSONFields {
				assert.False(t, containsJSONField(publicProjection, forbidden), "projection exposes private field %q: %s", forbidden, before)
			}

			clone := test.clone(test.projection)
			assert.Equal(t, test.projection, clone)
			require.True(t, mutateProjectionReferences(reflect.ValueOf(clone).Elem(), false), "projection fixture contains no nested mutable references")
			assert.NotEqual(t, test.projection, clone)

			after, err := json.Marshal(test.projection)
			require.NoError(t, err)
			assert.Equal(t, before, after)
		})
	}
}

func TestVersionOneSessionJSONContainsOnlyDurableAuthoredFields(t *testing.T) {
	t.Parallel()

	session := Session{
		Version: 1,
		Name:    "Persistence boundary",
		Terminals: []Terminal{{
			ID:        "durable-terminal",
			Name:      "Overseer",
			HackLevel: 4,
			IntroText: "WELCOME",
			Root: ContentNode{
				ID:   "root",
				Type: NodeFolder,
				Name: "ROOT",
				Children: []ContentNode{{
					ID: "entry", Type: NodeEntry, Name: "STATUS", Description: "Authored content",
				}},
			},
		}},
	}

	encoded, err := EncodeSession(session)
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, json.Unmarshal(encoded, &document))
	assertJSONFieldSet(t, document, "session", "name", "terminalGroups", "terminals", "version")

	terminals, ok := document["terminals"].([]any)
	require.True(t, ok)
	require.Len(t, terminals, 1)
	terminal, ok := terminals[0].(map[string]any)
	require.True(t, ok)
	assertJSONFieldSet(t, terminal, "terminal", "hackLevel", "id", "introText", "name", "root")

	for _, forbidden := range []string{
		"browserToken", "sessionId", "logicalSessionId", "sessions", "fallbackName",
		"connectionId", "connectionIds", "connected", "roster", "characterId",
		"claimedBySessionId", "assignmentsBySession", "sessionByCharacter", "role",
		"controllerSessionId", "broadcastId", "activeTerminalId", "revision",
		"pendingSwitch", "switchId", "terminalRuntimes", "lifecycle", "nav", "hack",
		"generationId", "secretWord", "wordsById", "usedPatterns", "attemptsLeft",
		"board", "patterns", "log", "outcome",
	} {
		assert.False(t, containsJSONField(document, forbidden), "version-1 session JSON contains process-local field %q: %s", forbidden, encoded)
	}
}

func seedProjection(t *testing.T, value reflect.Value, path string, depth int) {
	t.Helper()
	require.LessOrEqual(t, depth, 12, "projection type is unexpectedly recursive at %s", path)

	switch value.Kind() {
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
		seedProjection(t, value.Elem(), path+"*", depth+1)
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			field := value.Field(index)
			if field.CanSet() {
				seedProjection(t, field, path+"."+value.Type().Field(index).Name, depth+1)
			}
		}
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 1, 1))
		seedProjection(t, value.Index(0), path+"[0]", depth+1)
	case reflect.Map:
		value.Set(reflect.MakeMap(value.Type()))
		key := reflect.New(value.Type().Key()).Elem()
		seedProjection(t, key, path+".key", depth+1)
		element := reflect.New(value.Type().Elem()).Elem()
		seedProjection(t, element, path+".value", depth+1)
		value.SetMapIndex(key, element)
	case reflect.String:
		value.SetString(path)
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(int64(depth + 1))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(uint64(depth + 1))
	case reflect.Float32, reflect.Float64:
		value.SetFloat(float64(depth) + 0.5)
	}
}

func mutateProjectionReferences(value reflect.Value, behindReference bool) bool {
	mutated := false
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			return mutateProjectionReferences(value.Elem(), true)
		}
	case reflect.Struct:
		for _, field := range value.Fields() {
			if field.CanSet() && mutateProjectionReferences(field, behindReference) {
				mutated = true
			}
		}
	case reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			if mutateProjectionReferences(value.Index(index), true) {
				mutated = true
			}
		}
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			element := reflect.New(value.Type().Elem()).Elem()
			element.Set(iterator.Value())
			if mutateProjectionReferences(element, true) {
				value.SetMapIndex(iterator.Key(), element)
				mutated = true
			}
		}
	case reflect.String:
		if behindReference {
			value.SetString(value.String() + "-mutated")
			mutated = true
		}
	case reflect.Bool:
		if behindReference {
			value.SetBool(!value.Bool())
			mutated = true
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if behindReference {
			value.SetInt(value.Int() + 1)
			mutated = true
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if behindReference {
			value.SetUint(value.Uint() + 1)
			mutated = true
		}
	case reflect.Float32, reflect.Float64:
		if behindReference {
			value.SetFloat(value.Float() + 1)
			mutated = true
		}
	}
	return mutated
}

func assertJSONFieldSet(t *testing.T, object map[string]any, location string, want ...string) {
	t.Helper()
	got := make([]string, 0, len(object))
	for field := range object {
		got = append(got, field)
	}
	sort.Strings(got)
	sort.Strings(want)
	assert.Equal(t, want, got, "%s fields", location)
}

func containsJSONField(value any, forbidden string) bool {
	switch value := value.(type) {
	case map[string]any:
		for field, nested := range value {
			if field == forbidden || containsJSONField(nested, forbidden) {
				return true
			}
		}
	case []any:
		for _, nested := range value {
			if containsJSONField(nested, forbidden) {
				return true
			}
		}
	}
	return false
}

func TestCloneLiveBroadcastPreservesReturnPointProvenanceAndDetachesRoute(t *testing.T) {
	t.Parallel()

	original := &LiveBroadcast{
		ID: "broadcast-route",
		Route: []TerminalReturnPoint{
			{
				TerminalID:        "terminal-authored",
				TerminalName:      "Authored",
				FolderID:          "archive",
				AncestorFolderIDs: []string{"root", "records"},
				CommandID:         "go",
				CommandName:       "GO",
				Origin:            TerminalReturnAuthored,
			},
			{
				TerminalID:    "terminal-prefix",
				TerminalName:  "Prefix",
				Origin:        TerminalReturnInitialPrefix,
				GroupID:       "group-route",
				GroupPosition: 1,
			},
		},
	}

	clone := CloneLiveBroadcast(original)
	require.Equal(t, original, clone)
	require.NotSame(t, original, clone)
	require.Len(t, clone.Route, 2)
	assert.Equal(t, TerminalReturnAuthored, clone.Route[0].Origin)
	assert.Empty(t, clone.Route[0].GroupID)
	assert.Equal(t, TerminalReturnInitialPrefix, clone.Route[1].Origin)
	assert.Equal(t, "group-route", clone.Route[1].GroupID)
	assert.Equal(t, 1, clone.Route[1].GroupPosition)

	clone.Route[0].AncestorFolderIDs[0] = "mutated-root"
	clone.Route[0].CommandName = "MUTATED"
	clone.Route[1].GroupID = "mutated-group"
	clone.Route[1].GroupPosition = 99
	assert.Equal(t, []string{"root", "records"}, original.Route[0].AncestorFolderIDs)
	assert.Equal(t, "GO", original.Route[0].CommandName)
	assert.Equal(t, "group-route", original.Route[1].GroupID)
	assert.Equal(t, 1, original.Route[1].GroupPosition)
}

func TestCloneLiveBroadcastPreservesInitialTerminalEstablishedState(t *testing.T) {
	t.Parallel()

	for _, established := range []bool{false, true} {
		name := "fresh"
		if established {
			name = "initialized"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			original := &LiveBroadcast{
				ID:                           BroadcastID("broadcast-" + name),
				InitialTerminalEstablished:   established,
				InitialTerminalID:            "terminal-c",
				InitialTerminalGroupID:       "ordered-route",
				InitialTerminalGroupPosition: 2,
			}
			clone := CloneLiveBroadcast(original)
			require.NotNil(t, clone)
			assert.Equal(t, established, clone.InitialTerminalEstablished)
			assert.Equal(t, "terminal-c", clone.InitialTerminalID)
			assert.Equal(t, "ordered-route", clone.InitialTerminalGroupID)
			assert.Equal(t, 2, clone.InitialTerminalGroupPosition)

			clone.InitialTerminalEstablished = !established
			clone.InitialTerminalID = "mutated"
			clone.InitialTerminalGroupID = "mutated"
			clone.InitialTerminalGroupPosition = 99
			assert.Equal(t, established, original.InitialTerminalEstablished)
			assert.Equal(t, "terminal-c", original.InitialTerminalID)
			assert.Equal(t, "ordered-route", original.InitialTerminalGroupID)
			assert.Equal(t, 2, original.InitialTerminalGroupPosition)
		})
	}

	assert.Nil(t, CloneLiveBroadcast(nil))
}

func TestVersionOneEncodingIsInvariantAcrossCompleteProcessRuntimeActivity(t *testing.T) {
	session := Session{
		Version: 1, Name: "Runtime-neutral campaign",
		Terminals: []Terminal{{
			ID: "terminal-1", Name: "Overseer", HackLevel: 2, IntroText: "WELCOME",
			Root: ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}},
		}},
	}
	before, err := EncodeSession(session)
	require.NoError(t, err)
	controller := LogicalSessionID("session-1")
	activeTerminal := "terminal-1"
	_ = ProcessRuntime{
		Revision: 99,
		SessionsByID: map[LogicalSessionID]*LogicalSession{
			controller: {ID: controller, FallbackName: "TABLET LEFT", ConnectionIDs: map[ConnectionID]struct{}{"connection-1": {}}, RequestResults: map[RequestID]RequestResultRecord{}},
		},
		SessionIDByBrowserToken: map[BrowserToken]LogicalSessionID{"opaque-token": controller},
		RosterByID:              map[CharacterID]*CharacterRosterEntry{"character-1": {ID: "character-1", Name: "Mara"}},
		RosterOrder:             []CharacterID{"character-1"},
		Broadcast: &LiveBroadcast{
			ID: "broadcast-1", AssignmentsBySession: map[LogicalSessionID]CharacterID{controller: "character-1"},
			SessionByCharacter: map[CharacterID]LogicalSessionID{"character-1": controller}, ControllerSessionID: &controller,
			ActiveTerminalID: &activeTerminal, TerminalRuntimes: map[string]*TerminalRuntime{
				"terminal-1": {TerminalID: "terminal-1", TerminalName: "Overseer", HackLevel: 2, Lifecycle: TerminalLifecycleActive, Hack: &HackState{GenerationID: "private-generation", SecretWord: "SECRET"}},
			},
		},
		PendingSwitch: &TerminalSwitchDecision{ID: "switch-1", BroadcastID: "broadcast-1", SourceTerminalID: "terminal-1"},
	}
	after, err := EncodeSession(session)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "runtime activity changed version-1 encoding")
	for _, forbidden := range []string{"browserToken", "fallbackName", "connection", "broadcast", "controller", "claim", "pendingSwitch", "generation", "secretWord", "terminalRuntimes"} {
		assert.NotContains(t, string(after), forbidden, "durable encoding leaked runtime field")
	}
	decoded, err := DecodeSession(after)
	require.NoError(t, err)
	require.Len(t, decoded.Terminals, 1)
	assert.Equal(t, "terminal-1", decoded.Terminals[0].ID)
	assert.Equal(t, 2, decoded.Terminals[0].HackLevel)
}

func TestSessionPlayerConfigReferenceIsOptionalAndRoundTrips(t *testing.T) {
	t.Parallel()

	legacy := validSessionForPlayerConfigTest()
	encoded, err := EncodeSession(legacy)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"playerConfig"`)

	legacy.PlayerConfig = filepath.Join("players", "vault-13.json")
	encoded, err = EncodeSession(legacy)
	require.NoError(t, err)
	decoded, err := DecodeSession(encoded)
	require.NoError(t, err)
	assert.Equal(t, legacy.PlayerConfig, decoded.PlayerConfig)
}

func TestPlayerConfigV1StrictValidationAndStableEncoding(t *testing.T) {
	t.Parallel()

	empty := PlayerConfig{Version: 1, Name: "Empty Players", Roster: []CharacterRosterEntry{}}
	emptyEncoded, err := EncodePlayerConfig(empty)
	require.NoError(t, err)
	assert.Contains(t, string(emptyEncoded), `"roster": []`)
	emptyDecoded, err := DecodePlayerConfig(emptyEncoded)
	require.NoError(t, err)
	assert.NotNil(t, emptyDecoded.Roster)

	config := PlayerConfig{
		Version: 1,
		Name:    "Vault 13 Players",
		Roster: []CharacterRosterEntry{
			{ID: "mara", Name: "Mara", Intelligence: 10, HackerPerkAvailable: true},
			{ID: "boone", Name: "Boone", Intelligence: 1, HackerPerkAvailable: false},
		},
	}
	encoded, err := EncodePlayerConfig(config)
	require.NoError(t, err)
	assert.Equal(t, `{
  "version": 1,
  "name": "Vault 13 Players",
  "roster": [
    {
      "id": "mara",
      "name": "Mara",
      "intelligence": 10,
      "hackerPerkAvailable": true
    },
    {
      "id": "boone",
      "name": "Boone",
      "intelligence": 1,
      "hackerPerkAvailable": false
    }
  ]
}
`, string(encoded), "canonical player config must emit both attributes in stable order")
	decoded, err := DecodePlayerConfig(encoded)
	require.NoError(t, err)
	assert.Equal(t, config, decoded)
	assert.Equal(t, []CharacterID{"mara", "boone"}, []CharacterID{decoded.Roster[0].ID, decoded.Roster[1].ID})
	assert.True(t, bytes.HasSuffix(encoded, []byte("\n")), "player config must end with a newline")
	var document map[string]any
	require.NoError(t, json.Unmarshal(encoded, &document))
	assertJSONFieldSet(t, document, "player config", "name", "roster", "version")
	for _, forbidden := range []string{
		"browserToken", "sessionId", "connectionId", "connected", "claimedBySessionId",
		"controllerSessionId", "broadcastId", "revision", "requestId", "activeTerminalId",
		"nav", "hack", "secretWord", "attemptsLeft", "patterns", "log", "outcome",
	} {
		assert.False(t, containsJSONField(document, forbidden), "player config contains runtime field %q: %s", forbidden, encoded)
	}

	invalid := []struct {
		name string
		raw  string
	}{
		{name: "unsupported version", raw: `{"version":2,"name":"Players","roster":[]}`},
		{name: "blank name", raw: `{"version":1,"name":" ","roster":[]}`},
		{name: "null roster", raw: `{"version":1,"name":"Players","roster":null}`},
		{name: "duplicate stable ID", raw: `{"version":1,"name":"Players","roster":[{"id":"same","name":"One","intelligence":1,"hackerPerkAvailable":false},{"id":"same","name":"Two","intelligence":2,"hackerPerkAvailable":true}]}`},
		{name: "unknown top level field", raw: `{"version":1,"name":"Players","roster":[],"browserToken":"secret"}`},
		{name: "unknown nested field", raw: `{"version":1,"name":"Players","roster":[{"id":"mara","name":"Mara","intelligence":5,"hackerPerkAvailable":true,"futureAttribute":1}]}`},
		{name: "trailing value", raw: `{"version":1,"name":"Players","roster":[]} {}`},
		{name: "explicit zero intelligence", raw: `{"version":1,"name":"Players","roster":[{"id":"mara","name":"Mara","intelligence":0,"hackerPerkAvailable":false}]}`},
		{name: "intelligence above maximum", raw: `{"version":1,"name":"Players","roster":[{"id":"mara","name":"Mara","intelligence":11,"hackerPerkAvailable":false}]}`},
		{name: "fractional intelligence", raw: `{"version":1,"name":"Players","roster":[{"id":"mara","name":"Mara","intelligence":1.5,"hackerPerkAvailable":false}]}`},
		{name: "string intelligence", raw: `{"version":1,"name":"Players","roster":[{"id":"mara","name":"Mara","intelligence":"5","hackerPerkAvailable":false}]}`},
		{name: "null intelligence", raw: `{"version":1,"name":"Players","roster":[{"id":"mara","name":"Mara","intelligence":null,"hackerPerkAvailable":false}]}`},
		{name: "null hacker perk", raw: `{"version":1,"name":"Players","roster":[{"id":"mara","name":"Mara","intelligence":5,"hackerPerkAvailable":null}]}`},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodePlayerConfig([]byte(test.raw))
			assert.Error(t, err, "DecodePlayerConfig(%s) unexpectedly succeeded", test.raw)
		})
	}
}

func TestDecodePlayerConfigV1AppliesOnlyLegacyAttributeDefaults(t *testing.T) {
	t.Parallel()

	decoded, err := DecodePlayerConfig([]byte(`{
  "version": 1,
  "name": "Legacy Players",
  "roster": [
    {"id":"legacy","name":"Legacy"},
    {"id":"smart","name":"Smart","intelligence":8},
    {"id":"hacker","name":"Hacker","hackerPerkAvailable":true}
  ]
}`))
	require.NoError(t, err)
	require.Len(t, decoded.Roster, 3)
	assert.Equal(t, []CharacterRosterEntry{
		{ID: "legacy", Name: "Legacy", Intelligence: 1, HackerPerkAvailable: false},
		{ID: "smart", Name: "Smart", Intelligence: 8, HackerPerkAvailable: false},
		{ID: "hacker", Name: "Hacker", Intelligence: 1, HackerPerkAvailable: true},
	}, decoded.Roster, "legacy defaults must preserve stable identities and authored order")

	encoded, err := EncodePlayerConfig(decoded)
	require.NoError(t, err)
	assert.Equal(t, 3, bytes.Count(encoded, []byte(`"intelligence"`)))
	assert.Equal(t, 3, bytes.Count(encoded, []byte(`"hackerPerkAvailable"`)))
}

func TestCloneMasterCoordinationStatePreservesPlayerAttributesAndDetachesRoster(t *testing.T) {
	t.Parallel()

	original := &MasterCoordinationState{Roster: []MasterRosterEntry{
		{ID: "mara", Name: "Mara", Intelligence: 9, HackerPerkAvailable: true},
		{ID: "boone", Name: "Boone", Intelligence: 3, HackerPerkAvailable: false},
	}}
	clone := CloneMasterCoordinationState(original)
	require.Equal(t, original, clone)

	clone.Roster[0].Name = "Changed"
	clone.Roster[0].Intelligence = 1
	clone.Roster[0].HackerPerkAvailable = false
	assert.Equal(t, MasterRosterEntry{
		ID: "mara", Name: "Mara", Intelligence: 9, HackerPerkAvailable: true,
	}, original.Roster[0])
}

func validSessionForPlayerConfigTest() Session {
	return Session{
		Version: 1,
		Name:    "Campaign",
		Terminals: []Terminal{{
			ID: "terminal-1", Name: "Terminal", HackLevel: 0,
			Root: ContentNode{ID: "root", Type: NodeFolder, Name: "ROOT", Children: []ContentNode{}},
		}},
	}
}
