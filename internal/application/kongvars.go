package application

import (
	"errors"
	"strconv"

	"github.com/alecthomas/kong"
	apps "github.com/ninech/apis/apps/v1alpha1"
)

// DefaultReplicas is the number of replicas an application is created with
// when --replicas is not passed.
const DefaultReplicas = 2

// KongVars returns the variables which the flags of the application create and
// update commands interpolate: defaults and help texts they share.
func KongVars() (kong.Vars, error) {
	result := make(kong.Vars)
	result["app_default_size"] = string(apps.DefaultConfig.Size)
	if apps.DefaultConfig.Port == nil {
		return nil, errors.New("no default application port found")
	}
	result["app_default_port"] = strconv.Itoa(int(*apps.DefaultConfig.Port))
	result["app_default_replicas"] = strconv.Itoa(DefaultReplicas)
	if apps.DefaultConfig.EnableBasicAuth == nil {
		return nil, errors.New("no default application basic authentication settings found")
	}
	result["app_default_basic_auth"] = strconv.FormatBool(*apps.DefaultConfig.EnableBasicAuth)

	result["app_default_health_probe_period_seconds"] = "10"

	result["app_default_deploy_job_timeout"] = "5m"
	result["app_default_deploy_job_retries"] = "3"
	result["app_default_scheduled_job_timeout"] = "5m"
	result["app_default_scheduled_job_retries"] = "0"
	result["app_language_help"] = "Language specifies which language your app is. " +
		"If left empty, deploio will detect the language automatically. "
	result["app_dockerfile_enable_help"] = "Enable Dockerfile build (Beta) instead of the automatic " +
		"buildpack detection"
	result["app_dockerfile_path_help"] = "Specifies the path to the Dockerfile. If left empty a file " +
		"named Dockerfile will be searched in the application code root directory."
	result["app_dockerfile_build_context_help"] = "Defines the build context. If left empty, the application code root directory will be used as build context."
	result["app_buildpack_stack_help"] = "BuildpackStack sets the stack of buildpacks to use for building the application. " +
		"If left empty, the default stack (heroku) will be used. "
	return result, nil
}
