// Package hack implements the server-authoritative terminal hacking game.
package hack

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

const (
	boardRows       = 16
	boardRowWidth   = 12
	columnChars     = boardRows * boardRowWidth
	wordGap         = 4
	placementTries  = 300
	maximumAttempts = 4
)

var fillerPool = []byte("!@#$%^&*_+-=\\|;:'\",./?~")

var patternPairs = [...]string{"()", "[]", "{}", "<>"}

var delimiterClosers = [...]byte{')', ']', '}', '>'}

// Random is the small random-number boundary used by board generation.
type Random interface {
	Intn(limit int) int
}

type systemRandom struct{}

func (systemRandom) Intn(limit int) int {
	if limit <= 1 {
		return 0
	}
	var bytes [8]byte
	if _, err := cryptorand.Read(bytes[:]); err != nil {
		return 0
	}
	return int(binary.LittleEndian.Uint64(bytes[:]) % uint64(limit))
}

// GenerateBoard creates a fresh private hacking aggregate. Random and words
// are injectable; nil values use the system random source and built-in bank.
func GenerateBoard(generationID string, level int, random Random, words WordSource) *domain.HackState {
	if strings.TrimSpace(generationID) == "" {
		return nil
	}
	random = randomOrDefault(random)
	wordLength := levelWordLength(level)
	wordCount := 11 + clamp(level, 1, 5)
	if words == nil {
		words = WordBank{Random: random}
	}
	candidates := normalizeCandidates(words.PickWords(wordLength, wordCount), wordLength, wordCount)
	if len(candidates) < wordCount {
		fallback := WordBank{Random: random}.PickWords(wordLength, wordCount)
		candidates = supplementCandidates(candidates, fallback, wordCount)
	}
	if len(candidates) == 0 {
		return nil
	}

	secretWord := candidates[safeIntn(random, len(candidates))]
	for range placementTries {
		state := generateBoardAttempt(generationID, level, wordLength, secretWord, candidates, random)
		if state == nil {
			continue
		}
		if classifyFinalBoard(state).publishable() {
			return state
		}
	}
	return nil
}

func generateBoardAttempt(generationID string, level, wordLength int, secretWord string, candidates []string, random Random) *domain.HackState {
	state := &domain.HackState{
		GenerationID: generationID,
		Level:        level,
		WordLength:   wordLength,
		AttemptsMax:  maximumAttempts,
		AttemptsLeft: maximumAttempts,
		SecretWord:   secretWord,
		WordsByID:    make(map[string]domain.HackCandidate, len(candidates)),
		UsedPatterns: make(map[domain.HackPatternIdentity]struct{}),
		Log:          []string{},
	}

	wordScope := sha256.Sum256([]byte(generationID))
	columnA := newColumnBuilder(fmt.Sprintf("%x-A", wordScope[:8]), random)
	columnB := newColumnBuilder(fmt.Sprintf("%x-B", wordScope[:8]), random)
	for index, text := range candidates {
		builder := columnA
		if index%2 != 0 {
			builder = columnB
		}
		id, ok := builder.place(text, -1)
		if !ok {
			return nil
		}
		state.WordsByID[id] = domain.HackCandidate{Text: text}
	}
	if !placeInterruptedCandidateSpan(columnA, columnB, random) {
		return nil
	}

	patternTarget := 3 + safeIntn(random, 4)
	pairOffset := safeIntn(random, len(patternPairs))
	for index := range patternTarget {
		pair := patternPairs[(pairOffset+index)%len(patternPairs)]
		interiorLength := 0
		if index == 0 {
			interiorLength = 1 + safeIntn(random, 3)
		} else if safeIntn(random, 2) == 1 {
			interiorLength = 1 + safeIntn(random, 2)
		}
		primary, secondary := columnA, columnB
		if index%2 != 0 {
			primary, secondary = columnB, columnA
		}
		if !primary.placePattern(pair, interiorLength) && !secondary.placePattern(pair, interiorLength) {
			return nil
		}
	}
	for index := range patternTarget {
		primary, secondary := columnA, columnB
		if index%2 != 0 {
			primary, secondary = columnB, columnA
		}
		closer := delimiterClosers[(pairOffset+index)%len(delimiterClosers)]
		if !primary.placeDelimiterDecoy(closer) && !secondary.placeDelimiterDecoy(closer) {
			return nil
		}
	}

	state.Columns = []domain.HackColumn{columnA.finish(), columnB.finish()}
	return state
}

