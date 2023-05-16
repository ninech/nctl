package util

import apps "github.com/ninech/apis/apps/v1alpha1"

func UnverifiedAppHosts(app *apps.Application) []string {
	unverifiedHosts := []string{}
	for _, host := range app.Status.AtProvider.Hosts {
		if host.LatestSuccess == nil {
			unverifiedHosts = append(unverifiedHosts, host.Name)
		}
	}
	return unverifiedHosts
}

func VerifiedAppHosts(app *apps.Application) []string {
	verifiedHosts := []string{}
	for _, host := range app.Status.AtProvider.Hosts {
		if host.LatestSuccess != nil && host.Error == nil {
			verifiedHosts = append(verifiedHosts, host.Name)
		}
	}
	return verifiedHosts
}
