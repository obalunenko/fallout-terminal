package session

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overseerCatalogEntry matches one trimmed byText line of
// OVERSEER_ERROR_MESSAGES, `'<exact Go text>': '<message>',`, and captures the
// Go text.
var overseerCatalogEntry = regexp.MustCompile(`^'([^']*)': '[^']*',$`)

// TestSessionFailureTextsMatchOverseerCatalog keeps the Overseer's 'session'
// error catalog in step with the fixed failure texts in service.go. The Go
// texts stay English for the logs and the Overseer maps each one to Russian by
// its exact text, so changing a text means changing its catalog key too.
func TestSessionFailureTextsMatchOverseerCatalog(t *testing.T) {
	t.Parallel()

	texts := sessionFailureTexts(t, "service.go")
	require.NotEmpty(t, texts, "service.go has no literal sessionFailure or saveFailure texts")

	source, err := os.ReadFile(filepath.Join("..", "..", "frontend", "overseer", "src", "overseer.js"))
	require.NoError(t, err)
	sessionKeys, ok := overseerCatalogKeys(string(source), "session")
	require.True(t, ok, "overseer.js has no 'session' block in OVERSEER_ERROR_MESSAGES")
	playerConfigKeys, _ := overseerCatalogKeys(string(source), "player-config")

	missing := slices.DeleteFunc(slices.Sorted(maps.Keys(texts)), func(text string) bool { return sessionKeys[text] })
	assert.Empty(t, missing, "session failure texts with no 'session' byText entry")
	stale := slices.DeleteFunc(slices.Sorted(maps.Keys(sessionKeys)), func(key string) bool { return texts[key] })
	assert.Empty(t, stale, "'session' byText keys that match no session failure text")
	shared := slices.DeleteFunc(slices.Sorted(maps.Keys(sessionKeys)), func(key string) bool { return !playerConfigKeys[key] })
	assert.Empty(t, shared, "byText keys in both the 'session' and 'player-config' areas")
}

// sessionFailureTexts returns the fixed failure texts in filename: every string
// literal passed to sessionFailure, and every string literal passed as the last
// argument of saveFailure. Dynamic texts such as err.Error() are skipped,
// because the Overseer shows them with the area's general message.
func sessionFailureTexts(t *testing.T, filename string) map[string]bool {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	require.NoError(t, err)

	texts := make(map[string]bool)
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		var argument ast.Expr
		switch name.Name {
		case "sessionFailure":
			argument = call.Args[0]
		case "saveFailure":
			argument = call.Args[len(call.Args)-1]
		default:
			return true
		}
		literal, ok := argument.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		text, err := strconv.Unquote(literal.Value)
		require.NoError(t, err)
		texts[text] = true
		return true
	})
	return texts
}

// overseerCatalogKeys returns the byText keys of one area block of
// OVERSEER_ERROR_MESSAGES in source, and whether that block exists. It relies
// on the layout that contracts/overseer-ui.md «Catalog shape» fixes: the
// catalog closes with `});` at the start of a line, an area block runs from
// `  '<area>': Object.freeze({` to the next `  }),`, and each byText entry
// sits on its own line.
func overseerCatalogKeys(source, area string) (map[string]bool, bool) {
	_, catalog, ok := strings.Cut(source, "\nconst OVERSEER_ERROR_MESSAGES = Object.freeze({")
	if !ok {
		return nil, false
	}
	catalog, _, _ = strings.Cut(catalog, "\n});")
	_, block, ok := strings.Cut(catalog, "\n  '"+area+"': Object.freeze({")
	if !ok {
		return nil, false
	}
	block, _, _ = strings.Cut(block, "\n  }),")

	keys := make(map[string]bool)
	for line := range strings.Lines(block) {
		if match := overseerCatalogEntry.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			keys[match[1]] = true
		}
	}
	return keys, true
}
