package common

import "github.com/alecthomas/kong"

// TODO: for test test only
type ApplicationCmd struct {
	// Size      *string `help:"Size of the app${size_hint}." placeholder:"${app_default_size}"`
	// Port      *int32  `help:"Port the app is listening on${port_hint}." placeholder:"${app_default_port}"`
	// Replicas  *int32  `help:"Amount of replicas of the running app${replicas_hint}." placeholder:"${app_default_replicas}"`
	// BasicAuth *bool   `help:"Enable/Disable basic authentication for the app${basic_auth_hint}." placeholder:"${app_default_basic_auth}"`
}

type ProjectCmd struct {
	DisplayName *string `default:"" help:"Display Name of the project."`
}

// TODO: reflection test for dynamic predictor. Remove it.
type ResourceCmd struct {
	// update
	// Name string `arg:"" predictor:"resource_name" help:"Name of the resource to update."`
	// create
	// Name string `arg:"" help:"Name of the new resource. A random name is generated if omitted." default:""`
	// shared:
	Name string `arg:"" predictor:"${name_predictor}" help:"Name of the resource. ${name_help_note}" default:"${name_default}"`
}

// WithKongVars is implemented by any root-level command
// that wants to supply its own kong.Vars before parsing.
type WithKongVars interface {
	KongVars() kong.Vars
}
