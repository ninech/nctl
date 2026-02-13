// Package application provides utility functions for interacting with Deploio applications.
package application

import (
	"context"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"

	apps "github.com/ninech/apis/apps/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ApplicationNameLabel = "application.apps.nine.ch/name"
	PrivateKeySecretKey  = "privatekey"
	UsernameSecretKey    = "username"
	PasswordSecretKey    = "password"
	dnsNotSetText        = "<not set yet>"
	// DNSSetupURL redirects to the proper deplo.io docs entry about
	// how to setup custom hosts
	DNSSetupURL = "https://docs.nine.ch/a/myshbw3EY1"
	NoneText    = "<none>"
)

func UnverifiedHosts(app *apps.Application) []string {
	unverifiedHosts := []string{}
	for _, host := range app.Status.AtProvider.Hosts {
		if host.LatestSuccess == nil {
			unverifiedHosts = append(unverifiedHosts, host.Name)
		}
	}
	// we need to remove duplicate hosts as we might have multiple DNS
	// error messages per host (different DNS record types)
	return uniqueStrings(unverifiedHosts)
}

// uniqueStrings removes duplicates from the given source string slice and
// returns it cleaned
func uniqueStrings(source []string) []string {
	unique := make(map[string]bool, len(source))
	us := make([]string, len(unique))
	for _, elem := range source {
		if !unique[elem] {
			us = append(us, elem)
			unique[elem] = true
		}
	}

	return us
}

type EnvVarModifier func(envVar *apps.EnvVar)

func EnvVarsFromMap(env map[string]string, options ...EnvVarModifier) apps.EnvVars {
	vars := apps.EnvVars{}
	for k, v := range env {
		envVar := apps.EnvVar{Name: k, Value: v}
		for _, opt := range options {
			opt(&envVar)
		}
		vars = append(vars, envVar)
	}
	return vars
}

func Sensitive() EnvVarModifier {
	return func(envVar *apps.EnvVar) {
		envVar.Sensitive = ptr.To(true)
	}
}

func UpdateEnvVars(oldEnvs []apps.EnvVar, newEnvs, sensitiveEnvs map[string]string, toDelete []string) apps.EnvVars {
	if len(newEnvs) == 0 && len(sensitiveEnvs) == 0 && len(toDelete) == 0 {
		return oldEnvs
	}

	envMap := map[string]apps.EnvVar{}
	for _, v := range oldEnvs {
		envMap[v.Name] = v
	}

	new := EnvVarsFromMap(newEnvs)
	for _, v := range new {
		envMap[v.Name] = v
	}
	sensitive := EnvVarsFromMap(sensitiveEnvs, Sensitive())
	for _, v := range sensitive {
		envMap[v.Name] = v
	}

	for _, v := range toDelete {
		delete(envMap, v)
	}

	envs := []apps.EnvVar{}
	for _, v := range envMap {
		envs = append(envs, v)
	}

	sort.Slice(envs, func(i, j int) bool {
		return envs[i].Name < envs[j].Name
	})

	return envs
}

func EnvVarByName(envVars apps.EnvVars, name string) *apps.EnvVar {
	for _, e := range envVars {
		if e.Name == name {
			return &e
		}
	}

	return nil
}

type DNSDetail struct {
	Application string `json:"application"`
	Project     string `json:"project"`
	TXTRecord   string `json:"txtRecord"`
	CNAMETarget string `json:"cnameTarget"`
}

// GatherDNSDetails retrieves the DNS details of all given applications
func GatherDNSDetails(items []apps.Application) []DNSDetail {
	result := make([]DNSDetail, len(items))
	for i := range items {
		data := DNSDetail{
			Application: items[i].Name,
			Project:     items[i].Namespace,
			TXTRecord:   items[i].Status.AtProvider.TXTRecordContent,
			CNAMETarget: items[i].Status.AtProvider.CNAMETarget,
		}
		if data.TXTRecord == "" {
			data.TXTRecord = dnsNotSetText
		}
		if data.CNAMETarget == "" {
			data.CNAMETarget = dnsNotSetText
		}
		result[i] = data
	}
	return result
}

// ValidatePEM validates if the passed content is in valid PEM format, errors
// out if the content is empty
func ValidatePEM(content string) (*string, error) {
	if content == "" {
		return nil, fmt.Errorf("the SSH private key cannot be empty")
	}

	content = strings.TrimSpace(content)
	b, rest := pem.Decode([]byte(content))
	if b == nil || len(rest) > 0 {
		return nil, fmt.Errorf("no valid PEM formatted data found")
	}
	return &content, nil
}

// Releases returns a release list of an app. If the returned error is nil,
// the release list is guaranteed to have at least one item.
func Releases(ctx context.Context, client *api.Client, app types.NamespacedName) (*apps.ReleaseList, error) {
	releases := &apps.ReleaseList{}
	if err := client.List(
		ctx,
		releases,
		runtimeclient.InNamespace(app.Namespace),
		runtimeclient.MatchingLabels{ApplicationNameLabel: app.Name},
	); err != nil {
		return nil, err
	}

	if len(releases.Items) == 0 {
		return nil, fmt.Errorf("no releases found for application %s", app.Name)
	}
	return releases, nil
}

func LatestAvailableRelease(releases *apps.ReleaseList) *apps.Release {
	OrderReleaseList(releases, false)
	for _, release := range releases.Items {
		if release.Status.AtProvider.ReleaseStatus == apps.ReleaseProcessStatusAvailable {
			return &release
		}
	}
	return nil
}

// StaticEgresses returns all static egress resources targeting the
// specified app.
func StaticEgresses(ctx context.Context, client *api.Client, app types.NamespacedName) ([]networking.StaticEgress, error) {
	egressList := &networking.StaticEgressList{}
	if err := client.List(ctx, egressList, runtimeclient.InNamespace(app.Namespace)); err != nil {
		return nil, err
	}
	appEgresses := []networking.StaticEgress{}
	for _, egress := range egressList.Items {
		if egress.Spec.ForProvider.Target.Kind == apps.ApplicationKind &&
			egress.Spec.ForProvider.Target.Group == apps.Group &&
			egress.Spec.ForProvider.Target.Name == app.Name {
			appEgresses = append(appEgresses, egress)
		}
	}
	return appEgresses, nil
}
