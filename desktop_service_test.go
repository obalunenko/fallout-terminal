package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDesktopServiceReplaceTerminalGroupsIsTransparentTrustedForward(t *testing.T) {
	t.Parallel()

	forward := (*desktopService).ReplaceTerminalGroups
	require.NotNil(t, forward)
	payload := TerminalGroupReplacementPayload{
		TerminalGroups: []domain.TerminalGroup{
			{ID: "group-a", Name: "Group A", TerminalIDs: []string{"terminal-a"}},
		},
		ExpectedSessionRevision:      17,
		ExpectedCoordinationRevision: 29,
	}
	require.Equal(t, "group-a", payload.TerminalGroups[0].ID)
	require.Equal(t, uint64(17), payload.ExpectedSessionRevision)
	require.Equal(t, uint64(29), payload.ExpectedCoordinationRevision)

	file, err := parser.ParseFile(token.NewFileSet(), "desktop_service.go", nil, 0)
	require.NoError(t, err)
	var method *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Recv != nil && candidate.Name.Name == "ReplaceTerminalGroups" {
			method = candidate
			break
		}
	}
	require.NotNil(t, method, "desktop allowlist must expose ReplaceTerminalGroups")
	require.Len(t, method.Type.Params.List, 1)
	require.Len(t, method.Type.Params.List[0].Names, 1)
	require.Equal(t, "payload", method.Type.Params.List[0].Names[0].Name)
	payloadType, ok := method.Type.Params.List[0].Type.(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "TerminalGroupReplacementPayload", payloadType.Name)
	require.Len(t, method.Type.Results.List, 1)
	resultType, ok := method.Type.Results.List[0].Type.(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "TerminalGroupReplacementResult", resultType.Name)
	require.Len(t, method.Body.List, 1, "desktop method must remain a transparent core forward")
	returned, ok := method.Body.List[0].(*ast.ReturnStmt)
	require.True(t, ok)
	require.Len(t, returned.Results, 1)
	call, ok := returned.Results[0].(*ast.CallExpr)
	require.True(t, ok)
	require.Len(t, call.Args, 1)
	argument, ok := call.Args[0].(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "payload", argument.Name)
	coreCall, ok := call.Fun.(*ast.SelectorExpr)
	require.True(t, ok)
	require.Equal(t, "ReplaceTerminalGroups", coreCall.Sel.Name)
	core, ok := coreCall.X.(*ast.SelectorExpr)
	require.True(t, ok)
	require.Equal(t, "core", core.Sel.Name)
	receiver, ok := core.X.(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "service", receiver.Name)
}

func TestDesktopServiceApplicationUpdateMethodsAreTransparentCoreForwards(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parameter string
		result    string
	}{
		{name: "GetApplicationUpdateStatus", result: "ApplicationUpdateSnapshot"},
		{name: "ResolveApplicationUpdateOffer", parameter: "ApplicationUpdateOfferDecisionPayload", result: "ApplicationUpdateCommandResult"},
		{name: "ResolveApplicationUpdateRestart", parameter: "ApplicationUpdateRestartDecisionPayload", result: "ApplicationUpdateCommandResult"},
	}

	file, err := parser.ParseFile(token.NewFileSet(), "desktop_service.go", nil, 0)
	require.NoError(t, err)
	methods := make(map[string]*ast.FuncDecl)
	for _, declaration := range file.Decls {
		method, ok := declaration.(*ast.FuncDecl)
		if ok && method.Recv != nil {
			methods[method.Name.Name] = method
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			method := methods[test.name]
			require.NotNil(t, method, "desktop allowlist must expose %s", test.name)
			if test.parameter == "" {
				require.Empty(t, method.Type.Params.List)
			} else {
				require.Len(t, method.Type.Params.List, 1)
				require.Len(t, method.Type.Params.List[0].Names, 1)
				require.Equal(t, "payload", method.Type.Params.List[0].Names[0].Name)
				parameter, ok := method.Type.Params.List[0].Type.(*ast.Ident)
				require.True(t, ok)
				require.Equal(t, test.parameter, parameter.Name)
			}
			require.Len(t, method.Type.Results.List, 1)
			result, ok := method.Type.Results.List[0].Type.(*ast.Ident)
			require.True(t, ok)
			require.Equal(t, test.result, result.Name)
			require.Len(t, method.Body.List, 1, "%s must remain a transparent forward", test.name)
			returned, ok := method.Body.List[0].(*ast.ReturnStmt)
			require.True(t, ok)
			require.Len(t, returned.Results, 1)
			call, ok := returned.Results[0].(*ast.CallExpr)
			require.True(t, ok)
			selector, ok := call.Fun.(*ast.SelectorExpr)
			require.True(t, ok)
			require.Equal(t, test.name, selector.Sel.Name)
			core, ok := selector.X.(*ast.SelectorExpr)
			require.True(t, ok)
			service, ok := core.X.(*ast.Ident)
			require.True(t, ok)
			require.Equal(t, "service", service.Name)
			require.Equal(t, "core", core.Sel.Name)
			if test.parameter == "" {
				require.Empty(t, call.Args)
			} else {
				require.Len(t, call.Args, 1)
				argument, ok := call.Args[0].(*ast.Ident)
				require.True(t, ok)
				require.Equal(t, "payload", argument.Name)
			}
		})
	}
}

type recordingLogDirectoryOpener struct {
	paths []string
	err   error
}

func (opener *recordingLogDirectoryOpener) OpenDirectory(path string) error {
	opener.paths = append(opener.paths, path)
	return opener.err
}

func TestDesktopServiceOpensInjectedLogLocationWithoutFrontendPath(t *testing.T) {
	t.Parallel()
	opener := &recordingLogDirectoryOpener{}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		LogDirectoryOpener: opener,
		LogDirectory:       "/fixed/application/logs",
		ActiveLogPath:      func() string { return "/fixed/application/logs/application-current.log" },
	})
	result := newDesktopService(app).OpenLogLocation()
	require.True(t, result.OK)
	assert.Equal(t, []string{"/fixed/application/logs"}, opener.paths)
	assert.Equal(t, "/fixed/application/logs/application-current.log", result.ActiveLogPath)
}

func TestOpenLogLocationReturnsSafeManualFallback(t *testing.T) {
	t.Parallel()
	opener := &recordingLogDirectoryOpener{err: errors.New("raw native detail must not escape")}
	app := NewAppWithDependencies(t.Context(), AppDependencies{
		LogDirectoryOpener: opener,
		LogDirectory:       "/fixed/application/logs",
	})
	result := app.OpenLogLocation()
	require.False(t, result.OK)
	assert.Equal(t, "/fixed/application/logs", result.DirectoryPath)
	assert.Equal(t, "Could not open the application log directory.", result.Error)
	assert.NotContains(t, result.Error, "raw native detail")
}
