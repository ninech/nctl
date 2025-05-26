package test

import (
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// DeployJobKey defines deploy job comparable key
type DeployJobKey struct {
	Name, Command string
	Timeout       time.Duration
}

// ToDeployJobKey outputs deploy job comparable key
func ToDeployJobKey(j *apps.DeployJob) DeployJobKey {
	return DeployJobKey{
		Name:    j.Job.Name,
		Command: j.Job.Command,
		Timeout: ptr.Deref(j.Timeout, metav1.Duration{}).Duration,
	}
}

// WorkerJobKey defines worker job comparable key
type WorkerJobKey struct {
	Name, Command string
	Size          apps.ApplicationSize
}

// ToWorkerJobKey outputs worker job comparable key
func ToWorkerJobKey(j apps.WorkerJob) WorkerJobKey {
	return WorkerJobKey{
		Name:    j.Job.Name,
		Command: j.Job.Command,
		Size:    ptr.Deref(j.Size, ""),
	}
}

// ScheduledJobKey defines scheduled job comparable key
type ScheduledJobKey struct {
	Name, Command, Schedule string
	Size                    apps.ApplicationSize
	Retries                 int32
	Timeout                 time.Duration
}

// ToScheduledJobKey outputs scheduled job comparable key
func ToScheduledJobKey(j apps.ScheduledJob) ScheduledJobKey {
	return ScheduledJobKey{
		Name:     j.Job.Name,
		Command:  j.Job.Command,
		Schedule: j.Schedule,
		Size:     ptr.Deref(j.Size, ""),
		Retries:  ptr.Deref(j.Retries, 0),
		Timeout:  ptr.Deref(j.Timeout, metav1.Duration{}).Duration,
	}
}

// AssertJobsEqual generically checks that two slices of T contain exactly the
// same elements (order-independent), by mapping each element through toKey - a
// comparable key type
func AssertJobsEqual[T any, K comparable](t *testing.T, want, got []T, toKey func(T) K) {
	t.Helper()

	// record the exact frequency of each key.
	// That way we assert not only "this job exists' but also "and it exists exactly N times."
	count := func(list []T) map[K]int {
		m := make(map[K]int, len(list))
		for _, x := range list {
			m[toKey(x)]++
		}
		return m
	}

	assert.Equal(t, count(want), count(got))
}

// PtrToSlice converts any *S into a []D by calling conv, or returns nil if src==nil.
func PtrToSlice[S any, D any](src *S, conv func(*S) D) []D {
	if src == nil {
		return nil
	}
	return []D{conv(src)}
}

// NormalizeSlice turns any []T into nil if empty, else returns it unchanged.
func NormalizeSlice[T any](items []T) []T {
	if len(items) == 0 {
		return nil
	}
	return items
}
