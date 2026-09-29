package api

const (
	// ManagedByAnnotation is the annotation key marking objects that
	// nctl created or updated.
	ManagedByAnnotation = "app.kubernetes.io/managed-by"
	// Name is the name of the CLI and the value of ManagedByAnnotation
	// on objects managed by it.
	Name = "nctl"
)

// IsManagedBy reports whether the annotations mark an object as managed by
// nctl.
func IsManagedBy(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	return annotations[ManagedByAnnotation] == Name
}
