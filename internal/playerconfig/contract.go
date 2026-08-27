package playerconfig

import (
	"fmt"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	persistencev1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/persistence/v1"
)

func PlayerConfigToProto(value domain.PlayerConfig) (*persistencev1.PlayerConfig, error) {
	if err := domain.ValidatePlayerConfig(value); err != nil {
		return nil, err
	}
	result := &persistencev1.PlayerConfig{Version: int32(value.Version), Name: value.Name, Roster: make([]*persistencev1.RosterEntry, 0, len(value.Roster))}
	for _, entry := range value.Roster {
		result.Roster = append(result.Roster, &persistencev1.RosterEntry{
			Id:                  string(entry.ID),
			Name:                entry.Name,
			Intelligence:        new(int32(entry.Intelligence)),
			HackerPerkAvailable: new(entry.HackerPerkAvailable),
		})
	}
	return result, nil
}

func PlayerConfigFromProto(value *persistencev1.PlayerConfig) (domain.PlayerConfig, error) {
	if value == nil {
		return domain.PlayerConfig{}, fmt.Errorf("player config contract is required")
	}
	result := domain.PlayerConfig{Version: int(value.GetVersion()), Name: value.GetName(), Roster: make([]domain.CharacterRosterEntry, 0, len(value.GetRoster()))}
	for _, entry := range value.GetRoster() {
		intelligence := int32(1)
		hackerPerkAvailable := false
		if entry != nil {
			if entry.Intelligence != nil {
				intelligence = entry.GetIntelligence()
			}
			if entry.HackerPerkAvailable != nil {
				hackerPerkAvailable = entry.GetHackerPerkAvailable()
			}
		}
		result.Roster = append(result.Roster, domain.CharacterRosterEntry{
			ID:                  domain.CharacterID(entry.GetId()),
			Name:                entry.GetName(),
			Intelligence:        int(intelligence),
			HackerPerkAvailable: hackerPerkAvailable,
		})
	}
	if err := domain.ValidatePlayerConfig(result); err != nil {
		return domain.PlayerConfig{}, err
	}
	return result, nil
}

func verifyPlayerConfigContract(value domain.PlayerConfig) error {
	semantic, err := PlayerConfigToProto(value)
	if err != nil {
		return err
	}
	_, err = PlayerConfigFromProto(semantic)
	return err
}
