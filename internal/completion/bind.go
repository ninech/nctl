package completion

import (
	"errors"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	kongcompletion "github.com/jotaen/kong-completion"
	"github.com/ninech/nctl/internal/apiresource"
	"github.com/posener/complete"
)

const predictorTag = "completion-predictor"

// unboundResourceName stands in for [ResourceName] while the completion tree
// is built, until [bindResourceNames] replaces it with a predictor bound to
// the API resource of its command. Reaching it means a command was not bound.
type unboundResourceName struct{}

func (unboundResourceName) Predict(complete.Args) []string {
	return fail(errors.New("no API resource is bound to the command"))
}

// bindResourceNames replaces [ResourceName] placeholders with predictors bound
// to each command's API resource.
//
// posener/complete re-slices args during traversal but leaves LastCompleted
// pointing to the end of the line, meaning trailing flags shadow the command.
// Binding upfront resolves the resource and any command aliases during tree build.
func bindResourceNames(cmd complete.Command, node *kong.Node, bind func(resource string) complete.Predictor) error {
	if node == nil {
		return nil
	}

	for _, child := range node.Children {
		if child == nil || child.Type != kong.CommandNode {
			continue
		}
		// Commands excluded from completion have no node in the tree.
		sub, ok := cmd.Sub[child.Name]
		if !ok {
			continue
		}
		if err := bindNodeResourceName(sub, child, bind); err != nil {
			return err
		}
		if err := bindResourceNames(sub, child, bind); err != nil {
			return err
		}
	}

	return nil
}

func bindNodeResourceName(cmd complete.Command, node *kong.Node, bind func(resource string) complete.Predictor) error {
	for i, positional := range node.Positional {
		if positional == nil || positional.Tag == nil ||
			positional.Tag.Get(predictorTag) != ResourceName {
			continue
		}
		// kong-completion predicts positional arguments by their position,
		// so the predictors are in the order the command declares them.
		predictors, ok := cmd.Args.(*kongcompletion.PositionalPredictor)
		if !ok || i >= len(predictors.Predictors) {
			return fmt.Errorf("the command %q completes no argument in position %d",
				strings.TrimSpace(node.Path()), i)
		}
		predictors.Predictors[i] = bind(apiresource.OfCommand(node))
	}

	return nil
}
