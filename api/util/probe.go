package util

import (
	apps "github.com/ninech/apis/apps/v1alpha1"
)

// SetState describes how a field should be applied.
type SetState int

const (
	// leave as-is
	Unset SetState = iota
	// set to value
	Set
	// explicitly remove/unset
	Clear
)

// OptString is an "Optional string field" wrapper.
// It carries both a string value and a state (Unset / Set / Clear).
type OptString struct {
	State SetState
	Val   string
}

// OptInt32 is an "Optional int32 field" wrapper.
// It works the same way as OptString, but for numeric fields.
// Again, this lets us distinguish Unset (no flag) vs Set (positive value)
// vs Clear (explicitly reset to nil/default).
type OptInt32 struct {
	State SetState
	Val   int32
}

// ProbePatch is the normalized type used by both create and update paths.
type ProbePatch struct {
	Path          OptString
	PeriodSeconds OptInt32
}

// Patcher is implemented by command-specific flag structs to produce a ProbePatch.
type Patcher interface {
	ToProbePatch() ProbePatch
}

// ApplyProbePatch mutates cfg.
func ApplyProbePatch(cfg *apps.Config, pp ProbePatch) {
	switch pp.Path.State {
	case Set:
		ensureProbe(cfg)
		ensureHTTPGet(cfg)
		cfg.HealthProbe.HTTPGet.Path = pp.Path.Val
	case Clear:
		cfg.HealthProbe.HTTPGet = nil
	}

	switch pp.PeriodSeconds.State {
	case Set:
		ensureProbe(cfg)
		v := pp.PeriodSeconds.Val
		cfg.HealthProbe.PeriodSeconds = &v
	case Clear:
		cfg.HealthProbe.PeriodSeconds = nil
	}

	if cfg.HealthProbe != nil &&
		cfg.HealthProbe.HTTPGet == nil &&
		cfg.HealthProbe.PeriodSeconds == nil {
		cfg.HealthProbe = nil
	}
}

func ensureProbe(cfg *apps.Config) {
	if cfg.HealthProbe == nil {
		cfg.HealthProbe = &apps.Probe{}
	}
}

func ensureHTTPGet(cfg *apps.Config) {
	if cfg.HealthProbe.HTTPGet == nil {
		cfg.HealthProbe.HTTPGet = &apps.HTTPGetAction{}
	}
}
