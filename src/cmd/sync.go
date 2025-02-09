package cmd

import (
	"fmt"

	"github.com/git-town/git-town/v9/src/config"
	"github.com/git-town/git-town/v9/src/domain"
	"github.com/git-town/git-town/v9/src/execute"
	"github.com/git-town/git-town/v9/src/flags"
	"github.com/git-town/git-town/v9/src/runstate"
	"github.com/git-town/git-town/v9/src/runvm"
	"github.com/git-town/git-town/v9/src/steps"
	"github.com/git-town/git-town/v9/src/validate"
	"github.com/spf13/cobra"
)

const syncDesc = "Updates the current branch with all relevant changes"

const syncHelp = `
Synchronizes the current branch with the rest of the world.

When run on a feature branch
- syncs all ancestor branches
- pulls updates for the current branch
- merges the parent branch into the current branch
- pushes the current branch

When run on the main branch or a perennial branch
- pulls and pushes updates for the current branch
- pushes tags

If the repository contains an "upstream" remote,
syncs the main branch with its upstream counterpart.
You can disable this by running "git config %s false".`

func syncCmd() *cobra.Command {
	addDebugFlag, readDebugFlag := flags.Debug()
	addDryRunFlag, readDryRunFlag := flags.DryRun()
	addAllFlag, readAllFlag := flags.Bool("all", "a", "Sync all local branches")
	cmd := cobra.Command{
		Use:     "sync",
		GroupID: "basic",
		Args:    cobra.NoArgs,
		Short:   syncDesc,
		Long:    long(syncDesc, fmt.Sprintf(syncHelp, config.KeySyncUpstream)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(readAllFlag(cmd), readDryRunFlag(cmd), readDebugFlag(cmd))
		},
	}
	addAllFlag(&cmd)
	addDebugFlag(&cmd)
	addDryRunFlag(&cmd)
	return &cmd
}

func runSync(all, dryRun, debug bool) error {
	repo, err := execute.OpenRepo(execute.OpenRepoArgs{
		Debug:            debug,
		DryRun:           dryRun,
		OmitBranchNames:  false,
		ValidateIsOnline: false,
		ValidateGitRepo:  true,
	})
	if err != nil {
		return err
	}
	config, exit, err := determineSyncConfig(all, &repo)
	if err != nil || exit {
		return err
	}
	stepList, err := syncBranchesSteps(config)
	if err != nil {
		return err
	}
	runState := runstate.RunState{
		Command:     "sync",
		RunStepList: stepList,
	}
	return runvm.Execute(runvm.ExecuteArgs{
		RunState:  &runState,
		Run:       &repo.Runner,
		Connector: nil,
		Lineage:   config.lineage,
		RootDir:   repo.RootDir,
	})
}

type syncConfig struct {
	branches           domain.Branches
	branchesToSync     domain.BranchInfos
	hasOpenChanges     bool
	remotes            domain.Remotes
	isOffline          bool
	lineage            config.Lineage
	mainBranch         domain.LocalBranchName
	previousBranch     domain.LocalBranchName
	pullBranchStrategy config.PullBranchStrategy
	pushHook           bool
	shouldPushTags     bool
	shouldSyncUpstream bool
	syncStrategy       config.SyncStrategy
}

