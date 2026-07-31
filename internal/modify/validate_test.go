package modify

import (
	"errors"
	"testing"

	"github.com/github/gh-stack/internal/git"
	"github.com/github/gh-stack/internal/stack"
	"github.com/github/gh-stack/internal/tui/modifyview"
	"github.com/github/gh-stack/internal/tui/stackview"
	"github.com/stretchr/testify/assert"
)

func TestValidatePlan_RejectsUnsafePlans(t *testing.T) {
	s := &stack.Stack{
		Trunk:    stack.BranchRef{Branch: "main"},
		Branches: []stack.BranchRef{{Branch: "one"}, {Branch: "two"}},
	}
	sf := &stack.StackFile{Stacks: []stack.Stack{*s}}
	nodes := func() []modifyview.ModifyBranchNode {
		return []modifyview.ModifyBranchNode{
			{BranchNode: stackview.BranchNode{Ref: s.Branches[0]}},
			{BranchNode: stackview.BranchNode{Ref: s.Branches[1]}},
		}
	}

	restore := git.SetOps(&git.MockOps{
		ValidateRefNameFn: func(name string) error {
			if name == "bad name" {
				return errors.New("invalid ref")
			}
			return nil
		},
	})
	defer restore()

	t.Run("missing branch", func(t *testing.T) {
		assert.ErrorContains(t, ValidatePlan(sf, s, nodes()[:1]), "two")
	})

	t.Run("fold without target", func(t *testing.T) {
		plan := nodes()
		plan[0].PendingAction = &modifyview.PendingAction{Type: modifyview.ActionFoldDown}
		plan[0].Removed = true
		assert.ErrorContains(t, ValidatePlan(sf, s, plan), "no branch to fold into")
	})

	t.Run("invalid rename", func(t *testing.T) {
		plan := nodes()
		plan[0].PendingAction = &modifyview.PendingAction{
			Type: modifyview.ActionRename, NewName: "bad name",
		}
		assert.ErrorContains(t, ValidatePlan(sf, s, plan), "invalid branch name")
	})

	t.Run("action with reorder", func(t *testing.T) {
		plan := nodes()
		plan[0], plan[1] = plan[1], plan[0]
		plan[0].PendingAction = &modifyview.PendingAction{
			Type: modifyview.ActionRename, NewName: "renamed",
		}
		assert.ErrorContains(t, ValidatePlan(sf, s, plan), "cannot reorder")
	})
}
