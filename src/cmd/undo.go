package cmd

import (
	"fmt"

	"github.com/git-town/git-town/v9/src/config"
	"github.com/git-town/git-town/v9/src/execute"
	"github.com/git-town/git-town/v9/src/flags"
	"github.com/git-town/git-town/v9/src/messages"
	"github.com/git-town/git-town/v9/src/persistence"
	"github.com/git-town/git-town/v9/src/runstate"
	"github.com/git-town/git-town/v9/src/runvm"
	"github.com/spf13/cobra"
)

const undoDesc = "Undoes the last run git-town command"

func undoCmd() *cobra.Command {
	addDebugFlag, readDebugFlag := flags.Debug()
	cmd := cobra.Command{
		Use:     "undo",
		GroupID: "errors",
		Args:    cobra.NoArgs,
		Short:   undoDesc,
		Long:    long(undoDesc),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUndo(readDebugFlag(cmd))
		},
	}
	addDebugFlag(&cmd)
	return &cmd
}

func runUndo(debug bool) error {
	repo, err := execute.OpenRepo(execute.OpenRepoArgs{
		Debug:            debug,
		DryRun:           false,
		OmitBranchNames:  false,
		ValidateIsOnline: false,
		ValidateGitRepo:  true,
	})
	if err != nil {
		return err
	}
	config, err := determineUndoConfig(&repo)
	if err != nil {
		return err
	}
	undoRunState, err := determineUndoRunstate(&repo)
	if err != nil {
		return err
	}
	return runvm.Execute(runvm.ExecuteArgs{
		RunState:  undoRunState,
		Run:       &repo.Runner,
		Connector: nil,
		Lineage:   config.lineage,
		RootDir:   repo.RootDir,
	})
}

type undoConfig struct {
	lineage config.Lineage
}

func determineUndoConfig(repo *execute.OpenRepoResult) (*undoConfig, error) {
	lineage := repo.Runner.Config.Lineage()
	_, _, err := execute.LoadBranches(execute.LoadBranchesArgs{
		Repo:                  repo,
		Fetch:                 false,
		HandleUnfinishedState: false,
		Lineage:               lineage,
		ValidateIsConfigured:  true,
		ValidateNoOpenChanges: false,
	})
	if err != nil {
		return nil, err
	}
	return &undoConfig{
		lineage: lineage,
	}, nil
}

func determineUndoRunstate(repo *execute.OpenRepoResult) (*runstate.RunState, error) {
	runState, err := persistence.Load(repo.RootDir)
	if err != nil {
		return nil, fmt.Errorf(messages.RunstateLoadProblem, err)
	}
	if runState == nil || runState.IsUnfinished() {
		return nil, fmt.Errorf(messages.UndoNothingToDo)
	}
	undoRunState := runState.CreateUndoRunState()
	return &undoRunState, nil
}
