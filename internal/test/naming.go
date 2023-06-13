package test

import (
	apps "github.com/ninech/apis/apps/v1alpha1"
)

var (
	// TODO: would be nice to get this from "github.com/ninech/apis/apps/v1alpha1"
	AppMicro     apps.ApplicationSize = "micro"
	AppMini      apps.ApplicationSize = "mini"
	AppStandard1 apps.ApplicationSize = "standard-1"

	StatusSuperseded apps.ReleaseProcessStatus = "superseded"
	StatusAvailable  apps.ReleaseProcessStatus = "available"
)
