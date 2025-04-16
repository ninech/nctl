package validation

import (
	"errors"

	apps "github.com/ninech/apis/apps/v1alpha1"
)

// ConfigValidator validates a Config
type ConfigValidator struct {
	Config apps.Config
}

// TODO: remove local job validation logic in favor of webhook-based CRD validation
// We now rely on the Kubernetes admission webhook deployed for our CRD backend,
// which performs full validation of the config, including DeployJob. The removed
// logic only checked the job name and did not cover other important fields,
// making it insufficient for robust validation.

// Validate validates the config
func (c ConfigValidator) Validate() error {
	if c.Config.DeployJob != nil {
		if len(c.Config.DeployJob.Name) == 0 {
			return errors.New("deploy job name cannot be empty")
		}
	}
	return nil
}
