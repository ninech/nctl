package application

import (
	apps "github.com/ninech/apis/apps/v1alpha1"
)

// BuildTypeDockerfile is the build type of applications which are built
// using a Dockerfile. All other build types are buildpack stacks.
const BuildTypeDockerfile = "dockerfile"

// ApplyBuildType configures app to be built with buildType, which is either
// BuildTypeDockerfile or the name of a buildpack stack. The buildpack stack
// is kept when switching to Dockerfile builds, as it is ignored for them.
func ApplyBuildType(app *apps.Application, buildType string) {
	if buildType == BuildTypeDockerfile {
		app.Spec.ForProvider.DockerfileBuild.Enabled = true
		return
	}
	app.Spec.ForProvider.DockerfileBuild.Enabled = false
	app.Spec.ForProvider.BuildpackStack = apps.BuildpackStack(buildType)
}
