// Package copy provides commands to copy resources.
package copy

import (
	"io"
	"math/rand"
	"time"

	"github.com/lucasepe/codename"
	"github.com/ninech/nctl/internal/format"
)

type Cmd struct {
	Application applicationCmd `cmd:"" aliases:"app"`
}

// ResourceCmd is the shared base for the copy sub-commands.
//
// It has to be exported so that Kong initializes the embedded [format.Writer], see [format.Writer.BeforeApply].
type ResourceCmd struct {
	format.Writer `kong:"-"`
	Name          string `arg:"" help:"Name of the resource to copy." default:"" completion-predictor:"resource_name"`
	TargetName    string `help:"Target name of the new resource. A random name is generated if omitted." default:""`
	TargetProject string `help:"Target project of the new resource. The current project is used if omitted." default:"" completion-predictor:"project_name"`
}

// BeforeApply initializes Writer from Kong's bound [io.Writer].
func (cmd *ResourceCmd) BeforeApply(writer io.Writer) error {
	return cmd.Writer.BeforeApply(writer)
}

func getName(name string) string {
	if len(name) != 0 {
		return name
	}

	return codename.Generate(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
}
