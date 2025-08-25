package util

import (
	"context"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"

	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ApplicationNameLabel = "application.apps.nine.ch/name"
	ManagedByAnnotation  = "app.kubernetes.io/managed-by"
	NctlName             = "nctl"
	PrivateKeySecretKey  = "privatekey"
	UsernameSecretKey    = "username"
	PasswordSecretKey    = "password"
	dnsNotSetText        = "<not set yet>"
	// DNSSetupURL redirects to the proper deplo.io docs entry about
	// how to setup custom hosts
	DNSSetupURL = "https://docs.nine.ch/a/myshbw3EY1"
	NoneText    = "<none>"
)

func UnverifiedAppHosts(app *apps.Application) []string {
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

func VerifiedAppHosts(app *apps.Application) []string {
	verifiedHosts := []string{}
	for _, host := range app.Status.AtProvider.Hosts {
		if host.CheckType == meta.DNSCheckType("CAA") {
			continue
		}
		if host.LatestSuccess != nil && host.Error == nil {
			verifiedHosts = append(verifiedHosts, host.Name)
		}
	}
	return verifiedHosts
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

func EnvVarToString(envs apps.EnvVars) string {
	if envs == nil {
		return NoneText
	}

	if len(envs) == 0 {
		return NoneText
	}

	var keyValuePairs []string
	for _, env := range envs {
		if env.Sensitive != nil && *env.Sensitive {
			keyValuePairs = append(keyValuePairs, fmt.Sprintf("%v=*****", env.Name))
		} else {
			keyValuePairs = append(keyValuePairs, fmt.Sprintf("%v=%v", env.Name, env.Value))
		}
	}

	return strings.Join(keyValuePairs, ";")
}

func EnvVarByName(envVars apps.EnvVars, name string) *apps.EnvVar {
	for _, e := range envVars {
		if e.Name == name {
			return &e
		}
	}

	return nil
}

type GitAuth struct {
	Username      *string
	Password      *string
	SSHPrivateKey *string
}

func GitAuthFromApp(ctx context.Context, client *api.Client, app *apps.Application) (GitAuth, error) {
	auth := GitAuth{}
	if app.Spec.ForProvider.Git.Auth == nil {
		return auth, nil
	}

	if app.Spec.ForProvider.Git.Auth.FromSecret == nil {
		return auth, nil
	}

	secret := auth.Secret(app)
	if err := client.Get(ctx, client.Name(secret.Name), secret); err != nil {
		return auth, err
	}

	if val, ok := secret.Data[PrivateKeySecretKey]; ok {
		auth.SSHPrivateKey = ptr.To(string(val))
	}

	if val, ok := secret.Data[UsernameSecretKey]; ok {
		auth.Username = ptr.To(string(val))
	}

	if val, ok := secret.Data[PasswordSecretKey]; ok {
		auth.Password = ptr.To(string(val))
	}

	return auth, nil
}

func (git GitAuth) HasPrivateKey() bool {
	return git.SSHPrivateKey != nil
}

func (git GitAuth) HasBasicAuth() bool {
	return git.Username != nil && git.Password != nil
}

func (git GitAuth) Secret(app *apps.Application) *corev1.Secret {
	data := map[string][]byte{}

	if git.SSHPrivateKey != nil {
		data[PrivateKeySecretKey] = []byte(*git.SSHPrivateKey)
	} else if git.Username != nil && git.Password != nil {
		data[UsernameSecretKey] = []byte(*git.Username)
		data[PasswordSecretKey] = []byte(*git.Password)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GitAuthSecretName(app),
			Namespace: app.Namespace,
			Annotations: map[string]string{
				ManagedByAnnotation: NctlName,
			},
		},
		Data: data,
	}
}

// UpdateSecret replaces the data of the secret with the data from GitAuth. Only
// replaces fields which are non-nil.
func (git GitAuth) UpdateSecret(secret *corev1.Secret) {
	if git.SSHPrivateKey != nil {
		secret.Data[PrivateKeySecretKey] = []byte(*git.SSHPrivateKey)
	}

	if git.Username != nil {
		secret.Data[UsernameSecretKey] = []byte(*git.Username)
	}

	if git.Password != nil {
		secret.Data[PasswordSecretKey] = []byte(*git.Password)
	}
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[ManagedByAnnotation] = NctlName
}

// Enabled returns true if any kind of credentials are set in the GitAuth
func (git GitAuth) Enabled() bool {
	return git.HasBasicAuth() || git.HasPrivateKey()
}

// Valid validates the credentials in the GitAuth
func (git GitAuth) Valid() error {
	if git.SSHPrivateKey != nil {
		if *git.SSHPrivateKey == "" {
			return fmt.Errorf("the SSH private key cannot be empty")
		}
	}

	if git.Username != nil && git.Password != nil {
		if *git.Username == "" || *git.Password == "" {
			return fmt.Errorf("the username/password cannot be empty")
		}
	}

	return nil
}

// GitAuthSecretName returns the name of the secret which contains the git
// credentials for the given applications git source
func GitAuthSecretName(app *apps.Application) string {
	return app.Name
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

func ApplicationLatestAvailableRelease(ctx context.Context, client *api.Client, app types.NamespacedName) (*apps.Release, error) {
	releases, err := appReleases(ctx, client, app)
	if err != nil {
		return nil, err
	}

	release := latestAvailableRelease(releases)
	if release == nil {
		return nil, fmt.Errorf("no ready release found for application %s", app.Name)
	}

	return release, nil
}

// ApplicationLatestRelease returns the latest release of an app, prioritizing
// available releases and if no available release is found just the latest
// progressing or failed release.
func ApplicationLatestRelease(ctx context.Context, client *api.Client, app types.NamespacedName) (*apps.Release, error) {
	releases, err := appReleases(ctx, client, app)
	if err != nil {
		return nil, err
	}

	release := latestAvailableRelease(releases)
	if release == nil {
		// in case no release is available, we just return the latest release in
		// the list.
		return &releases.Items[0], nil
	}

	return release, nil
}

// appReleases returns a release list of an app. If the returned error is nil,
// the release list is guaranteed to have at least one item.
func appReleases(ctx context.Context, client *api.Client, app types.NamespacedName) (*apps.ReleaseList, error) {
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

func latestAvailableRelease(releases *apps.ReleaseList) *apps.Release {
	OrderReleaseList(releases, false)
	for _, release := range releases.Items {
		if release.Status.AtProvider.ReleaseStatus == apps.ReleaseProcessStatusAvailable {
			return &release
		}
	}
	return nil
}

// ApplicationStaticEgresses returns all static egress resources targeting the
// specified app.
func ApplicationStaticEgresses(ctx context.Context, client *api.Client, app types.NamespacedName) ([]networking.StaticEgress, error) {
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
