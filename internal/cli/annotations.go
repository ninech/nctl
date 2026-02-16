package cli

const (
	ManagedByAnnotation = "app.kubernetes.io/managed-by"
	Name                = "nctl"
)

func IsManagedBy(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	return annotations[ManagedByAnnotation] == Name
}
