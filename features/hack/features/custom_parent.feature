@skipWindows
Feature: customize the parent for the new feature branch

  Background:
    Given the current branch is "existing"
    When I run "git-town hack new --prompt" and answer the prompts:
      | PROMPT                                         | ANSWER        |
      | Please specify the parent branch of 'new'      | [DOWN][ENTER] |
      | Please specify the parent branch of 'existing' | [ENTER]       |

  Scenario: result
    Then it runs the commands
      | BRANCH   | COMMAND                     |
      | existing | git fetch --prune --tags    |
      |          | git checkout main           |
      | main     | git rebase origin/main      |
      |          | git checkout existing       |
      | existing | git merge --no-edit main    |
      |          | git push -u origin existing |
      |          | git branch new existing     |
      |          | git checkout new            |
    And the current branch is now "new"
    And this branch lineage exists now
      | BRANCH   | PARENT   |
      | existing | main     |
      | new      | existing |

  Scenario: undo
    When I run "git town undo"
    Then it runs the commands
      | BRANCH   | COMMAND                   |
      | new      | git checkout existing     |
      | existing | git branch -D new         |
      |          | git push origin :existing |
      |          | git checkout main         |
      | main     | git checkout existing     |
    And the current branch is now "existing"
    And this branch lineage exists now
      | BRANCH   | PARENT |
      | existing | main   |
