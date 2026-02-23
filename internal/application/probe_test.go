package application

import (
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestApplyProbePatch(t *testing.T) {
	t.Parallel()

	setPath := func(s string) OptString {
		return OptString{State: Set, Val: s}
	}
	clearPath := func() OptString {
		return OptString{State: Clear}
	}
	unsetPath := func() OptString {
		return OptString{State: Unset}
	}
	setPer := func(n int32) OptInt32 {
		return OptInt32{State: Set, Val: n}
	}
	clearPer := func() OptInt32 {
		return OptInt32{State: Clear}
	}
	unsetPer := func() OptInt32 {
		return OptInt32{State: Unset}
	}

	tests := []struct {
		name string
		cfg  apps.Config
		pp   ProbePatch
		want func(t *testing.T, got *apps.Config)
	}{
		{
			name: "no-op when everything Unset and cfg nil",
			cfg:  apps.Config{},
			pp:   ProbePatch{Path: unsetPath(), PeriodSeconds: unsetPer()},
			want: func(t *testing.T, got *apps.Config) {
				is := require.New(t)
				is.Nil(got.HealthProbe)
			},
		},
		{
			name: "set Path creates probe+httpget and assigns path",
			cfg:  apps.Config{},
			pp:   ProbePatch{Path: setPath("/healthz"), PeriodSeconds: unsetPer()},
			want: func(t *testing.T, got *apps.Config) {
				is := require.New(t)
				is.NotNil(got.HealthProbe)
				is.NotNil(got.HealthProbe.HTTPGet)
				is.Equal("/healthz", got.HealthProbe.HTTPGet.Path)
				is.Nil(got.HealthProbe.PeriodSeconds)
			},
		},
		{
			name: "set PeriodSeconds creates probe and sets value (owns memory)",
			cfg:  apps.Config{},
			pp:   ProbePatch{Path: unsetPath(), PeriodSeconds: setPer(7)},
			want: func(t *testing.T, got *apps.Config) {
				is := require.New(t)
				is.NotNil(got.HealthProbe)
				is.NotNil(got.HealthProbe.PeriodSeconds)
				is.Equal(int32(7), *got.HealthProbe.PeriodSeconds)
				is.Nil(got.HealthProbe.HTTPGet)
			},
		},
		{
			name: "set both fields on existing probe updates both",
			cfg: apps.Config{
				HealthProbe: &apps.Probe{
					ProbeHandler: apps.ProbeHandler{
						HTTPGet: &apps.HTTPGetAction{Path: "/old"},
					},
					PeriodSeconds: new(int32(3)),
				},
			},
			pp: ProbePatch{Path: setPath("/new"), PeriodSeconds: setPer(9)},
			want: func(t *testing.T, got *apps.Config) {
				is := require.New(t)
				is.NotNil(got.HealthProbe)
				is.NotNil(got.HealthProbe.HTTPGet)
				is.Equal("/new", got.HealthProbe.HTTPGet.Path)
				is.NotNil(got.HealthProbe.PeriodSeconds)
				is.Equal(int32(9), *got.HealthProbe.PeriodSeconds)
			},
		},
		{
			name: "clear Path removes HTTPGet but keeps probe if other fields remain",
			cfg: apps.Config{
				HealthProbe: &apps.Probe{
					ProbeHandler: apps.ProbeHandler{
						HTTPGet: &apps.HTTPGetAction{Path: "/keep-me?no"},
					},
					PeriodSeconds: new(int32(5)),
				},
			},
			pp: ProbePatch{Path: clearPath(), PeriodSeconds: unsetPer()},
			want: func(t *testing.T, got *apps.Config) {
				is := require.New(t)
				is.NotNil(got.HealthProbe)
				is.Nil(got.HealthProbe.HTTPGet)
				is.NotNil(got.HealthProbe.PeriodSeconds)
				is.Equal(int32(5), *got.HealthProbe.PeriodSeconds)
			},
		},
		{
			name: "clear PeriodSeconds sets it to nil but preserves HTTPGet",
			cfg: apps.Config{
				HealthProbe: &apps.Probe{
					ProbeHandler: apps.ProbeHandler{
						HTTPGet: &apps.HTTPGetAction{Path: "/ok"},
					},
					PeriodSeconds: new(int32(11)),
				},
			},
			pp: ProbePatch{Path: unsetPath(), PeriodSeconds: clearPer()},
			want: func(t *testing.T, got *apps.Config) {
				is := require.New(t)
				is.NotNil(got.HealthProbe)
				is.NotNil(got.HealthProbe.HTTPGet)
				is.Equal("/ok", got.HealthProbe.HTTPGet.Path)
				is.Nil(got.HealthProbe.PeriodSeconds)
			},
		},
		{
			name: "clearing last fields removes the whole HealthProbe",
			cfg: apps.Config{
				HealthProbe: &apps.Probe{
					ProbeHandler:  apps.ProbeHandler{HTTPGet: &apps.HTTPGetAction{Path: "/gone"}},
					PeriodSeconds: nil,
				},
			},
			pp: ProbePatch{Path: clearPath(), PeriodSeconds: unsetPer()},
			want: func(t *testing.T, got *apps.Config) {
				is := require.New(t)
				is.Nil(got.HealthProbe)
			},
		},
		{
			name: "unset fields do not create or modify probe",
			cfg: apps.Config{
				HealthProbe: nil,
			},
			pp: ProbePatch{Path: unsetPath(), PeriodSeconds: unsetPer()},
			want: func(t *testing.T, got *apps.Config) {
				is := require.New(t)
				is.Nil(got.HealthProbe)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := tt.cfg
			ApplyProbePatch(&cfg, tt.pp)
			tt.want(t, &cfg)
		})
	}
}
