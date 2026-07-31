package modify

import (
	"fmt"

	"github.com/github/gh-stack/internal/git"
	"github.com/github/gh-stack/internal/stack"
	"github.com/github/gh-stack/internal/tui/modifyview"
)

// ValidatePlan checks a staged plan before it changes Git refs.
func ValidatePlan(sf *stack.StackFile, s *stack.Stack, nodes []modifyview.ModifyBranchNode) error {
	seen := make(map[string]bool, len(nodes))
	finalNames := make(map[string]bool, len(nodes))
	order := make([]string, 0, len(nodes))
	remaining := 0
	hasAction := false

	for i, node := range nodes {
		name := node.Ref.Branch
		if node.Ref.IsMerged() {
			if node.PendingAction != nil {
				return fmt.Errorf("merged branch %q cannot be modified", name)
			}
			continue
		}

		if !node.IsInserted {
			if s.IndexOf(name) < 0 {
				return fmt.Errorf("branch %q is not part of this stack", name)
			}
			if seen[name] {
				return fmt.Errorf("branch %q appears more than once", name)
			}
			seen[name] = true
			order = append(order, name)
		}

		if node.PendingAction != nil {
			hasAction = true
			if err := validateAction(nodes, i); err != nil {
				return err
			}
		} else if node.Removed || node.IsInserted {
			return fmt.Errorf("branch %q has incomplete action state", name)
		}

		if node.Removed {
			continue
		}
		finalName := name
		if node.PendingAction != nil && node.PendingAction.Type == modifyview.ActionRename {
			finalName = node.PendingAction.NewName
		}
		if err := validateNewBranchName(sf, s, name, finalName, node.IsInserted); err != nil {
			return err
		}
		if finalNames[finalName] {
			return fmt.Errorf("branch %q appears more than once in the result", finalName)
		}
		finalNames[finalName] = true
		if !node.IsInserted {
			remaining++
		}
	}

	for _, branch := range s.Branches {
		if !branch.IsMerged() && !seen[branch.Branch] {
			return fmt.Errorf("branch %q is missing", branch.Branch)
		}
	}
	if remaining == 0 {
		return fmt.Errorf("a stack must keep at least one branch")
	}
	if hasAction && orderChanged(s, order) {
		return fmt.Errorf("a plan cannot reorder branches and apply other actions")
	}
	return nil
}

func validateAction(nodes []modifyview.ModifyBranchNode, index int) error {
	node := nodes[index]
	action := node.PendingAction.Type
	switch action {
	case modifyview.ActionDrop:
		if !node.Removed {
			return fmt.Errorf("drop for %q is not marked as removed", node.Ref.Branch)
		}
	case modifyview.ActionRename:
		if node.Removed || node.PendingAction.NewName == "" {
			return fmt.Errorf("rename for %q is invalid", node.Ref.Branch)
		}
	case modifyview.ActionFoldDown, modifyview.ActionFoldUp:
		if !node.Removed {
			return fmt.Errorf("fold for %q is not marked as removed", node.Ref.Branch)
		}
		step := -1
		if action == modifyview.ActionFoldUp {
			step = 1
		}
		for i := index + step; i >= 0 && i < len(nodes); i += step {
			if nodes[i].Removed || nodes[i].Ref.IsMerged() {
				continue
			}
			if nodes[i].IsInserted {
				return fmt.Errorf("branch %q cannot fold into an inserted branch", node.Ref.Branch)
			}
			return nil
		}
		return fmt.Errorf("branch %q has no branch to fold into", node.Ref.Branch)
	case modifyview.ActionInsertBelow, modifyview.ActionInsertAbove:
		if !node.IsInserted || node.Removed {
			return fmt.Errorf("insert for %q is invalid", node.Ref.Branch)
		}
	default:
		return fmt.Errorf("branch %q has unknown action %q", node.Ref.Branch, action)
	}
	return nil
}

func validateNewBranchName(sf *stack.StackFile, s *stack.Stack, oldName, newName string, inserted bool) error {
	if newName == oldName && !inserted {
		return nil
	}
	if err := git.ValidateRefName(newName); err != nil {
		return fmt.Errorf("invalid branch name %q: %w", newName, err)
	}
	if git.BranchExists(newName) {
		return fmt.Errorf("branch %q already exists", newName)
	}
	if err := sf.ValidateNoDuplicateBranch(newName); err != nil {
		return err
	}
	return nil
}

func orderChanged(s *stack.Stack, order []string) bool {
	active := make([]string, 0, len(s.Branches))
	for _, branch := range s.Branches {
		if !branch.IsMerged() {
			active = append(active, branch.Branch)
		}
	}
	if len(active) != len(order) {
		return true
	}
	for i := range active {
		if active[i] != order[i] {
			return true
		}
	}
	return false
}
