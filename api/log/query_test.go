package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelector(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		matchers []Matcher
		expected string
	}{
		"no matchers": {
			expected: `{}`,
		},
		"single matcher": {
			matchers: []Matcher{Equal("app", "some-app")},
			expected: `{app="some-app"}`,
		},
		"multiple matchers keep their order": {
			matchers: []Matcher{Equal("app", "some-app"), InProject("default")},
			expected: `{app="some-app",namespace="default"}`,
		},
		"not equal and empty values": {
			matchers: []Matcher{NotEqual("build", ""), Equal("job", "")},
			expected: `{build!="",job=""}`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, Selector(tc.matchers...))
		})
	}
}

func TestQueries(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		query    func(name, project string) string
		expected string
	}{
		"application": {
			query:    ApplicationQuery,
			expected: `{namespace="some-project",app="some-name",build=""}`,
		},
		"build": {
			query:    BuildQuery,
			expected: `{namespace="some-project",build="some-name"}`,
		},
		"builds of application": {
			query:    BuildsOfAppQuery,
			expected: `{namespace="some-project",app="some-name",build!=""}`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.query("some-name", "some-project"))
		})
	}
}
