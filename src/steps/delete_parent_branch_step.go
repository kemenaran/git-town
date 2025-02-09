package steps

import (
	"github.com/git-town/git-town/v9/src/domain"
	"github.com/git-town/git-town/v9/src/git"
)

// DeleteParentBranchStep removes the parent branch entry in the Git Town configuration.
type DeleteParentBranchStep struct {
	Branch domain.LocalBranchName
	Parent domain.LocalBranchName
	EmptyStep
}

func (step *DeleteParentBranchStep) CreateUndoSteps(_ *git.BackendCommands) ([]Step, error) {
	if step.Parent.IsEmpty() {
		return []Step{}, nil
	}
	return []Step{&SetParentStep{Branch: step.Branch, ParentBranch: step.Parent}}, nil
}

func (step *DeleteParentBranchStep) Run(args RunArgs) error {
	return args.Runner.Config.RemoveParent(step.Branch)
}