// ApplyGuess applies a candidate ID or a "column:character" filler target.
// Unknown, stale, and terminal-state actions are ignored.
func ApplyGuess(state *domain.HackState, targetID string) {
	if state == nil || state.Solved || state.Failed {
		return
	}

	if candidate, ok := state.WordsByID[targetID]; ok {
		pushLog(state, candidate.Text)
		matches := countMatches(candidate.Text, state.SecretWord)
		if matches == state.WordLength {
			state.Solved = true
			pushSuccessLog(state)
			return
		}
		spendAttempt(state, matches)
		return
	}

	columnIndex, characterIndex, ok := parseFillerTarget(targetID)
	if !ok || columnIndex >= len(state.Columns) {
		return
	}
	column := &state.Columns[columnIndex]
	if characterIndex >= len(column.Text) || containsWord(column.Words, characterIndex) {
		return
	}
	for _, pattern := range discoverPatternSpans(state.GenerationID, state.Columns) {
		if pattern.ColumnIndex == columnIndex && pattern.AbsoluteStart == characterIndex {
			return
		}
	}
	pushLog(state, string(column.Text[characterIndex]))
	spendAttempt(state, 0)
}

// ForceSuccess solves a currently active puzzle without spending an attempt.
func ForceSuccess(state *domain.HackState) {
	if state == nil || state.Solved || state.Failed {
		return
	}
	pushLog(state, state.SecretWord)
	state.Solved = true
	pushSuccessLog(state)
}

// ApplyPattern consumes one currently valid coordinate span and applies
// exactly one shared effect. Invalid, stale, repeated, and terminal-state
// actions leave the aggregate unchanged.
func ApplyPattern(state *domain.HackState, requestedID string, random Random) bool {
	if state == nil || state.Solved || state.Failed {
		return false
	}
	var requested *domain.HackPattern
	for _, pattern := range discoverPatternSpans(state.GenerationID, state.Columns) {
		if patternID(pattern.Identity) == requestedID {
			copy := pattern
			requested = &copy
			break
		}
	}
	if requested == nil {
		return false
	}
	if state.UsedPatterns == nil {
		state.UsedPatterns = make(map[domain.HackPatternIdentity]struct{})
	}
	if _, used := state.UsedPatterns[requested.Identity]; used {
		return false
	}
	state.UsedPatterns[requested.Identity] = struct{}{}
	random = randomOrDefault(random)

	outcome := safeIntn(random, 100)
	decoys := incorrectCandidateIDs(state)
	if outcome >= 80 || len(decoys) == 0 {
		state.AttemptsLeft = state.AttemptsMax
		pushLog(state, "Попытки восстановлены.")
		return true
	}

	dudID := decoys[safeIntn(random, len(decoys))]
	dotCandidate(state, dudID)
	pushLog(state, "Ложное слово удалено.")
	return true
}

