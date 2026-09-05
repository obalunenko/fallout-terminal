package domain

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionJSONKeepsLegacyFacilityAbsent(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "version": 1,
  "name": "Legacy facility-free session",
  "terminals": [{
    "id": "terminal-a",
    "name": "Terminal A",
    "hackLevel": 0,
    "introText": "",
    "root": {"id": "root", "type": "folder", "name": "ROOT", "children": []}
  }]
}`)

	session, err := DecodeSession(raw)
	require.NoError(t, err)
	assert.Nil(t, session.Facility)

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"facility"`)

	roundTrip, err := DecodeSession(encoded)
	require.NoError(t, err)
	assert.Nil(t, roundTrip.Facility)
}

func TestFacilityJSONPreservesUnknownFieldsOnEveryNestedEntity(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "version": 1,
  "name": "Facility extras",
  "terminals": [{
    "id": "terminal-a",
    "name": "Terminal A",
    "hackLevel": 0,
    "introText": "",
    "root": {
      "id": "root",
      "type": "folder",
      "name": "ROOT",
      "children": [{
        "id": "command-open",
        "type": "command",
        "name": "OPEN",
        "text": "OPEN",
        "stateChange": {
          "completedName": "OPENED",
          "confirmationText": "Open?",
          "facilityAction": {
            "transitions": [{"deviceId": "door-a", "transitionId": "open", "futureRequest": true}],
            "futureAction": true
          }
        },
        "facilityNameVariants": [{
          "when": {"deviceId": "door-a", "stateId": "open", "futureEquality": true},
          "text": "OPENED",
          "futureVariant": true
        }],
        "availableWhen": {"deviceId": "door-a", "stateId": "closed", "futureAvailability": true}
      }, {
        "id": "entry-status",
        "type": "entry",
        "name": "STATUS",
        "blocks": [{
          "id": "block-status",
          "initialText": "CLOSED",
          "facilityTextVariants": [{
            "when": {"deviceId": "door-a", "stateId": "open", "futureBlockEquality": true},
            "text": "OPEN",
            "futureBlockVariant": true
          }],
          "futureBlock": true
        }],
        "visibleWhen": {"deviceId": "door-a", "stateId": "open", "futureVisibility": true}
      }]
    }
  }],
  "facility": {
    "revision": 7,
    "devices": [{
      "id": "door-a",
      "name": "Door A",
      "kind": "door",
      "initialStateId": "closed",
      "currentStateId": "closed",
      "states": [
        {"id": "closed", "name": "Closed", "futureState": true},
        {"id": "open", "name": "Open"}
      ],
      "transitions": [{
        "id": "open",
        "name": "Open door",
        "sourceStateId": "closed",
        "destinationStateId": "open",
        "preconditions": [{"deviceId": "power-a", "stateId": "online", "futurePrecondition": true}],
        "conditionEffects": [{"conditionId": "door-offline", "active": false, "futureConditionEffect": true}],
        "futureTransition": true
      }],
      "futureDevice": true
    }, {
      "id": "power-a",
      "name": "Power A",
      "kind": "power-grid",
      "initialStateId": "online",
      "currentStateId": "online",
      "states": [{"id": "online", "name": "Online"}],
      "transitions": []
    }],
    "conditions": [{
      "id": "door-offline",
      "name": "Door offline",
      "category": "offline",
      "device": {"deviceId": "door-a", "futureScope": true},
      "initialActive": true,
      "currentActive": true,
      "effects": [{"displayInstability": {"futureDisplayEffect": true}, "futureEffect": true}],
      "recovery": [{"privateOverseerAction": true, "futureRecovery": true}],
      "futureCondition": true
    }],
    "recoveryPrograms": [{
      "id": "program-open",
      "name": "Open program",
      "transitions": [{"deviceId": "door-a", "transitionId": "open", "futureProgramRequest": true}],
      "futureProgram": true
    }],
    "futureFacility": true
  }
}`)

	session, err := DecodeSession(raw)
	require.NoError(t, err)
	require.NotNil(t, session.Facility)
	require.Len(t, session.Facility.Devices, 2)
	require.Len(t, session.Facility.Conditions, 1)
	require.Len(t, session.Facility.RecoveryPrograms, 1)

	session.Facility.Devices[0].Name = "Renamed Door"
	session.Facility.Conditions[0].Name = "Renamed Condition"
	session.Facility.RecoveryPrograms[0].Name = "Renamed Program"

	encoded, err := EncodeSession(session)
	require.NoError(t, err)
	for _, field := range []string{
		"futureAction", "futureAvailability", "futureBlock", "futureBlockEquality",
		"futureBlockVariant", "futureCondition", "futureConditionEffect", "futureDevice",
		"futureDisplayEffect", "futureEffect", "futureEquality", "futureFacility",
		"futurePrecondition", "futureProgram", "futureProgramRequest", "futureRecovery",
		"futureRequest", "futureScope", "futureState", "futureTransition", "futureVariant",
		"futureVisibility",
	} {
		assert.Contains(t, string(encoded), `"`+field+`"`, field)
	}
	assert.Contains(t, string(encoded), "Renamed Door")
	assert.Contains(t, string(encoded), "Renamed Condition")
	assert.Contains(t, string(encoded), "Renamed Program")

	var normalized any
	require.NoError(t, json.Unmarshal(encoded, &normalized))
	stable, err := json.Marshal(normalized)
	require.NoError(t, err)
	assert.False(t, bytes.Contains(stable, []byte(`"facility":null`)))

	roundTrip, err := DecodeSession(encoded)
	require.NoError(t, err)
	require.NotNil(t, roundTrip.Facility)
	assert.Equal(t, "Renamed Door", roundTrip.Facility.Devices[0].Name)
	assert.Equal(t, "Renamed Condition", roundTrip.Facility.Conditions[0].Name)
	assert.Equal(t, "Renamed Program", roundTrip.Facility.RecoveryPrograms[0].Name)
}
