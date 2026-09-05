package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

var (
	sessionFields  = fieldSet("version", "name", "playerConfig", "terminals", "terminalGroups", "facility")
	terminalFields = fieldSet("id", "name", "hackLevel", "introText", "root", "commandStates")
	nodeFields     = fieldSet(
		"id", "type", "name", "children", "text", "description", "blocks", "stateChange", "terminalTransition",
		"facilityNameVariants", "visibleWhen", "availableWhen",
	)
	entryContentBlockFields       = fieldSet("id", "initialText", "facilityTextVariants")
	facilityFields                = fieldSet("revision", "devices", "conditions", "recoveryPrograms")
	facilityDeviceFields          = fieldSet("id", "name", "kind", "customKind", "initialStateId", "currentStateId", "states", "transitions")
	facilityDeviceStateFields     = fieldSet("id", "name")
	facilityTransitionFields      = fieldSet("id", "name", "sourceStateId", "destinationStateId", "preconditions", "conditionEffects", "recovery")
	facilityEqualityFields        = fieldSet("deviceId", "stateId")
	facilityRequestFields         = fieldSet("deviceId", "transitionId")
	facilityConditionEffectFields = fieldSet("conditionId", "active")
	diagnosticDeviceScopeFields   = fieldSet("deviceId")
	diagnosticTerminalScopeFields = fieldSet("terminalId")
	capabilityBlockEffectFields   = fieldSet("capability")
	diagnosticPathEffectFields    = fieldSet("terminalId", "nodeId")
	recordSubstitutionFields      = fieldSet("terminalId", "blockId", "replacementText")
	diagnosticEffectFields        = fieldSet("capabilityBlock", "diagnosticPath", "recordSubstitution", "displayInstability")
	diagnosticRecoveryFields      = fieldSet("transition", "recoveryProgramId", "privateOverseerAction")
	diagnosticConditionFields     = fieldSet("id", "name", "category", "customCategory", "device", "terminal", "initialActive", "currentActive", "effects", "recovery")
	recoveryProgramFields         = fieldSet("id", "name", "transitions")
	facilityActionFields          = fieldSet("transitions", "recoveryProgramId")
	facilityTransitionListFields  = fieldSet("transitions")
	facilityTextVariantFields     = fieldSet("when", "text")
)

// DecodePlayerConfig strictly decodes one standalone version-1 authored roster.
func DecodePlayerConfig(data []byte) (PlayerConfig, error) {
	var wire playerConfigWire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return PlayerConfig{}, fmt.Errorf("decode player config: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return PlayerConfig{}, fmt.Errorf("decode player config: trailing JSON value")
		}
		return PlayerConfig{}, fmt.Errorf("decode player config: %w", err)
	}
	config, err := wire.canonical()
	if err != nil {
		return PlayerConfig{}, fmt.Errorf("decode player config: %w", err)
	}
	if err := ValidatePlayerConfig(config); err != nil {
		return PlayerConfig{}, fmt.Errorf("validate player config: %w", err)
	}
	return config, nil
}

type playerConfigWire struct {
	Version int                           `json:"version"`
	Name    string                        `json:"name"`
	Roster  []playerConfigRosterEntryWire `json:"roster"`
}

type playerConfigRosterEntryWire struct {
	ID                  CharacterID     `json:"id"`
	Name                string          `json:"name"`
	Intelligence        json.RawMessage `json:"intelligence"`
	HackerPerkAvailable json.RawMessage `json:"hackerPerkAvailable"`
}

func (wire playerConfigWire) canonical() (PlayerConfig, error) {
	config := PlayerConfig{Version: wire.Version, Name: wire.Name}
	if wire.Roster != nil {
		config.Roster = make([]CharacterRosterEntry, 0, len(wire.Roster))
	}
	for index, entry := range wire.Roster {
		intelligence, err := decodeLegacyInt(entry.Intelligence, 1)
		if err != nil {
			return PlayerConfig{}, fmt.Errorf("roster[%d].intelligence: %w", index, err)
		}
		hackerAvailable, err := decodeLegacyBool(entry.HackerPerkAvailable, false)
		if err != nil {
			return PlayerConfig{}, fmt.Errorf("roster[%d].hackerPerkAvailable: %w", index, err)
		}
		config.Roster = append(config.Roster, CharacterRosterEntry{
			ID: entry.ID, Name: entry.Name, Intelligence: intelligence, HackerPerkAvailable: hackerAvailable,
		})
	}
	return config, nil
}

