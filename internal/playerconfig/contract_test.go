package playerconfig

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	persistencev1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/persistence/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestPlayerConfigContractMapsEveryKnownField(t *testing.T) {
	value := domain.PlayerConfig{Version: 1, Name: "Vault", Roster: []domain.CharacterRosterEntry{
		{ID: "character-1", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true},
		{ID: "character-2", Name: "Boone", Intelligence: 4, HackerPerkAvailable: false},
	}}
	semantic, err := PlayerConfigToProto(value)
	require.NoError(t, err)
	require.Equal(t, int32(1), semantic.GetVersion())
	require.Len(t, semantic.GetRoster(), 2)
	require.Equal(t, "character-2", semantic.GetRoster()[1].GetId())
	require.NotNil(t, semantic.GetRoster()[1].Intelligence)
	require.Equal(t, int32(4), semantic.GetRoster()[1].GetIntelligence())
	require.NotNil(t, semantic.GetRoster()[1].HackerPerkAvailable)
	require.False(t, semantic.GetRoster()[1].GetHackerPerkAvailable())
	roundTrip, err := PlayerConfigFromProto(semantic)
	require.NoError(t, err)
	require.Equal(t, value, roundTrip)
	roundTripSemantic, err := PlayerConfigToProto(roundTrip)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(semantic, roundTripSemantic, protocmp.Transform()))
}

func TestPlayerConfigContractAppliesDefaultsOnlyToAbsentOptionalProfileFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   *persistencev1.RosterEntry
		want    domain.CharacterRosterEntry
		wantErr bool
	}{
		{
			name:  "legacy fields absent",
			entry: &persistencev1.RosterEntry{Id: "mara", Name: "Mara"},
			want:  domain.CharacterRosterEntry{ID: "mara", Name: "Mara", Intelligence: 1, HackerPerkAvailable: false},
		},
		{
			name: "explicit minimum and false",
			entry: &persistencev1.RosterEntry{
				Id: "mara", Name: "Mara", Intelligence: proto.Int32(1), HackerPerkAvailable: new(false),
			},
			want: domain.CharacterRosterEntry{ID: "mara", Name: "Mara", Intelligence: 1, HackerPerkAvailable: false},
		},
		{
			name: "explicit maximum and true",
			entry: &persistencev1.RosterEntry{
				Id: "mara", Name: "Mara", Intelligence: proto.Int32(10), HackerPerkAvailable: new(true),
			},
			want: domain.CharacterRosterEntry{ID: "mara", Name: "Mara", Intelligence: 10, HackerPerkAvailable: true},
		},
		{
			name: "present invalid intelligence is not defaulted",
			entry: &persistencev1.RosterEntry{
				Id: "mara", Name: "Mara", Intelligence: proto.Int32(0), HackerPerkAvailable: new(false),
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			semantic := &persistencev1.PlayerConfig{
				Version: 1,
				Name:    "Vault",
				Roster:  []*persistencev1.RosterEntry{test.entry},
			}
			got, err := PlayerConfigFromProto(semantic)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []domain.CharacterRosterEntry{test.want}, got.Roster)
		})
	}
}

func TestPlayerConfigContractRetainsStrictVersionIdentityAndDuplicateValidation(t *testing.T) {
	tests := []domain.PlayerConfig{
		{Version: 2, Name: "Vault", Roster: []domain.CharacterRosterEntry{}},
		{Version: 1, Name: "Vault", Roster: nil},
		{Version: 1, Name: "Vault", Roster: []domain.CharacterRosterEntry{{ID: "same", Name: "Mara"}, {ID: "same", Name: "Boone"}}},
	}
	for _, value := range tests {
		_, err := PlayerConfigToProto(value)
		require.Error(t, err)
	}
}