func determineSyncConfig(allFlag bool, repo *execute.OpenRepoResult) (*syncConfig, bool, error) {
	lineage := repo.Runner.Config.Lineage()
	branches, exit, err := execute.LoadBranches(execute.LoadBranchesArgs{
		Repo:                  repo,
		Fetch:                 true,
		HandleUnfinishedState: true,
		Lineage:               lineage,
		ValidateIsConfigured:  true,
		ValidateNoOpenChanges: false,
	})
	if err != nil || exit {
		return nil, exit, err
	}
	previousBranch := repo.Runner.Backend.PreviouslyCheckedOutBranch()
	hasOpenChanges, err := repo.Runner.Backend.HasOpenChanges()
	if err != nil {
		return nil, false, err
	}
	remotes, err := repo.Runner.Backend.Remotes()
	if err != nil {
		return nil, false, err
	}
	mainBranch := repo.Runner.Config.MainBranch()
	var branchNamesToSync domain.LocalBranchNames
	var shouldPushTags bool
	var configUpdated bool
	if allFlag {
		localBranches := branches.All.LocalBranches()
		configUpdated, err = validate.KnowsBranchesAncestors(validate.KnowsBranchesAncestorsArgs{
			AllBranches: localBranches,
			Backend:     &repo.Runner.Backend,
			BranchTypes: branches.Types,
			Lineage:     lineage,
			MainBranch:  mainBranch,
		})
		if err != nil {
			return nil, false, err
		}
		branchNamesToSync = localBranches.Names()
		shouldPushTags = true
	} else {
		configUpdated, err = validate.KnowsBranchAncestors(branches.Initial, validate.KnowsBranchAncestorsArgs{
			AllBranches:   branches.All,
			Backend:       &repo.Runner.Backend,
			BranchTypes:   branches.Types,
			DefaultBranch: mainBranch,
			Lineage:       lineage,
			MainBranch:    mainBranch,
		})
		if err != nil {
			return nil, false, err
		}
	}
	if configUpdated {
		lineage = repo.Runner.Config.Lineage() // reload after ancestry change
		branches.Types = repo.Runner.Config.BranchTypes()
	}
	if !allFlag {
		branchNamesToSync = domain.LocalBranchNames{branches.Initial}
		if configUpdated {
			repo.Runner.Config.Reload()
			branches.Types = repo.Runner.Config.BranchTypes()
		}
		shouldPushTags = !branches.Types.IsFeatureBranch(branches.Initial)
	}
	allBranchNamesToSync := lineage.BranchesAndAncestors(branchNamesToSync)
	syncStrategy, err := repo.Runner.Config.SyncStrategy()
	if err != nil {
		return nil, false, err
	}
	pushHook, err := repo.Runner.Config.PushHook()
	if err != nil {
		return nil, false, err
	}
	pullBranchStrategy, err := repo.Runner.Config.PullBranchStrategy()
	if err != nil {
		return nil, false, err
	}
	shouldSyncUpstream, err := repo.Runner.Config.ShouldSyncUpstream()
	if err != nil {
		return nil, false, err
	}
	branchesToSync, err := branches.All.Select(allBranchNamesToSync)
	return &syncConfig{
		branches:           branches,
		branchesToSync:     branchesToSync,
		hasOpenChanges:     hasOpenChanges,
		remotes:            remotes,
		isOffline:          repo.IsOffline,
		lineage:            lineage,
		mainBranch:         mainBranch,
		previousBranch:     previousBranch,
		pullBranchStrategy: pullBranchStrategy,
		pushHook:           pushHook,
		shouldPushTags:     shouldPushTags,
		shouldSyncUpstream: shouldSyncUpstream,
		syncStrategy:       syncStrategy,
	}, false, err
}

// syncBranchesSteps provides the step list for the "git sync" command.
func syncBranchesSteps(config *syncConfig) (runstate.StepList, error) {
	list := runstate.StepListBuilder{}
	for _, branch := range config.branchesToSync {
		syncBranchSteps(&list, syncBranchStepsArgs{
			branch:             branch,
			branchTypes:        config.branches.Types,
			remotes:            config.remotes,
			isOffline:          config.isOffline,
			lineage:            config.lineage,
			mainBranch:         config.mainBranch,
			pullBranchStrategy: config.pullBranchStrategy,
			pushBranch:         true,
			pushHook:           config.pushHook,
			shouldSyncUpstream: config.shouldSyncUpstream,
			syncStrategy:       config.syncStrategy,
		})
	}
	list.Add(&steps.CheckoutStep{Branch: config.branches.Initial})
	if config.remotes.HasOrigin() && config.shouldPushTags && !config.isOffline {
		list.Add(&steps.PushTagsStep{})
	}
	list.Wrap(runstate.WrapOptions{
		RunInGitRoot:     true,
		StashOpenChanges: config.hasOpenChanges,
		MainBranch:       config.mainBranch,
		InitialBranch:    config.branches.Initial,
		PreviousBranch:   config.previousBranch,
	})
	return list.Result()
}

// syncBranchSteps provides the steps to sync a particular branch.
func syncBranchSteps(list *runstate.StepListBuilder, args syncBranchStepsArgs) {
	isFeatureBranch := args.branchTypes.IsFeatureBranch(args.branch.LocalName)
	if !isFeatureBranch && !args.remotes.HasOrigin() {
		// perennial branch but no remote --> this branch cannot be synced
		return
	}
	list.Add(&steps.CheckoutStep{Branch: args.branch.LocalName})
	if isFeatureBranch {
		syncFeatureBranchSteps(list, args.branch, args.lineage, args.syncStrategy)
	} else {
		syncPerennialBranchSteps(list, syncPerennialBranchStepsArgs{
			branch:             args.branch,
			mainBranch:         args.mainBranch,
			pullBranchStrategy: args.pullBranchStrategy,
			shouldSyncUpstream: args.shouldSyncUpstream,
			hasUpstream:        args.remotes.HasUpstream(),
		})
	}
	if args.pushBranch && args.remotes.HasOrigin() && !args.isOffline {
		switch {
		case !args.branch.HasTrackingBranch():
			list.Add(&steps.CreateTrackingBranchStep{Branch: args.branch.LocalName, NoPushHook: false})
		case !isFeatureBranch:
			list.Add(&steps.PushCurrentBranchStep{CurrentBranch: args.branch.LocalName, NoPushHook: false, Undoable: false})
		default:
			pushFeatureBranchSteps(list, args.branch.LocalName, args.syncStrategy, args.pushHook)
		}
	}
}