func decodeLegacyInt(raw json.RawMessage, fallback int) (int, error) {
	if len(raw) == 0 {
		return fallback, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, fmt.Errorf("must be an integer")
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("must be an integer: %w", err)
	}
	return value, nil
}

func decodeLegacyBool(raw json.RawMessage, fallback bool) (bool, error) {
	if len(raw) == 0 {
		return fallback, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, fmt.Errorf("must be a boolean")
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("must be a boolean: %w", err)
	}
	return value, nil
}

// EncodePlayerConfig emits stable, human-readable version-1 JSON.
func EncodePlayerConfig(config PlayerConfig) ([]byte, error) {
	if err := ValidatePlayerConfig(config); err != nil {
		return nil, fmt.Errorf("validate player config: %w", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode player config: %w", err)
	}
	return append(data, '\n'), nil
}

// DecodeSession decodes a version-1 document while retaining compatible unknown fields.
func DecodeSession(data []byte) (Session, error) {
	var session Session
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&session); err != nil {
		return Session{}, fmt.Errorf("decode session: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Session{}, fmt.Errorf("decode session: trailing JSON value")
		}
		return Session{}, fmt.Errorf("decode session: %w", err)
	}
	session = NormalizeTerminalGroups(session)
	if err := ValidateSession(session); err != nil {
		return Session{}, fmt.Errorf("validate session: %w", err)
	}
	return session, nil
}

// EncodeSession emits stable, human-readable version-1 JSON with a final newline.
func EncodeSession(session Session) ([]byte, error) {
	session = NormalizeTerminalGroups(session)
	if err := ValidateSession(session); err != nil {
		return nil, fmt.Errorf("validate session: %w", err)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode session: %w", err)
	}
	return append(data, '\n'), nil
}

// UnmarshalJSON retains unknown top-level session fields.
func (s *Session) UnmarshalJSON(data []byte) error {
	type alias Session
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	s.Extra = extrasFrom(data, sessionFields)
	decoded.Extra = s.Extra
	*s = Session(decoded)
	return nil
}

// MarshalJSON restores unknown top-level session fields.
func (s Session) MarshalJSON() ([]byte, error) {
	type alias Session
	return marshalWithExtras(alias(s), s.Extra, sessionFields)
}

// UnmarshalJSON retains unknown terminal fields.
func (t *Terminal) UnmarshalJSON(data []byte) error {
	type alias Terminal
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	t.Extra = extrasFrom(data, terminalFields)
	decoded.Extra = t.Extra
	*t = Terminal(decoded)
	return nil
}

// MarshalJSON restores unknown terminal fields.
func (t Terminal) MarshalJSON() ([]byte, error) {
	type alias Terminal
	return marshalWithExtras(alias(t), t.Extra, terminalFields)
}

// UnmarshalJSON retains unknown fields on known content-node variants.
func (n *ContentNode) UnmarshalJSON(data []byte) error {
	type alias ContentNode
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	n.Extra = extrasFrom(data, nodeFields)
	decoded.Extra = n.Extra
	*n = ContentNode(decoded)
	return nil
}

