package validation

import (
	"errors"

	apps "github.com/ninech/apis/apps/v1alpha1"
)

// ConfigValidator validates a Config
type ConfigValidator struct {
	Config apps.Config
}

// Validate validates the config
func (c ConfigValidator) Validate() error {
	if c.Config.DeployJob != nil {
		if len(c.Config.DeployJob.Name) == 0 {
			return errors.New("deploy job name cannot be empty")
		}
	}
	return nil
}
