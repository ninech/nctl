package gitinfo

import (
	"fmt"

	apps "github.com/ninech/apis/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	PrivateKeySecretKey = "privatekey"
	UsernameSecretKey   = "username"
	PasswordSecretKey   = "password"
)

// Auth contains the credentials for a git repository.
type Auth struct {
	Username      *string
	Password      *string
	SSHPrivateKey *string
}

func (a Auth) HasPrivateKey() bool {
	return a.SSHPrivateKey != nil
}

func (a Auth) HasBasicAuth() bool {
	return a.Username != nil && a.Password != nil
}

// NewAuthSecret returns a new secret for the given application. It can be used as
// a key for Get/Delete operations or as a base for populating credentials.
func NewAuthSecret(app *apps.Application) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthSecretName(app),
			Namespace: app.Namespace,
		},
	}
}

// ApplyToSecret writes the Auth credentials into the given secret's Data field.
// Only writes fields which are non-nil.
func (a Auth) ApplyToSecret(secret *corev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}

	if a.SSHPrivateKey != nil {
		secret.Data[PrivateKeySecretKey] = []byte(*a.SSHPrivateKey)
	}

	if a.Username != nil {
		secret.Data[UsernameSecretKey] = []byte(*a.Username)
	}

	if a.Password != nil {
		secret.Data[PasswordSecretKey] = []byte(*a.Password)
	}
}

// Enabled returns true if any kind of credentials are set in the GitAuth
func (a Auth) Enabled() bool {
	return a.HasBasicAuth() || a.HasPrivateKey()
}

// Valid validates the credentials in the GitAuth
func (a Auth) Valid() error {
	if a.SSHPrivateKey != nil {
		if *a.SSHPrivateKey == "" {
			return fmt.Errorf("the SSH private key cannot be empty")
		}
	}

	if a.Username != nil && a.Password != nil {
		if *a.Username == "" || *a.Password == "" {
			return fmt.Errorf("the username/password cannot be empty")
		}
	}

	return nil
}

// AuthSecretName returns the name of the secret which contains the git
// credentials for the given applications git source
func AuthSecretName(app *apps.Application) string {
	return app.Name
}

// UpdateFromSecret updates the Auth object with the data from the given secret.
func (a *Auth) UpdateFromSecret(secret *corev1.Secret) {
	if val, ok := secret.Data[PrivateKeySecretKey]; ok {
		a.SSHPrivateKey = ptr.To(string(val))
	}

	if val, ok := secret.Data[UsernameSecretKey]; ok {
		a.Username = ptr.To(string(val))
	}

	if val, ok := secret.Data[PasswordSecretKey]; ok {
		a.Password = ptr.To(string(val))
	}
}
