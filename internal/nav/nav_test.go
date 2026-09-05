package nav

import (
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestDefault(t *testing.T) {
	t.Parallel()

	want := domain.NavState{Path: []string{"root"}, Mode: "list"}
	assert.Equal(t, want, Default())
}

func TestApplyAction(t *testing.T) {
	t.Parallel()

	tree := navigationTree()
	tests := []struct {
		name   string
		state  domain.NavState
		action string
		nodeID string
		want   domain.NavState
	}{
		{
			name:   "enter direct folder child",
			state:  Default(),
			action: "enter",
			nodeID: "docs",
			want:   domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
		{
			name:   "reject descendant folder that is not a direct child",
			state:  Default(),
			action: "enter",
			nodeID: "nested",
			want:   Default(),
		},
		{
			name:   "reject non-folder enter target",
			state:  Default(),
			action: "enter",
			nodeID: "root-entry",
			want:   Default(),
		},
		{
			name:   "open direct entry child and clear command",
			state:  navState([]string{"root"}, "list", "", "root-command"),
			action: "entry",
			nodeID: "root-entry",
			want:   navState([]string{"root"}, "entry", "root-entry", ""),
		},
		{
			name:   "reject entry outside current folder",
			state:  Default(),
			action: "entry",
			nodeID: "report",
			want:   Default(),
		},
		{
			name:   "run direct command child",
			state:  Default(),
			action: "command",
			nodeID: "root-command",
			want:   navState([]string{"root"}, "list", "", "root-command"),
		},
		{
			name:   "reject command outside current folder",
			state:  Default(),
			action: "command",
			nodeID: "read",
			want:   Default(),
		},
		{
			name:   "back closes entry before leaving folder",
			state:  navState([]string{"root", "docs"}, "entry", "report", ""),
			action: "back",
			want:   domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
		{
			name:   "back leaves folder and clears command",
			state:  navState([]string{"root", "docs"}, "list", "", "read"),
			action: "back",
			want:   Default(),
		},
		{
			name:   "back never escapes root",
			state:  Default(),
			action: "back",
			want:   Default(),
		},
		{
			name:   "back at root closes command result",
			state:  navState([]string{"root"}, "list", "", "root-command"),
			action: "back",
			want:   Default(),
		},
		{
			name:   "unknown action is a no-op",
			state:  Default(),
			action: "launch",
			nodeID: "root-command",
			want:   Default(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, ApplyAction(test.state, tree, test.action, test.nodeID))
		})
	}
}

func TestRevalidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state domain.NavState
		tree  domain.ContentNode
		want  domain.NavState
	}{
		{
			name:  "keep valid direct-child entry",
			state: navState([]string{"root", "docs"}, "entry", "report", ""),
			tree:  navigationTree(),
			want:  navState([]string{"root", "docs"}, "entry", "report", ""),
		},
		{
			name:  "truncate path at first missing folder",
			state: navState([]string{"root", "docs", "missing"}, "list", "", "read"),
			tree:  navigationTree(),
			want:  domain.NavState{Path: []string{"root", "docs"}, Mode: "list", CommandNodeID: new("read")},
		},
		{
			name:  "reject a non-folder path component",
			state: navState([]string{"root", "root-entry"}, "entry", "root-entry", ""),
			tree:  navigationTree(),
			want:  navState([]string{"root"}, "entry", "root-entry", ""),
		},
		{
			name:  "drop deleted entry",
			state: navState([]string{"root", "docs"}, "entry", "report", ""),
			tree:  treeWithout("report"),
			want:  domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
		{
			name:  "drop entry moved outside current folder",
			state: navState([]string{"root", "docs"}, "entry", "report", ""),
			tree:  treeWithReportMovedToArchive(),
			want:  domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
		{
			name:  "drop command moved outside current folder",
			state: navState([]string{"root", "docs"}, "list", "", "read"),
			tree:  treeWithReadMovedToArchive(),
			want:  domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
		{
			name:  "entry mode requires an entry id",
			state: domain.NavState{Path: []string{"root", "docs"}, Mode: "entry"},
			tree:  navigationTree(),
			want:  domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, Revalidate(test.state, test.tree))
		})
	}
}

func TestApplyActionRejectsAuthoritativelyUnavailableCommand(t *testing.T) {
	t.Parallel()

	tree := navigationTree()
	setCommandAvailability(&tree, "root-command", new(false))
	assert.Equal(t, Default(), ApplyAction(Default(), tree, "command", "root-command"))

	setCommandAvailability(&tree, "root-command", new(true))
	assert.Equal(t,
		navState([]string{"root"}, "list", "", "root-command"),
		ApplyAction(Default(), tree, "command", "root-command"),
	)

	// Absence preserves the version-1 behavior for ordinary commands.
	setCommandAvailability(&tree, "root-command", nil)
	assert.Equal(t,
		navState([]string{"root"}, "list", "", "root-command"),
		ApplyAction(Default(), tree, "command", "root-command"),
	)
}

func TestRevalidateRepairsHiddenCurrentAndTargetToNearestValidParent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state domain.NavState
		tree  domain.ContentNode
		want  domain.NavState
	}{
		{
			name:  "hidden current folder falls back to visible parent",
			state: navState([]string{"root", "docs", "nested"}, "entry", "deep-entry", ""),
			tree:  treeWithout("nested"),
			want:  domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
		{
			name:  "hidden open entry returns to its parent menu",
			state: navState([]string{"root", "docs"}, "entry", "report", ""),
			tree:  treeWithout("report"),
			want:  domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
		{
			name:  "hidden selected command returns to its parent menu",
			state: navState([]string{"root", "docs"}, "list", "", "read"),
			tree:  treeWithout("read"),
			want:  domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
		{
			name:  "unavailable selected command returns to its parent menu",
			state: navState([]string{"root", "docs"}, "list", "", "read"),
			tree: func() domain.ContentNode {
				tree := navigationTree()
				setCommandAvailability(&tree, "read", new(false))
				return tree
			}(),
			want: domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, Revalidate(test.state, test.tree))
		})
	}

	hiddenTargetTree := treeWithout("archive")
	assert.Equal(t, Default(), ApplyAction(Default(), hiddenTargetTree, "enter", "archive"))
}