// PublicState creates a detached client-safe projection of private state.
func PublicState(state *domain.HackState) *domain.PublicHackState {
	if state == nil {
		return nil
	}
	public := &domain.PublicHackState{
		Level:        state.Level,
		WordLength:   state.WordLength,
		AttemptsMax:  state.AttemptsMax,
		AttemptsLeft: state.AttemptsLeft,
		Solved:       state.Solved,
		Failed:       state.Failed,
		Log:          cloneStrings(state.Log),
		Columns:      make([]domain.HackColumn, len(state.Columns)),
		Patterns:     discoverPatterns(state),
	}
	for index, column := range state.Columns {
		public.Columns[index] = column
		public.Columns[index].Addresses = append([]string(nil), column.Addresses...)
		public.Columns[index].Words = append([]domain.HackWord(nil), column.Words...)
	}
	return public
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

type columnBuilder struct {
	prefix string
	random Random
	chars  []byte
	used   []bool
	words  []domain.HackWord
	nextID int
}

func newColumnBuilder(prefix string, random Random) *columnBuilder {
	return &columnBuilder{
		prefix: prefix,
		random: random,
		chars:  make([]byte, columnChars),
		used:   make([]bool, columnChars),
		nextID: 1,
	}
}

func (builder *columnBuilder) place(text string, requestedStart int) (string, bool) {
	start := requestedStart
	if start < 0 {
		start = builder.randomStart(len(text))
	}
	if !builder.canPlace(start, len(text)) {
		return "", false
	}
	id := fmt.Sprintf("%s%d", builder.prefix, builder.nextID)
	builder.nextID++
	copy(builder.chars[start:start+len(text)], text)
	for index := start; index < start+len(text); index++ {
		builder.used[index] = true
	}
	builder.words = append(builder.words, domain.HackWord{
		ID: id, Start: start, Length: len(text),
	})
	return id, true
}

func placeInterruptedCandidateSpan(first, second *columnBuilder, random Random) bool {
	builders := []*columnBuilder{first, second}
	startBuilder := safeIntn(random, len(builders))
	pairOffset := safeIntn(random, len(patternPairs))
	for builderIndex := range builders {
		builder := builders[(startBuilder+builderIndex)%len(builders)]
		if builder.placeInterruptedSpan(patternPairs[(pairOffset+builderIndex)%len(patternPairs)]) {
			return true
		}
	}
	return false
}

func (builder *columnBuilder) placeInterruptedSpan(pair string) bool {
	if len(pair) != 2 || len(builder.words) == 0 {
		return false
	}
	wordOffset := safeIntn(builder.random, len(builder.words))
	for index := range builder.words {
		word := builder.words[(wordOffset+index)%len(builder.words)]
		start := word.Start - 1
		end := word.Start + word.Length
		if start < 0 || end >= len(builder.used) || start/boardRowWidth != end/boardRowWidth || builder.used[start] || builder.used[end] {
			continue
		}
		builder.chars[start] = pair[0]
		builder.chars[end] = pair[1]
		builder.used[start] = true
		builder.used[end] = true
		return true
	}
	return false
}

func (builder *columnBuilder) placePattern(pair string, interiorLength int) bool {
	if len(pair) != 2 || interiorLength < 0 {
		return false
	}
	spanLength := interiorLength + 2
	if spanLength > boardRowWidth {
		return false
	}
	startsPerRow := boardRowWidth - spanLength + 1
	limit := boardRows * startsPerRow
	for range placementTries {
		candidate := safeIntn(builder.random, limit)
		row := candidate / startsPerRow
		column := candidate % startsPerRow
		start := row*boardRowWidth + column
		if builder.canPlacePattern(start, spanLength) {
			builder.writePattern(start, pair, interiorLength)
			return true
		}
	}
	for row := range boardRows {
		for column := range startsPerRow {
			start := row*boardRowWidth + column
			if builder.canPlacePattern(start, spanLength) {
				builder.writePattern(start, pair, interiorLength)
				return true
			}
		}
	}
	return false
}

func (builder *columnBuilder) canPlacePattern(start, length int) bool {
	if start < 0 || length < 2 || start+length > len(builder.used) || start/boardRowWidth != (start+length-1)/boardRowWidth {
		return false
	}
	for index := start; index < start+length; index++ {
		if builder.used[index] {
			return false
		}
	}
	return true
}

func (builder *columnBuilder) writePattern(start int, pair string, interiorLength int) {
	builder.chars[start] = pair[0]
	for index := range interiorLength {
		builder.chars[start+1+index] = fillerPool[safeIntn(builder.random, len(fillerPool))]
	}
	builder.chars[start+interiorLength+1] = pair[1]
	for index := start; index < start+interiorLength+2; index++ {
		builder.used[index] = true
	}
}

func (builder *columnBuilder) placeDelimiterDecoy(delimiter byte) bool {
	for range placementTries {
		index := safeIntn(builder.random, len(builder.used))
		if !builder.used[index] {
			builder.chars[index] = delimiter
			builder.used[index] = true
			return true
		}
	}
	for index := range builder.used {
		if !builder.used[index] {
			builder.chars[index] = delimiter
			builder.used[index] = true
			return true
		}
	}
	return false
}

func (builder *columnBuilder) randomStart(length int) int {
	limit := columnChars - length + 1
	for range placementTries {
		candidate := safeIntn(builder.random, limit)
		if builder.canPlace(candidate, length) {
			return candidate
		}
	}
	for candidate := range limit {
		if builder.canPlace(candidate, length) {
			return candidate
		}
	}
	return -1
}

func (builder *columnBuilder) canPlace(start, length int) bool {
	if start < 0 || length <= 0 || start+length > len(builder.chars) {
		return false
	}
	from := max(0, start-wordGap)
	to := min(len(builder.chars), start+length+wordGap)
	for index := from; index < to; index++ {
		if builder.used[index] {
			return false
		}
	}
	return true
}

func (builder *columnBuilder) finish() domain.HackColumn {
	for index := range builder.chars {
		if !builder.used[index] {
			builder.chars[index] = fillerPool[safeIntn(builder.random, len(fillerPool))]
		}
	}
	return domain.HackColumn{
		Addresses: generateAddresses(boardRows, builder.random),
		Text:      string(builder.chars),
		Words:     append([]domain.HackWord(nil), builder.words...),
	}
}

func generateAddresses(count int, random Random) []string {
	address := safeIntn(random, 0x4000) + 0xC000
	steps := [...]int{0x0C, 0x10, 0x14, 0x18}
	step := steps[safeIntn(random, len(steps))]
	addresses := make([]string, count)
	for index := range addresses {
		addresses[index] = fmt.Sprintf("0x%04X", address&0xFFFF)
		address += step
	}
	return addresses
}

func normalizeCandidates(input []string, length, count int) []string {
	result := make([]string, 0, min(count, len(input)))
	seen := make(map[string]struct{}, len(input))
	for _, raw := range input {
		word := strings.ToUpper(raw)
		if len(word) != length || strings.IndexFunc(word, func(value rune) bool { return !isASCIIAlpha(value) }) >= 0 {
			continue
		}
		if _, duplicate := seen[word]; duplicate {
			continue
		}
		seen[word] = struct{}{}
		result = append(result, word)
		if len(result) == count {
			break
		}
	}
	return result
}

func supplementCandidates(candidates, fallback []string, count int) []string {
	result := append([]string(nil), candidates...)
	seen := make(map[string]struct{}, len(result))
	for _, word := range result {
		seen[word] = struct{}{}
	}
	for _, word := range fallback {
		if len(result) == count {
			break
		}
		if _, duplicate := seen[word]; duplicate {
			continue
		}
		seen[word] = struct{}{}
		result = append(result, word)
	}
	return result
}

func dotCandidate(state *domain.HackState, id string) {
	for columnIndex := range state.Columns {
		column := &state.Columns[columnIndex]
		for wordIndex, word := range column.Words {
			if word.ID != id {
				continue
			}
			column.Text = column.Text[:word.Start] + strings.Repeat(".", word.Length) + column.Text[word.Start+word.Length:]
			column.Words = append(column.Words[:wordIndex], column.Words[wordIndex+1:]...)
			delete(state.WordsByID, id)
			return
		}
	}
}

func incorrectCandidateIDs(state *domain.HackState) []string {
	ids := make([]string, 0, len(state.WordsByID))
	for _, column := range state.Columns {
		for _, word := range column.Words {
			candidate, exists := state.WordsByID[word.ID]
			if exists && candidate.Text != state.SecretWord {
				ids = append(ids, word.ID)
			}
		}
	}
	return ids
}

func discoverPatterns(state *domain.HackState) []domain.PublicHackPattern {
	spans := discoverPatternSpans(state.GenerationID, state.Columns)
	patterns := make([]domain.PublicHackPattern, len(spans))
	for index, span := range spans {
		_, wasUsed := state.UsedPatterns[span.Identity]
		patterns[index] = domain.PublicHackPattern{
			ID:    patternID(span.Identity),
			Row:   span.Identity.Row,
			Start: span.Identity.Start,
			End:   span.Identity.End,
			Used:  wasUsed,
		}
	}
	return patterns
}

func discoverPatternSpans(generationID string, columns []domain.HackColumn) []domain.HackPattern {
	closers := map[byte]byte{'(': ')', '[': ']', '{': '}', '<': '>'}
	patterns := make([]domain.HackPattern, 0)
	renderedRowBase := 0
	for columnIndex, column := range columns {
		text := column.Text
		for rowStart := 0; rowStart < len(text); rowStart += boardRowWidth {
			rowEnd := min(rowStart+boardRowWidth, len(text))
			for start := rowStart; start < rowEnd; start++ {
				closer, isOpening := closers[text[start]]
				if !isOpening {
					continue
				}
				relativeEnd := strings.IndexByte(text[start+1:rowEnd], closer)
				if relativeEnd < 0 {
					continue
				}
				end := start + 1 + relativeEnd
				if strings.IndexFunc(text[start+1:end], isASCIIAlpha) >= 0 {
					continue
				}
				patterns = append(patterns, domain.HackPattern{
					Identity: domain.HackPatternIdentity{
						GenerationID: generationID,
						Row:          renderedRowBase + rowStart/boardRowWidth,
						Start:        start - rowStart,
						End:          end - rowStart,
					},
					ColumnIndex:   columnIndex,
					AbsoluteStart: start,
					AbsoluteEnd:   end,
					Pair:          string([]byte{text[start], closer}),
				})
			}
		}
		renderedRowBase += (len(text) + boardRowWidth - 1) / boardRowWidth
	}
	return patterns
}

type boardPosition struct {
	row         int
	offset      int
	columnIndex int
	absolute    int
}

type finalBoardCamouflage struct {
	patterns                     []domain.HackPattern
	standaloneDelimiters         []boardPosition
	hasNonEmptyPattern           bool
	hasAlphabeticInterruptedSpan bool
	candidateRows                map[int]struct{}
	patternRows                  map[int]struct{}
	decoyRows                    map[int]struct{}
	ordinaryFillerRows           map[int]struct{}
}

func (camouflage finalBoardCamouflage) publishable() bool {
	patternCount := len(camouflage.patterns)
	return patternCount >= 3 && patternCount <= 6 &&
		len(camouflage.standaloneDelimiters) >= patternCount &&
		camouflage.hasNonEmptyPattern &&
		camouflage.hasAlphabeticInterruptedSpan &&
		len(camouflage.candidateRows) >= 2 &&
		len(camouflage.patternRows) >= 2 &&
		len(camouflage.decoyRows) >= 2 &&
		len(camouflage.ordinaryFillerRows) >= 2 &&
		rowIntervalsOverlapPairwise(camouflage.candidateRows, camouflage.patternRows, camouflage.decoyRows)
}

func classifyFinalBoard(state *domain.HackState) finalBoardCamouflage {
	camouflage := finalBoardCamouflage{
		candidateRows:      make(map[int]struct{}),
		patternRows:        make(map[int]struct{}),
		decoyRows:          make(map[int]struct{}),
		ordinaryFillerRows: make(map[int]struct{}),
	}
	if state == nil {
		return camouflage
	}

	camouflage.patterns = discoverPatternSpans(state.GenerationID, state.Columns)
	covered := make(map[[2]int]struct{})
	for _, pattern := range camouflage.patterns {
		camouflage.patternRows[pattern.Identity.Row] = struct{}{}
		if pattern.Identity.End-pattern.Identity.Start > 1 {
			camouflage.hasNonEmptyPattern = true
		}
		for offset := pattern.Identity.Start; offset <= pattern.Identity.End; offset++ {
			covered[[2]int{pattern.Identity.Row, offset}] = struct{}{}
		}
	}

	renderedRowBase := 0
	for columnIndex, column := range state.Columns {
		wordPositions := make(map[int]struct{})
		for _, word := range column.Words {
			for absolute := word.Start; absolute < word.Start+word.Length && absolute < len(column.Text); absolute++ {
				wordPositions[absolute] = struct{}{}
				camouflage.candidateRows[renderedRowBase+absolute/boardRowWidth] = struct{}{}
			}
		}

		for rowStart := 0; rowStart < len(column.Text); rowStart += boardRowWidth {
			rowEnd := min(rowStart+boardRowWidth, len(column.Text))
			row := renderedRowBase + rowStart/boardRowWidth
			if rowHasAlphabeticInterruptedSpan(column.Text[rowStart:rowEnd]) {
				camouflage.hasAlphabeticInterruptedSpan = true
			}
			for absolute := rowStart; absolute < rowEnd; absolute++ {
				value := column.Text[absolute]
				offset := absolute - rowStart
				if isDelimiter(value) {
					if _, inValidRange := covered[[2]int{row, offset}]; !inValidRange {
						camouflage.standaloneDelimiters = append(camouflage.standaloneDelimiters, boardPosition{
							row: row, offset: offset, columnIndex: columnIndex, absolute: absolute,
						})
						camouflage.decoyRows[row] = struct{}{}
					}
					continue
				}
				if _, isWord := wordPositions[absolute]; !isWord && !isASCIIAlpha(rune(value)) {
					camouflage.ordinaryFillerRows[row] = struct{}{}
				}
			}
		}
		renderedRowBase += (len(column.Text) + boardRowWidth - 1) / boardRowWidth
	}
	return camouflage
}

func rowHasAlphabeticInterruptedSpan(row string) bool {
	closers := map[byte]byte{'(': ')', '[': ']', '{': '}', '<': '>'}
	for start := 0; start < len(row); start++ {
		closer, isOpening := closers[row[start]]
		if !isOpening {
			continue
		}
		relativeEnd := strings.IndexByte(row[start+1:], closer)
		if relativeEnd < 0 {
			continue
		}
		end := start + 1 + relativeEnd
		if strings.IndexFunc(row[start+1:end], isASCIIAlpha) >= 0 {
			return true
		}
	}
	return false
}

func rowIntervalsOverlapPairwise(rowSets ...map[int]struct{}) bool {
	if len(rowSets) < 2 {
		return true
	}
	type interval struct{ low, high int }
	intervals := make([]interval, len(rowSets))
	for index, rows := range rowSets {
		if len(rows) == 0 {
			return false
		}
		first := true
		for row := range rows {
			if first || row < intervals[index].low {
				intervals[index].low = row
			}
			if first || row > intervals[index].high {
				intervals[index].high = row
			}
			first = false
		}
	}
	for left := range intervals {
		for right := left + 1; right < len(intervals); right++ {
			if intervals[left].high < intervals[right].low || intervals[right].high < intervals[left].low {
				return false
			}
		}
	}
	return true
}

func isDelimiter(value byte) bool {
	return strings.IndexByte("()[]{}<>", value) >= 0
}

func patternID(identity domain.HackPatternIdentity) string {
	raw := fmt.Sprintf("%s\x00%d\x00%d\x00%d", identity.GenerationID, identity.Row, identity.Start, identity.End)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func isASCIIAlpha(value rune) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func parseFillerTarget(target string) (int, int, bool) {
	parts := strings.Split(target, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, false
	}
	column, err := strconv.Atoi(parts[0])
	if err != nil || column < 0 {
		return 0, 0, false
	}
	character, err := strconv.Atoi(parts[1])
	if err != nil || character < 0 {
		return 0, 0, false
	}
	return column, character, true
}

func containsWord(words []domain.HackWord, index int) bool {
	for _, word := range words {
		if index >= word.Start && index < word.Start+word.Length {
			return true
		}
	}
	return false
}

func countMatches(candidate, secret string) int {
	limit := min(len(candidate), len(secret))
	matches := 0
	for index := range limit {
		if candidate[index] == secret[index] {
			matches++
		}
	}
	return matches
}

func spendAttempt(state *domain.HackState, matches int) {
	state.AttemptsLeft = max(0, state.AttemptsLeft-1)
	pushLog(state, "Отказ в доступе", fmt.Sprintf("%d/%d правильно.", matches, state.WordLength))
	if state.AttemptsLeft == 0 {
		state.Failed = true
	}
}

func pushSuccessLog(state *domain.HackState) {
	pushLog(state, "Точно!", "Пожалуйста,", "подождите", "входа в систему.")
}

func pushLog(state *domain.HackState, lines ...string) {
	for _, line := range lines {
		state.Log = append(state.Log, "> "+line)
	}
}

func levelWordLength(level int) int {
	if level >= 1 && level <= 5 {
		return level + 3
	}
	return 4
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}

func randomOrDefault(random Random) Random {
	if random == nil {
		return systemRandom{}
	}
	return random
}

func safeIntn(random Random, limit int) int {
	if limit <= 1 {
		return 0
	}
	value := random.Intn(limit)
	value %= limit
	if value < 0 {
		value += limit
	}
	return value
}
