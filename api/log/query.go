package log

import (
	"fmt"
	"strings"

	apps "github.com/ninech/apis/apps/v1alpha1"
)

// Matcher is a single LogQL label matcher such as `app="name"`. Matchers are
// combined into a stream selector with [Selector].
type Matcher string

// Equal returns a matcher selecting streams whose label key equals value.
func Equal(key, value string) Matcher {
	return matcher("=", key, value)
}

// NotEqual returns a matcher selecting streams whose label key does not equal
// value.
func NotEqual(key, value string) Matcher {
	return matcher("!=", key, value)
}

func matcher(operator, key, value string) Matcher {
	return Matcher(fmt.Sprintf(`%s%s"%s"`, key, operator, value))
}

// InProject returns a matcher selecting the streams of the given project.
func InProject(project string) Matcher {
	return Equal("namespace", project)
}

// Selector returns the LogQL stream selector combining all matchers.
func Selector(matchers ...Matcher) string {
	s := make([]string, 0, len(matchers))
	for _, m := range matchers {
		s = append(s, string(m))
	}
	return "{" + strings.Join(s, ",") + "}"
}

// ApplicationQuery selects the logs of the application name in project,
// excluding its build logs.
func ApplicationQuery(name, project string) string {
	return Selector(
		InProject(project),
		Equal(apps.LogLabelApplication, name),
		Equal(apps.LogLabelBuild, ""),
	)
}

// BuildQuery selects the logs of the build name in project.
func BuildQuery(name, project string) string {
	return Selector(InProject(project), Equal(apps.LogLabelBuild, name))
}

// BuildsOfAppQuery selects the logs of all builds of the application name in
// project.
func BuildsOfAppQuery(name, project string) string {
	return Selector(
		InProject(project),
		Equal(apps.LogLabelApplication, name),
		NotEqual(apps.LogLabelBuild, ""),
	)
}
