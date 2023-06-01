package util

import (
	apps "github.com/ninech/apis/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ApplicationNameLabel = "application.apps.nine.ch/name"
	PrivateKeySecretKey  = "privatekey"
	UsernameSecretKey    = "username"
	PasswordSecretKey    = "password"
)

func UnverifiedAppHosts(app *apps.Application) []string {
	unverifiedHosts := []string{}
	for _, host := range app.Status.AtProvider.Hosts {
		if host.LatestSuccess == nil {
			unverifiedHosts = append(unverifiedHosts, host.Name)
		}
	}
	return unverifiedHosts
}

func VerifiedAppHosts(app *apps.Application) []string {
	verifiedHosts := []string{}
	for _, host := range app.Status.AtProvider.Hosts {
		if host.LatestSuccess != nil && host.Error == nil {
			verifiedHosts = append(verifiedHosts, host.Name)
		}
	}
	return verifiedHosts
}

func EnvVarsFromMap(env map[string]string) apps.EnvVars {
	vars := apps.EnvVars{}
	for k, v := range env {
		vars = append(vars, apps.EnvVar{Name: k, Value: v})
	}
	return vars
}

type GitAuth struct {
	Username      *string
	Password      *string
	SSHPrivateKey *string
}

func (git GitAuth) Secret(name, namespace string) *corev1.Secret {
	data := map[string][]byte{}

	if git.SSHPrivateKey != nil {
		data[PrivateKeySecretKey] = []byte(*git.SSHPrivateKey)
	} else if git.Username != nil && git.Password != nil {
		data[UsernameSecretKey] = []byte(*git.Username)
		data[PasswordSecretKey] = []byte(*git.Password)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
}

func (git GitAuth) Enabled() bool {
	if git.Username != nil ||
		git.Password != nil ||
		git.SSHPrivateKey != nil {
		return true
	}

	return false
}
