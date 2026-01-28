package copy

import (
	"math/rand"
	"time"

	"github.com/lucasepe/codename"
)

type Cmd struct {
	Application applicationCmd `cmd:"" aliases:"app"`
}

type resourceCmd struct {
	Name          string `arg:"" help:"Name of the resource to copy." default:"" completion-predictor:"resource_name"`
	TargetName    string `help:"Target name of the new resource. A random name is generated if omitted." default:""`
	TargetProject string `help:"Target project of the new resource. The current project is used if omitted." default:"" completion-predictor:"project_name"`
}

func getName(name string) string {
	if len(name) != 0 {
		return name
	}

	return codename.Generate(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
}
