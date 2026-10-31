package playerconfig

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
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

// TestFailureTextsMatchOverseerCatalog keeps the Overseer's 'player-config'
// error catalog in step with the fixed failure texts in service.go. The Go
// texts stay English for the logs and the Overseer maps each one to Russian by
// its exact text, so changing a text means changing its catalog key too.
func TestFailureTextsMatchOverseerCatalog(t *testing.T) {
	t.Parallel()

	texts := failureTexts(t, "service.go")
	require.NotEmpty(t, texts, "service.go has no literal failure texts")

	source, err := os.ReadFile(filepath.Join("..", "..", "frontend", "overseer", "src", "overseer.js"))
	require.NoError(t, err)
	playerConfigKeys, ok := overseerCatalogKeys(string(source), "player-config")
	require.True(t, ok, "overseer.js has no 'player-config' block in OVERSEER_ERROR_MESSAGES")
	sessionKeys, _ := overseerCatalogKeys(string(source), "session")

	missing := slices.DeleteFunc(slices.Sorted(maps.Keys(texts)), func(text string) bool { return playerConfigKeys[text] })
	assert.Empty(t, missing, "player config failure texts with no 'player-config' byText entry")
	stale := slices.DeleteFunc(slices.Sorted(maps.Keys(playerConfigKeys)), func(key string) bool { return texts[key] })
	assert.Empty(t, stale, "'player-config' byText keys that match no player config failure text")
	shared := slices.DeleteFunc(slices.Sorted(maps.Keys(playerConfigKeys)), func(key string) bool { return !sessionKeys[key] })
	assert.Empty(t, shared, "byText keys in both the 'player-config' and 'session' areas")
}

// failureTexts returns the fixed failure texts in filename: every string
// literal passed to failure, and the message of every package-level
// errors.New sentinel passed as failure(sentinel.Error()), such as
// errStorageUnavailable. Dynamic texts such as err.Error() are skipped,
// because the Overseer shows them with the area's general message.
func failureTexts(t *testing.T, filename string) map[string]bool {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	require.NoError(t, err)

	// sentinels maps "<name>.Error()" to the text of each errors.New variable.
	sentinels := make(map[string]string)
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		for _, spec := range general.Specs {
			value := spec.(*ast.ValueSpec)
			for index, name := range value.Names[:len(value.Values)] {
				call, ok := value.Values[index].(*ast.CallExpr)
				if !ok || types.ExprString(call.Fun) != "errors.New" || len(call.Args) != 1 {
					continue
				}
				if text, ok := stringLiteral(t, call.Args[0]); ok {
					sentinels[name.Name+".Error()"] = text
				}
			}
		}
	}

	texts := make(map[string]bool)
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || types.ExprString(call.Fun) != "failure" || len(call.Args) != 1 {
			return true
		}
		if text, ok := stringLiteral(t, call.Args[0]); ok {
			texts[text] = true
		} else if text, ok := sentinels[types.ExprString(call.Args[0])]; ok {
			texts[text] = true
		}
		return true
	})
	return texts
}

// stringLiteral returns the unquoted value of expression when it is a string
// literal.
func stringLiteral(t *testing.T, expression ast.Expr) (string, bool) {
	t.Helper()
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	text, err := strconv.Unquote(literal.Value)
	require.NoError(t, err)
	return text, true
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
