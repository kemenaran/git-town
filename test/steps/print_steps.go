package steps

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/cucumber/messages-go/v10"
)

// PrintSteps defines Gherkin steps around printing things to the terminal.
func PrintSteps(suite *godog.Suite, fs *FeatureState) {
	suite.Step(`^it does not print "([^\"]*)"$`, func(text string) error {
		if strings.Contains(fs.state.runRes.OutputSanitized(), text) {
			return fmt.Errorf("text found: %q", text)
		}
		return nil
	})

	suite.Step(`^it prints:$`, func(expected *messages.PickleStepArgument_PickleDocString) error {
		if !strings.Contains(fs.state.runRes.OutputSanitized(), expected.Content) {
			return fmt.Errorf("text not found:\n\nEXPECTED: %q\n\nACTUAL:\n\n%q", expected.Content, fs.state.runRes.OutputSanitized())
		}
		return nil
	})

	suite.Step(`^it prints no output$`, func() error {
		output := fs.state.runRes.OutputSanitized()
		if output != "" {
			return fmt.Errorf("expected no output but found %q", output)
		}
		return nil
	})

	suite.Step(`^it prints the error:$`, func(expected *messages.PickleStepArgument_PickleDocString) error {
		if !strings.Contains(fs.state.runRes.OutputSanitized(), expected.Content) {
			return fmt.Errorf("text not found: %s\n\nactual text:\n%s", expected.Content, fs.state.runRes.OutputSanitized())
		}
		if fs.state.runErr == nil {
			return fmt.Errorf("expected error")
		}
		return nil
	})

	suite.Step(`^it prints the initial configuration prompt$`, func() error {
		expected := "Git Town needs to be configured"
		if !fs.state.runRes.OutputContainsText(expected) {
			return fmt.Errorf("text not found:\n\nEXPECTED: %q\n\nACTUAL:\n\n%q\n----------------------------", expected, fs.state.runRes.Output())
		}
		return nil
	})

	suite.Step(`^I am not prompted for any parent branches$`, func() error {
		notExpected := "Please specify the parent branch of"
		if fs.state.runRes.OutputContainsText(notExpected) {
			return fmt.Errorf("text found:\n\nDID NOT EXPECT: %q\n\nACTUAL\n\n%q\n----------------------------", notExpected, fs.state.runRes.Output())
		}
		return nil
	})
}
