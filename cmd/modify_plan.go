package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/github/gh-stack/internal/config"
	ghapi "github.com/github/gh-stack/internal/github"
	"github.com/github/gh-stack/internal/modify"
	"github.com/github/gh-stack/internal/stack"
	"github.com/github/gh-stack/internal/tui/modifyview"
	"github.com/github/gh-stack/internal/tui/stackview"
)

type modifyPlan struct {
	Branches []modifyPlanBranch `json:"branches"`
}

type modifyPlanBranch struct {
	Name    string `json:"name"`
	Action  string `json:"action,omitempty"`
	NewName string `json:"newName,omitempty"`
}

func runModifyPlan(cfg *config.Config, planPath string) error {
	plan, err := readModifyPlan(cfg.In, planPath)
	if err != nil {
		cfg.Errorf("invalid modify plan: %s", err)
		return ErrInvalidArgs
	}

	result, err := checkModifyPreconditions(cfg, false)
	if err != nil {
		return err
	}
	if result.Stack.IsFullyMerged() {
		cfg.Errorf("all branches in this stack have been merged")
		return ErrSilent
	}

	nodes, err := buildPlanNodes(cfg, result.Stack, result.CurrentBranch, result.PRDetails, plan)
	if err != nil {
		cfg.Errorf("invalid modify plan: %s", err)
		return ErrInvalidArgs
	}
	if err := modify.ValidatePlan(result.StackFile, result.Stack, nodes); err != nil {
		cfg.Errorf("invalid modify plan: %s", err)
		return ErrInvalidArgs
	}

	applyResult, conflict, applyErr := modify.ApplyPlan(
		cfg,
		result.GitDir,
		result.Stack,
		result.StackFile,
		nodes,
		result.CurrentBranch,
		updateBaseSHAs,
	)
	if conflict != nil {
		cfg.Warningf("Rebasing %s caused a conflict", conflict.Branch)
		printConflictDetailsWithContinue(cfg, conflict.Branch, "gh stack modify --continue")
		cfg.Printf("Or restore the stack with `%s`", cfg.ColorCyan("gh stack modify --abort"))
		return ErrConflict
	}
	if applyErr != nil {
		cfg.Errorf("failed to apply modifications: %s", applyErr)
		return ErrSilent
	}

	printModifySuccess(cfg, applyResult)
	return nil
}

func readModifyPlan(input io.Reader, planPath string) (*modifyPlan, error) {
	reader := input
	var file *os.File
	if planPath != "-" {
		var err error
		file, err = os.Open(planPath)
		if err != nil {
			return nil, fmt.Errorf("read plan: %w", err)
		}
		defer file.Close()
		reader = file
	}

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var plan modifyPlan
	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("plan must contain one JSON document")
	}
	if len(plan.Branches) == 0 {
		return nil, fmt.Errorf("plan contains no branches")
	}

	seen := make(map[string]bool, len(plan.Branches))
	for _, branch := range plan.Branches {
		if branch.Name == "" {
			return nil, fmt.Errorf("plan contains a branch with no name")
		}
		if seen[branch.Name] {
			return nil, fmt.Errorf("branch %q appears more than once", branch.Name)
		}
		seen[branch.Name] = true

		switch branch.Action {
		case "", "drop", "fold":
			if branch.NewName != "" {
				return nil, fmt.Errorf("branch %q sets newName without a rename", branch.Name)
			}
		case "rename":
			if branch.NewName == "" {
				return nil, fmt.Errorf("rename for %q has no newName", branch.Name)
			}
		default:
			return nil, fmt.Errorf("branch %q has unknown action %q", branch.Name, branch.Action)
		}
	}

	return &plan, nil
}

func buildPlanNodes(
	cfg *config.Config,
	s *stack.Stack,
	currentBranch string,
	prDetails map[string]*ghapi.PRDetails,
	plan *modifyPlan,
) ([]modifyview.ModifyBranchNode, error) {
	loaded := stackview.LoadBranchNodes(cfg, s, currentBranch, prDetails)
	byName := make(map[string]stackview.BranchNode, len(loaded))
	positions := make(map[string]int, len(loaded))
	for i, node := range loaded {
		byName[node.Ref.Branch] = node
		positions[node.Ref.Branch] = i
	}

	nodes := make([]modifyview.ModifyBranchNode, 0, len(plan.Branches))
	for _, branch := range plan.Branches {
		node, ok := byName[branch.Name]
		if !ok {
			return nil, fmt.Errorf("branch %q is not part of this stack", branch.Name)
		}

		modifyNode := modifyview.ModifyBranchNode{
			BranchNode:       node,
			OriginalPosition: positions[branch.Name],
		}
		switch branch.Action {
		case "drop":
			modifyNode.PendingAction = &modifyview.PendingAction{Type: modifyview.ActionDrop}
			modifyNode.Removed = true
		case "fold":
			modifyNode.PendingAction = &modifyview.PendingAction{Type: modifyview.ActionFoldDown}
			modifyNode.Removed = true
		case "rename":
			modifyNode.PendingAction = &modifyview.PendingAction{
				Type:    modifyview.ActionRename,
				NewName: branch.NewName,
			}
		}
		nodes = append(nodes, modifyNode)
	}

	return nodes, nil
}
