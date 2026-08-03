package cmd

import (
	"github.com/github/gh-stack/internal/config"
	"github.com/github/gh-stack/internal/git"
	"github.com/github/gh-stack/internal/modify"
	"github.com/github/gh-stack/internal/stack"
	"github.com/spf13/cobra"
)

// JoinCmd joins a stack to the local stack that owns its trunk.
func JoinCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "join <branch>",
		Short: "Join a stack to the parent stack directly below it",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runJoin(cfg, args[0])
		},
	}
}

func runJoin(cfg *config.Config, branch string) error {
	gitDir, err := git.GitDir()
	if err != nil {
		cfg.Errorf("not a git repository")
		return ErrNotInStack
	}
	if err := modify.CheckStateGuard(gitDir); err != nil {
		cfg.Errorf("%s", err)
		return ErrModifyRecovery
	}
	sf, err := stack.Load(gitDir)
	if err != nil {
		cfg.Errorf("failed to load stack state: %s", err)
		return ErrNotInStack
	}

	childIdx := -1
	for i := range sf.Stacks {
		if sf.Stacks[i].IndexOf(branch) >= 0 {
			if childIdx >= 0 {
				cfg.Errorf("branch %q is owned by multiple stacks", branch)
				return ErrDisambiguate
			}
			childIdx = i
		}
	}
	if childIdx < 0 {
		cfg.Errorf("branch %q is not owned by a stack", branch)
		return ErrNotInStack
	}

	child := sf.Stacks[childIdx]
	parentIdx := -1
	for i := range sf.Stacks {
		if i != childIdx && sf.Stacks[i].IndexOf(child.Trunk.Branch) >= 0 {
			if parentIdx >= 0 {
				cfg.Errorf("parent branch %q is owned by multiple stacks", child.Trunk.Branch)
				return ErrDisambiguate
			}
			parentIdx = i
		}
	}
	if parentIdx < 0 {
		cfg.Errorf("stack containing %q has no tracked parent stack", branch)
		return ErrNotInStack
	}

	parent := sf.Stacks[parentIdx]
	if len(parent.Branches) == 0 || parent.Branches[len(parent.Branches)-1].Branch != child.Trunk.Branch {
		cfg.Errorf("cannot join: parent branch %q is not the top of its stack", child.Trunk.Branch)
		return ErrInvalidArgs
	}
	if child.ID != "" && child.ID != parent.ID {
		cfg.Errorf("cannot join: the upper stack is already linked as a different GitHub stack")
		return ErrInvalidArgs
	}

	joined := parent
	joined.Branches = append(append([]stack.BranchRef{}, parent.Branches...), child.Branches...)
	newStacks := make([]stack.Stack, 0, len(sf.Stacks)-1)
	for i := range sf.Stacks {
		switch i {
		case childIdx:
			continue
		case parentIdx:
			newStacks = append(newStacks, joined)
		default:
			newStacks = append(newStacks, sf.Stacks[i])
		}
	}
	sf.Stacks = newStacks
	if err := stack.Save(gitDir, sf); err != nil {
		return handleSaveError(cfg, err)
	}

	cfg.Successf("Joined %s to the parent stack (%d PR layers)", branch, len(joined.Branches))
	return nil
}
