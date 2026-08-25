package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/internal/domain"
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