func TestBackAndAcknowledgementRemainAvailableWhenCommandsAreBlocked(t *testing.T) {
	t.Parallel()

	tree := navigationTree()
	setCommandAvailability(&tree, "root-command", new(false))
	setCommandAvailability(&tree, "read", new(false))

	assert.Equal(t,
		Default(),
		ApplyAction(navState([]string{"root", "docs"}, "list", "", "read"), tree, "back", ""),
		"Back must leave a valid folder even when command execution is blocked",
	)
	assert.Equal(t,
		Default(),
		ApplyAction(navState([]string{"root"}, "list", "", "root-command"), tree, "back", ""),
		"Back must acknowledge an unavailable command result at the root",
	)
	assert.Equal(t,
		domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		ApplyAction(navState([]string{"root", "docs"}, "entry", "report", ""), tree, "back", ""),
		"Back must close an entry while command execution is blocked",
	)
}

func TestStableFolderLookupAndReturnRestoration(t *testing.T) {
	t.Parallel()

	tree := navigationTree()
	path, ok := FindFolderPath(tree, "nested")
	assert.True(t, ok)
	assert.Equal(t, []string{"root", "docs", "nested"}, path)

	// Stable-ID lookup follows the folder to its current location after an edit.
	moved := tree
	nested := moved.Children[0].Children[2]
	moved.Children[0].Children = moved.Children[0].Children[:2]
	moved.Children[1].Children = append(moved.Children[1].Children, nested)
	assert.Equal(t, domain.NavState{Path: []string{"root", "archive", "nested"}, Mode: "list"}, RestoreFolder(moved, "nested", []string{"root", "docs"}))

	// If the exact folder is gone, the nearest surviving authored ancestor wins.
	withoutNested := treeWithout("nested")
	assert.Equal(t, domain.NavState{Path: []string{"root", "docs"}, Mode: "list"}, RestoreFolder(withoutNested, "nested", []string{"root", "docs"}))
	withoutDocs := treeWithout("docs")
	assert.Equal(t, Default(), RestoreFolder(withoutDocs, "nested", []string{"root", "docs"}))
	assert.Equal(t, Default(), RestoreFolder(tree, "missing", nil))
}