// MarshalJSON restores unknown content-node fields and preserves folder children arrays.
func (n ContentNode) MarshalJSON() ([]byte, error) {
	type alias ContentNode
	data, err := marshalWithExtras(alias(n), n.Extra, nodeFields)
	if err != nil || n.Type != NodeFolder {
		return data, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	children, err := json.Marshal(n.Children)
	if err != nil {
		return nil, err
	}
	raw["children"] = children
	return json.Marshal(raw)
}

// UnmarshalJSON retains unknown entry-content block fields.
func (b *EntryContentBlock) UnmarshalJSON(data []byte) error {
	type alias EntryContentBlock
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	b.Extra = extrasFrom(data, entryContentBlockFields)
	decoded.Extra = b.Extra
	*b = EntryContentBlock(decoded)
	return nil
}

// MarshalJSON restores unknown entry-content block fields.
func (b EntryContentBlock) MarshalJSON() ([]byte, error) {
	type alias EntryContentBlock
	return marshalWithExtras(alias(b), b.Extra, entryContentBlockFields)
}

// UnmarshalJSON retains unknown facility fields.
func (f *Facility) UnmarshalJSON(data []byte) error {
	type alias Facility
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	f.Extra = extrasFrom(data, facilityFields)
	decoded.Extra = f.Extra
	*f = Facility(decoded)
	return nil
}

// MarshalJSON restores unknown facility fields.
func (f Facility) MarshalJSON() ([]byte, error) {
	type alias Facility
	return marshalWithExtras(alias(f), f.Extra, facilityFields)
}

// UnmarshalJSON retains unknown facility-device fields.
func (d *FacilityDevice) UnmarshalJSON(data []byte) error {
	type alias FacilityDevice
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	d.Extra = extrasFrom(data, facilityDeviceFields)
	decoded.Extra = d.Extra
	*d = FacilityDevice(decoded)
	return nil
}

// MarshalJSON restores unknown facility-device fields.
func (d FacilityDevice) MarshalJSON() ([]byte, error) {
	type alias FacilityDevice
	return marshalWithExtras(alias(d), d.Extra, facilityDeviceFields)
}

// UnmarshalJSON retains unknown facility-device state fields.
func (s *FacilityDeviceState) UnmarshalJSON(data []byte) error {
	type alias FacilityDeviceState
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	s.Extra = extrasFrom(data, facilityDeviceStateFields)
	decoded.Extra = s.Extra
	*s = FacilityDeviceState(decoded)
	return nil
}

// MarshalJSON restores unknown facility-device state fields.
func (s FacilityDeviceState) MarshalJSON() ([]byte, error) {
	type alias FacilityDeviceState
	return marshalWithExtras(alias(s), s.Extra, facilityDeviceStateFields)
}

// UnmarshalJSON retains unknown facility-device transition fields.
func (t *FacilityDeviceTransition) UnmarshalJSON(data []byte) error {
	type alias FacilityDeviceTransition
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	t.Extra = extrasFrom(data, facilityTransitionFields)
	decoded.Extra = t.Extra
	*t = FacilityDeviceTransition(decoded)
	return nil
}

// MarshalJSON restores unknown facility-device transition fields.
func (t FacilityDeviceTransition) MarshalJSON() ([]byte, error) {
	type alias FacilityDeviceTransition
	return marshalWithExtras(alias(t), t.Extra, facilityTransitionFields)
}

// UnmarshalJSON retains unknown facility state-equality fields.
func (e *FacilityStateEquality) UnmarshalJSON(data []byte) error {
	type alias FacilityStateEquality
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	e.Extra = extrasFrom(data, facilityEqualityFields)
	decoded.Extra = e.Extra
	*e = FacilityStateEquality(decoded)
	return nil
}

// MarshalJSON restores unknown facility state-equality fields.
func (e FacilityStateEquality) MarshalJSON() ([]byte, error) {
	type alias FacilityStateEquality
	return marshalWithExtras(alias(e), e.Extra, facilityEqualityFields)
}

// UnmarshalJSON retains unknown facility transition-request fields.
func (r *FacilityTransitionRequest) UnmarshalJSON(data []byte) error {
	type alias FacilityTransitionRequest
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	r.Extra = extrasFrom(data, facilityRequestFields)
	decoded.Extra = r.Extra
	*r = FacilityTransitionRequest(decoded)
	return nil
}

// MarshalJSON restores unknown facility transition-request fields.
func (r FacilityTransitionRequest) MarshalJSON() ([]byte, error) {
	type alias FacilityTransitionRequest
	return marshalWithExtras(alias(r), r.Extra, facilityRequestFields)
}

// UnmarshalJSON retains unknown facility condition-effect fields.
func (e *FacilityConditionEffect) UnmarshalJSON(data []byte) error {
	type alias FacilityConditionEffect
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	e.Extra = extrasFrom(data, facilityConditionEffectFields)
	decoded.Extra = e.Extra
	*e = FacilityConditionEffect(decoded)
	return nil
}

// MarshalJSON restores unknown facility condition-effect fields.
func (e FacilityConditionEffect) MarshalJSON() ([]byte, error) {
	type alias FacilityConditionEffect
	return marshalWithExtras(alias(e), e.Extra, facilityConditionEffectFields)
}

// UnmarshalJSON retains unknown device-scope fields.
func (s *DiagnosticDeviceScope) UnmarshalJSON(data []byte) error {
	type alias DiagnosticDeviceScope
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	s.Extra = extrasFrom(data, diagnosticDeviceScopeFields)
	decoded.Extra = s.Extra
	*s = DiagnosticDeviceScope(decoded)
	return nil
}

// MarshalJSON restores unknown device-scope fields.
func (s DiagnosticDeviceScope) MarshalJSON() ([]byte, error) {
	type alias DiagnosticDeviceScope
	return marshalWithExtras(alias(s), s.Extra, diagnosticDeviceScopeFields)
}

// UnmarshalJSON retains unknown terminal-scope fields.
func (s *DiagnosticTerminalScope) UnmarshalJSON(data []byte) error {
	type alias DiagnosticTerminalScope
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	s.Extra = extrasFrom(data, diagnosticTerminalScopeFields)
	decoded.Extra = s.Extra
	*s = DiagnosticTerminalScope(decoded)
	return nil
}

// MarshalJSON restores unknown terminal-scope fields.
func (s DiagnosticTerminalScope) MarshalJSON() ([]byte, error) {
	type alias DiagnosticTerminalScope
	return marshalWithExtras(alias(s), s.Extra, diagnosticTerminalScopeFields)
}

// UnmarshalJSON retains unknown capability-block fields.
func (e *CapabilityBlockEffect) UnmarshalJSON(data []byte) error {
	type alias CapabilityBlockEffect
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	e.Extra = extrasFrom(data, capabilityBlockEffectFields)
	decoded.Extra = e.Extra
	*e = CapabilityBlockEffect(decoded)
	return nil
}

// MarshalJSON restores unknown capability-block fields.
func (e CapabilityBlockEffect) MarshalJSON() ([]byte, error) {
	type alias CapabilityBlockEffect
	return marshalWithExtras(alias(e), e.Extra, capabilityBlockEffectFields)
}

// UnmarshalJSON retains unknown diagnostic-path fields.
func (e *DiagnosticPathEffect) UnmarshalJSON(data []byte) error {
	type alias DiagnosticPathEffect
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	e.Extra = extrasFrom(data, diagnosticPathEffectFields)
	decoded.Extra = e.Extra
	*e = DiagnosticPathEffect(decoded)
	return nil
}

// MarshalJSON restores unknown diagnostic-path fields.
func (e DiagnosticPathEffect) MarshalJSON() ([]byte, error) {
	type alias DiagnosticPathEffect
	return marshalWithExtras(alias(e), e.Extra, diagnosticPathEffectFields)
}

// UnmarshalJSON retains unknown record-substitution fields.
func (e *RecordSubstitutionEffect) UnmarshalJSON(data []byte) error {
	type alias RecordSubstitutionEffect
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	e.Extra = extrasFrom(data, recordSubstitutionFields)
	decoded.Extra = e.Extra
	*e = RecordSubstitutionEffect(decoded)
	return nil
}

// MarshalJSON restores unknown record-substitution fields.
func (e RecordSubstitutionEffect) MarshalJSON() ([]byte, error) {
	type alias RecordSubstitutionEffect
	return marshalWithExtras(alias(e), e.Extra, recordSubstitutionFields)
}

// UnmarshalJSON retains unknown display-instability fields.
func (e *DisplayInstabilityEffect) UnmarshalJSON(data []byte) error {
	type alias DisplayInstabilityEffect
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	e.Extra = extrasFrom(data, nil)
	decoded.Extra = e.Extra
	*e = DisplayInstabilityEffect(decoded)
	return nil
}

// MarshalJSON restores unknown display-instability fields.
func (e DisplayInstabilityEffect) MarshalJSON() ([]byte, error) {
	type alias DisplayInstabilityEffect
	return marshalWithExtras(alias(e), e.Extra, nil)
}

// UnmarshalJSON retains unknown diagnostic-effect wrapper fields.
func (e *DiagnosticEffect) UnmarshalJSON(data []byte) error {
	type alias DiagnosticEffect
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	e.Extra = extrasFrom(data, diagnosticEffectFields)
	decoded.Extra = e.Extra
	*e = DiagnosticEffect(decoded)
	return nil
}

// MarshalJSON restores unknown diagnostic-effect wrapper fields.
func (e DiagnosticEffect) MarshalJSON() ([]byte, error) {
	type alias DiagnosticEffect
	return marshalWithExtras(alias(e), e.Extra, diagnosticEffectFields)
}

// UnmarshalJSON retains unknown diagnostic-recovery wrapper fields.
func (r *DiagnosticRecoveryReference) UnmarshalJSON(data []byte) error {
	type alias DiagnosticRecoveryReference
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	r.Extra = extrasFrom(data, diagnosticRecoveryFields)
	decoded.Extra = r.Extra
	*r = DiagnosticRecoveryReference(decoded)
	return nil
}

// MarshalJSON restores unknown diagnostic-recovery wrapper fields.
func (r DiagnosticRecoveryReference) MarshalJSON() ([]byte, error) {
	type alias DiagnosticRecoveryReference
	return marshalWithExtras(alias(r), r.Extra, diagnosticRecoveryFields)
}

// UnmarshalJSON retains unknown diagnostic-condition fields.
func (c *DiagnosticCondition) UnmarshalJSON(data []byte) error {
	type alias DiagnosticCondition
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	c.Extra = extrasFrom(data, diagnosticConditionFields)
	decoded.Extra = c.Extra
	*c = DiagnosticCondition(decoded)
	return nil
}

// MarshalJSON restores unknown diagnostic-condition fields.
func (c DiagnosticCondition) MarshalJSON() ([]byte, error) {
	type alias DiagnosticCondition
	return marshalWithExtras(alias(c), c.Extra, diagnosticConditionFields)
}

// UnmarshalJSON retains unknown recovery-program fields.
func (p *RecoveryProgram) UnmarshalJSON(data []byte) error {
	type alias RecoveryProgram
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	p.Extra = extrasFrom(data, recoveryProgramFields)
	decoded.Extra = p.Extra
	*p = RecoveryProgram(decoded)
	return nil
}

// MarshalJSON restores unknown recovery-program fields.
func (p RecoveryProgram) MarshalJSON() ([]byte, error) {
	type alias RecoveryProgram
	return marshalWithExtras(alias(p), p.Extra, recoveryProgramFields)
}

// UnmarshalJSON retains unknown facility-action fields.
func (a *FacilityActionConfig) UnmarshalJSON(data []byte) error {
	type alias FacilityActionConfig
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	a.Extra = extrasFrom(data, facilityActionFields)
	decoded.Extra = a.Extra
	*a = FacilityActionConfig(decoded)
	return nil
}

// MarshalJSON restores unknown facility-action fields.
func (a FacilityActionConfig) MarshalJSON() ([]byte, error) {
	type alias FacilityActionConfig
	return marshalWithExtras(alias(a), a.Extra, facilityActionFields)
}

// UnmarshalJSON accepts the version-1 transition-list object and the early
// direct-array fixture shape while retaining unknown fields in either form.
func (l *FacilityTransitionList) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) {
		var transitions []FacilityTransitionRequest
		if err := json.Unmarshal(data, &transitions); err != nil {
			return err
		}
		*l = FacilityTransitionList{Transitions: transitions}
		return nil
	}

	type alias FacilityTransitionList
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	l.Extra = extrasFrom(data, facilityTransitionListFields)
	decoded.Extra = l.Extra
	*l = FacilityTransitionList(decoded)
	return nil
}

