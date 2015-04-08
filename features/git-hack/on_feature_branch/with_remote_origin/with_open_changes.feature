Feature: git hack: starting a new feature from a feature branch (with open changes and remote repo)

  As a developer working on something unrelated to my current feature branch
  I want to be able to create a new up-to-date feature branch and continue my work there
  So that my work can exist on its own branch, code reviews remain effective, and my team productive.


  Background:
    Given I have a feature branch named "existing_feature"
    And the following commits exist in my repository
      | BRANCH           | LOCATION | MESSAGE                 | FILE NAME    |
      | main             | remote   | main commit             | main_file    |
      | existing_feature | local    | existing feature commit | feature_file |
    And I am on the "existing_feature" branch
    And I have an uncommitted file
    When I run `git hack new_feature`


  Scenario: result
    Then it runs the Git commands
      | BRANCH           | COMMAND                          |
      | existing_feature | git fetch --prune                |
      |                  | git stash -u                     |
      |                  | git checkout main                |
      | main             | git rebase origin/main           |
      |                  | git checkout -b new_feature main |
      | new_feature      | git stash pop                    |
    And I end up on the "new_feature" branch
    And I still have my uncommitted file
    And I have the following commits
      | BRANCH           | LOCATION         | MESSAGE                 | FILE NAME    |
      | main             | local and remote | main commit             | main_file    |
      | existing_feature | local            | existing feature commit | feature_file |
      | new_feature      | local            | main commit             | main_file    |