func TestRestoreFolderUsesStableIdentityThenNearestSurvivingAncestor(t *testing.T) {
	t.Parallel()

	tree := navigationTree()
	// Rename does not change a stable folder identity.
	tree.Children[0].Children[2].Name = "RENAMED"
	assert.Equal(t, domain.NavState{Path: []string{"root", "docs", "nested"}, Mode: "list"},
		RestoreFolder(tree, "nested", []string{"root", "docs"}))

	// Moving the saved folder wins over its historical ancestry.
	nested := tree.Children[0].Children[2]
	tree.Children[0].Children = tree.Children[0].Children[:2]
	tree.Children[1].Children = append(tree.Children[1].Children, nested)
	assert.Equal(t, domain.NavState{Path: []string{"root", "archive", "nested"}, Mode: "list"},
		RestoreFolder(tree, "nested", []string{"root", "docs"}))

	// Deleting the saved folder falls back from the closest saved ancestor to root.
	removeNode(&tree, "nested")
	assert.Equal(t, domain.NavState{Path: []string{"root", "docs"}, Mode: "list"},
		RestoreFolder(tree, "nested", []string{"root", "docs"}))
	removeNode(&tree, "docs")
	assert.Equal(t, Default(), RestoreFolder(tree, "nested", []string{"root", "docs"}))
}

func navigationTree() domain.ContentNode {
	return domain.ContentNode{
		ID: "root", Type: domain.NodeFolder, Name: "ROOT",
		Children: []domain.ContentNode{
			{
				ID: "docs", Type: domain.NodeFolder, Name: "DOCS",
				Children: []domain.ContentNode{
					{ID: "report", Type: domain.NodeEntry, Name: "REPORT", Description: "Report"},
					{ID: "read", Type: domain.NodeCommand, Name: "READ", Text: "Reading"},
					{
						ID: "nested", Type: domain.NodeFolder, Name: "NESTED",
						Children: []domain.ContentNode{
							{ID: "deep-entry", Type: domain.NodeEntry, Name: "DEEP", Description: "Deep"},
						},
					},
				},
			},
			{
				ID: "archive", Type: domain.NodeFolder, Name: "ARCHIVE",
				Children: []domain.ContentNode{
					{ID: "old-entry", Type: domain.NodeEntry, Name: "OLD", Description: "Old"},
				},
			},
			{ID: "root-entry", Type: domain.NodeEntry, Name: "WELCOME", Description: "Welcome"},
			{ID: "root-command", Type: domain.NodeCommand, Name: "STATUS", Text: "Online"},
		},
	}
}

func treeWithout(nodeID string) domain.ContentNode {
	tree := navigationTree()
	removeNode(&tree, nodeID)
	return tree
}

func treeWithReportMovedToArchive() domain.ContentNode {
	tree := treeWithout("report")
	tree.Children[1].Children = append(tree.Children[1].Children, domain.ContentNode{
		ID: "report", Type: domain.NodeEntry, Name: "REPORT", Description: "Moved report",
	})
	return tree
}

func treeWithReadMovedToArchive() domain.ContentNode {
	tree := treeWithout("read")
	tree.Children[1].Children = append(tree.Children[1].Children, domain.ContentNode{
		ID: "read", Type: domain.NodeCommand, Name: "READ", Text: "Moved command",
	})
	return tree
}

func removeNode(parent *domain.ContentNode, nodeID string) bool {
	for index := range parent.Children {
		if parent.Children[index].ID == nodeID {
			parent.Children = append(parent.Children[:index], parent.Children[index+1:]...)
			return true
		}
		if removeNode(&parent.Children[index], nodeID) {
			return true
		}
	}
	return false
}

func setCommandAvailability(tree *domain.ContentNode, nodeID string, available *bool) bool {
	if tree.ID == nodeID && tree.Type == domain.NodeCommand {
		tree.Available = available
		return true
	}
	for index := range tree.Children {
		if setCommandAvailability(&tree.Children[index], nodeID, available) {
			return true
		}
	}
	return false
}

func navState(path []string, mode, viewEntryID, commandNodeID string) domain.NavState {
	state := domain.NavState{Path: path, Mode: mode}
	if viewEntryID != "" {
		state.ViewEntryID = new(viewEntryID)
	}
	if commandNodeID != "" {
		state.CommandNodeID = new(commandNodeID)
	}
	return state
}
