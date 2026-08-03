package cmd

import (
	"testing"

	"github.com/github/gh-stack/internal/config"
	"github.com/github/gh-stack/internal/git"
	"github.com/github/gh-stack/internal/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoin_MergesAdjacentTrackedStacks(t *testing.T) {
	tmpDir := t.TempDir()
	sf := &stack.StackFile{SchemaVersion: 1, Stacks: []stack.Stack{
		{
			Trunk: stack.BranchRef{Branch: "main", Head: "main-head"},
			Branches: []stack.BranchRef{{
				Branch: "benchmark", Head: "benchmark-head", Base: "main-head",
				PullRequest: &stack.PullRequestRef{Number: 86},
			}},
		},
		{
			Trunk: stack.BranchRef{Branch: "benchmark", Head: "benchmark-head"},
			Branches: []stack.BranchRef{{
				Branch: "dataset", Head: "dataset-head", Base: "benchmark-head",
				PullRequest: &stack.PullRequestRef{Number: 87},
			}},
		},
	}}
	require.NoError(t, stack.Save(tmpDir, sf))
	restore := git.SetOps(&git.MockOps{GitDirFn: func() (string, error) { return tmpDir, nil }})
	defer restore()
	cfg, _, _ := config.NewTestConfig()

	err := runJoin(cfg, "dataset")

	require.NoError(t, err)
	got, err := stack.Load(tmpDir)
	require.NoError(t, err)
	require.Len(t, got.Stacks, 1)
	assert.Equal(t, "main", got.Stacks[0].Trunk.Branch)
	assert.Equal(t, []string{"benchmark", "dataset"}, got.Stacks[0].BranchNames())
	assert.Equal(t, 86, got.Stacks[0].Branches[0].PullRequest.Number)
	assert.Equal(t, 87, got.Stacks[0].Branches[1].PullRequest.Number)
}

func TestJoin_RequiresParentToBeTopOfItsStack(t *testing.T) {
	tmpDir := t.TempDir()
	sf := &stack.StackFile{SchemaVersion: 1, Stacks: []stack.Stack{
		{
			Trunk:    stack.BranchRef{Branch: "main"},
			Branches: []stack.BranchRef{{Branch: "benchmark"}, {Branch: "other"}},
		},
		{
			Trunk:    stack.BranchRef{Branch: "benchmark"},
			Branches: []stack.BranchRef{{Branch: "dataset"}},
		},
	}}
	require.NoError(t, stack.Save(tmpDir, sf))
	restore := git.SetOps(&git.MockOps{GitDirFn: func() (string, error) { return tmpDir, nil }})
	defer restore()
	cfg, _, _ := config.NewTestConfig()

	err := runJoin(cfg, "dataset")

	assert.ErrorIs(t, err, ErrInvalidArgs)
}