type syncBranchStepsArgs struct {
	branch             domain.BranchInfo
	branchTypes        domain.BranchTypes
	remotes            domain.Remotes
	isOffline          bool
	lineage            config.Lineage
	mainBranch         domain.LocalBranchName
	pullBranchStrategy config.PullBranchStrategy
	pushBranch         bool
	pushHook           bool
	shouldSyncUpstream bool
	syncStrategy       config.SyncStrategy
}

// syncFeatureBranchSteps adds all the steps to sync the feature branch with the given name.
func syncFeatureBranchSteps(list *runstate.StepListBuilder, branch domain.BranchInfo, lineage config.Lineage, syncStrategy config.SyncStrategy) {
	if branch.HasTrackingBranch() {
		pullTrackingBranchOfCurrentFeatureBranchStep(list, branch.RemoteName, syncStrategy)
	}
	pullParentBranchOfCurrentFeatureBranchStep(list, lineage.Parent(branch.LocalName), syncStrategy)
}

// syncPerennialBranchSteps adds all the steps to sync the perennial branch with the given name.
func syncPerennialBranchSteps(list *runstate.StepListBuilder, args syncPerennialBranchStepsArgs) {
	if args.branch.HasTrackingBranch() {
		updateCurrentPerennialBranchStep(list, args.branch.RemoteName, args.pullBranchStrategy)
	}
	if args.branch.LocalName == args.mainBranch && args.hasUpstream && args.shouldSyncUpstream {
		list.Add(&steps.FetchUpstreamStep{Branch: args.mainBranch})
		list.Add(&steps.RebaseBranchStep{Branch: domain.NewBranchName("upstream/" + args.mainBranch.String())})
	}
}

type syncPerennialBranchStepsArgs struct {
	branch             domain.BranchInfo
	mainBranch         domain.LocalBranchName
	pullBranchStrategy config.PullBranchStrategy
	shouldSyncUpstream bool
	hasUpstream        bool
}

// pullTrackingBranchOfCurrentFeatureBranchStep adds the step to pull updates from the remote branch of the current feature branch into the current feature branch.
func pullTrackingBranchOfCurrentFeatureBranchStep(list *runstate.StepListBuilder, trackingBranch domain.RemoteBranchName, strategy config.SyncStrategy) {
	switch strategy {
	case config.SyncStrategyMerge:
		list.Add(&steps.MergeStep{Branch: trackingBranch.BranchName()})
	case config.SyncStrategyRebase:
		list.Add(&steps.RebaseBranchStep{Branch: trackingBranch.BranchName()})
	default:
		list.Fail("unknown syncStrategy value: %q", strategy)
	}
}

// pullParentBranchOfCurrentFeatureBranchStep adds the step to pull updates from the parent branch of the current feature branch into the current feature branch.
func pullParentBranchOfCurrentFeatureBranchStep(list *runstate.StepListBuilder, parentBranch domain.LocalBranchName, strategy config.SyncStrategy) {
	switch strategy {
	case config.SyncStrategyMerge:
		list.Add(&steps.MergeStep{Branch: parentBranch.BranchName()})
	case config.SyncStrategyRebase:
		list.Add(&steps.RebaseBranchStep{Branch: parentBranch.BranchName()})
	default:
		list.Fail("unknown syncStrategy value: %q", strategy)
	}
}

// updateCurrentPerennialBranchStep provides the steps to update the current perennial branch with changes from the given other branch.
func updateCurrentPerennialBranchStep(list *runstate.StepListBuilder, otherBranch domain.RemoteBranchName, strategy config.PullBranchStrategy) {
	switch strategy {
	case config.PullBranchStrategyMerge:
		list.Add(&steps.MergeStep{Branch: otherBranch.BranchName()})
	case config.PullBranchStrategyRebase:
		list.Add(&steps.RebaseBranchStep{Branch: otherBranch.BranchName()})
	default:
		list.Fail("unknown syncStrategy value: %q", strategy)
	}
}

func pushFeatureBranchSteps(list *runstate.StepListBuilder, branch domain.LocalBranchName, syncStrategy config.SyncStrategy, pushHook bool) {
	switch syncStrategy {
	case config.SyncStrategyMerge:
		list.Add(&steps.PushCurrentBranchStep{CurrentBranch: branch, NoPushHook: !pushHook, Undoable: false})
	case config.SyncStrategyRebase:
		list.Add(&steps.ForcePushBranchStep{Branch: branch, NoPushHook: false})
	default:
		list.Fail("unknown syncStrategy value: %q", syncStrategy)
	}
}
