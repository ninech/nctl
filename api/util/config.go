package util

import (
	"errors"

	apps "github.com/ninech/apis/apps/v1alpha1"
)

// ValidateConfig validates the configuration of an application.
func ValidateConfig(config apps.Config) error {
	if config.DeployJob != nil {
		if len(config.DeployJob.Name) == 0 {
			return errors.New("deploy job name cannot be empty")
		}
	}
	return nil
}