// MarshalJSON restores unknown facility transition-list fields.
func (l FacilityTransitionList) MarshalJSON() ([]byte, error) {
	type alias FacilityTransitionList
	return marshalWithExtras(alias(l), l.Extra, facilityTransitionListFields)
}

// UnmarshalJSON retains unknown facility text-variant fields.
func (v *FacilityTextVariant) UnmarshalJSON(data []byte) error {
	type alias FacilityTextVariant
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	v.Extra = extrasFrom(data, facilityTextVariantFields)
	decoded.Extra = v.Extra
	*v = FacilityTextVariant(decoded)
	return nil
}

// MarshalJSON restores unknown facility text-variant fields.
func (v FacilityTextVariant) MarshalJSON() ([]byte, error) {
	type alias FacilityTextVariant
	return marshalWithExtras(alias(v), v.Extra, facilityTextVariantFields)
}

func fieldSet(fields ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		set[field] = struct{}{}
	}
	return set
}

func extrasFrom(data []byte, known map[string]struct{}) map[string]json.RawMessage {
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	for field := range known {
		delete(raw, field)
	}
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func marshalWithExtras(value any, extras map[string]json.RawMessage, known map[string]struct{}) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(extras) == 0 {
		return data, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	for field, value := range extras {
		if _, exists := known[field]; exists {
			return nil, fmt.Errorf("extra field %q shadows a known field", field)
		}
		raw[field] = value
	}
	return json.Marshal(raw)
}
