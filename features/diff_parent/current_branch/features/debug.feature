Feature: display debug statistics

  Scenario: feature branch
    And the current branch is a feature branch "feature"
    When I run "git-town diff-parent --debug"
    Then it runs the commands
      | BRANCH  | TYPE     | COMMAND                       |
      |         | backend  | git version                   |
      |         | backend  | git config -lz --global       |
      |         | backend  | git config -lz --local        |
      |         | backend  | git rev-parse --show-toplevel |
      |         | backend  | git branch -vva               |
      | feature | frontend | git diff main..feature        |
    And it prints:
      """
      Ran 6 shell commands.
      """
