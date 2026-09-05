// Package nav implements transport-independent shared-navigation transitions.
package nav

import (
	"slices"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

const (
	modeList  = "list"
	modeEntry = "entry"
)

// Default returns the canonical navigation state at the terminal root.
func Default() domain.NavState {
	return domain.NavState{Path: []string{"root"}, Mode: modeList}
}

// ApplyAction applies one player navigation request to the shared state.
// Targets are accepted only when they are direct children of the current
// folder and have the type required by the action.
func ApplyAction(state domain.NavState, tree domain.ContentNode, action, nodeID string) domain.NavState {
	next := cloneState(state)
	folder, ok := currentFolder(tree, next.Path)

	switch action {
	case "enter":
		if !ok {
			return next
		}
		node := directChild(folder, nodeID, domain.NodeFolder)
		if node == nil {
			return next
		}
		next.Path = append(next.Path, node.ID)
		next.Mode = modeList
		next.ViewEntryID = nil
		next.CommandNodeID = nil

	case "back":
		if next.Mode == modeEntry {
			next.Mode = modeList
			next.ViewEntryID = nil
		} else if len(next.Path) > 1 {
			next.Path = next.Path[:len(next.Path)-1]
			next.CommandNodeID = nil
		} else if next.CommandNodeID != nil {
			next.CommandNodeID = nil
		}

	case "entry":
		if !ok {
			return next
		}
		node := directChild(folder, nodeID, domain.NodeEntry)
		if node == nil {
			return next
		}
		next.Mode = modeEntry
		next.ViewEntryID = new(node.ID)
		next.CommandNodeID = nil

	case "command":
		if !ok {
			return next
		}
		node := directChild(folder, nodeID, domain.NodeCommand)
		if !availableCommand(node) {
			return next
		}
		next.CommandNodeID = new(node.ID)
	}

	return next
}

// Revalidate repairs a shared navigation state after the content tree changes.
// The path is truncated at its first invalid folder and leaf references are
// retained only when they still identify direct children of the resulting
// current folder.
func Revalidate(state domain.NavState, tree domain.ContentNode) domain.NavState {
	path, folder := revalidatedPath(tree, state.Path)
	next := domain.NavState{
		Path: path,
		Mode: state.Mode,
	}

	if next.Mode != modeEntry {
		next.Mode = modeList
	} else if state.ViewEntryID != nil && directChild(folder, *state.ViewEntryID, domain.NodeEntry) != nil {
		next.ViewEntryID = new(*state.ViewEntryID)
	} else {
		next.Mode = modeList
	}

	if state.CommandNodeID != nil {
		command := directChild(folder, *state.CommandNodeID, domain.NodeCommand)
		if availableCommand(command) {
			next.CommandNodeID = new(*state.CommandNodeID)
		}
	}

	return next
}

// FindFolderPath resolves a stable folder ID to its current ancestry.
func FindFolderPath(tree domain.ContentNode, folderID string) ([]string, bool) {
	if tree.ID != "root" || tree.Type != domain.NodeFolder || folderID == "" {
		return nil, false
	}
	var visit func(domain.ContentNode, []string) ([]string, bool)
	visit = func(folder domain.ContentNode, path []string) ([]string, bool) {
		if folder.ID == folderID {
			return append([]string(nil), path...), true
		}
		for _, child := range folder.Children {
			if child.Type != domain.NodeFolder {
				continue
			}
			if found, ok := visit(child, append(path, child.ID)); ok {
				return found, true
			}
		}
		return nil, false
	}
	return visit(tree, []string{"root"})
}

// RestoreFolder follows an exact stable folder to its current location, then
// falls back through the saved nearest-to-root ancestry and finally root.
func RestoreFolder(tree domain.ContentNode, folderID string, ancestorFolderIDs []string) domain.NavState {
	if path, ok := FindFolderPath(tree, folderID); ok {
		return domain.NavState{Path: path, Mode: modeList}
	}
	for _, ancestorFolderID := range slices.Backward(ancestorFolderIDs) {
		if path, ok := FindFolderPath(tree, ancestorFolderID); ok {
			return domain.NavState{Path: path, Mode: modeList}
		}
	}
	return Default()
}

func revalidatedPath(tree domain.ContentNode, path []string) ([]string, domain.ContentNode) {
	valid := []string{"root"}
	current := tree

	for index := 1; index < len(path); index++ {
		next := directChild(current, path[index], domain.NodeFolder)
		if next == nil {
			break
		}
		valid = append(valid, next.ID)
		current = *next
	}

	return valid, current
}

func currentFolder(tree domain.ContentNode, path []string) (domain.ContentNode, bool) {
	if len(path) == 0 || path[0] != "root" || tree.ID != "root" || tree.Type != domain.NodeFolder {
		return domain.ContentNode{}, false
	}

	current := tree
	for index := 1; index < len(path); index++ {
		next := directChild(current, path[index], domain.NodeFolder)
		if next == nil {
			return domain.ContentNode{}, false
		}
		current = *next
	}

	return current, true
}

func directChild(parent domain.ContentNode, nodeID, nodeType string) *domain.ContentNode {
	for index := range parent.Children {
		child := &parent.Children[index]
		if child.ID == nodeID && child.Type == nodeType {
			return child
		}
	}
	return nil
}

func availableCommand(command *domain.ContentNode) bool {
	return command != nil && (command.Available == nil || *command.Available)
}

func cloneState(state domain.NavState) domain.NavState {
	clone := state
	clone.Path = append([]string(nil), state.Path...)
	if state.ViewEntryID != nil {
		clone.ViewEntryID = new(*state.ViewEntryID)
	}
	if state.CommandNodeID != nil {
		clone.CommandNodeID = new(*state.CommandNodeID)
	}
	return clone
}
