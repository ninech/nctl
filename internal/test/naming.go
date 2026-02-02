package test

import (
	apps "github.com/ninech/apis/apps/v1alpha1"
)

var (
	// TODO: would be nice to get this from "github.com/ninech/apis/apps/v1alpha1"
	AppSizeNotSet apps.ApplicationSize = ""
	AppMicro      apps.ApplicationSize = "micro"
	AppMini       apps.ApplicationSize = "mini"
	AppStandard1  apps.ApplicationSize = "standard-1"
	AppStandard2  apps.ApplicationSize = "standard-2"
	AppStandard4  apps.ApplicationSize = "standard-4"

	StatusSuperseded apps.ReleaseProcessStatus = "superseded"
	StatusAvailable  apps.ReleaseProcessStatus = "available"
)
