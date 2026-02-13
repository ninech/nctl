package application

import apps "github.com/ninech/apis/apps/v1alpha1"

const (
	// DNSSetupURL redirects to the proper deplo.io docs entry about
	// how to setup custom hosts
	DNSSetupURL = "https://docs.nine.ch/a/myshbw3EY1"

	dnsNotSetText = "<not set yet>"
)

func UnverifiedHosts(app *apps.Application) []string {
	unverifiedHosts := []string{}
	for _, host := range app.Status.AtProvider.Hosts {
		if host.LatestSuccess == nil {
			unverifiedHosts = append(unverifiedHosts, host.Name)
		}
	}
	// we need to remove duplicate hosts as we might have multiple DNS
	// error messages per host (different DNS record types)
	return uniqueStrings(unverifiedHosts)
}

// uniqueStrings removes duplicates from the given source string slice and
// returns it cleaned
func uniqueStrings(source []string) []string {
	unique := make(map[string]bool, len(source))
	us := make([]string, len(unique))
	for _, elem := range source {
		if !unique[elem] {
			us = append(us, elem)
			unique[elem] = true
		}
	}

	return us
}

type DNSDetail struct {
	Application string `json:"application"`
	Project     string `json:"project"`
	TXTRecord   string `json:"txtRecord"`
	CNAMETarget string `json:"cnameTarget"`
}

// DNSDetails retrieves the DNS details of all given applications
func DNSDetails(items []apps.Application) []DNSDetail {
	result := make([]DNSDetail, len(items))
	for i := range items {
		data := DNSDetail{
			Application: items[i].Name,
			Project:     items[i].Namespace,
			TXTRecord:   items[i].Status.AtProvider.TXTRecordContent,
			CNAMETarget: items[i].Status.AtProvider.CNAMETarget,
		}
		if data.TXTRecord == "" {
			data.TXTRecord = dnsNotSetText
		}
		if data.CNAMETarget == "" {
			data.CNAMETarget = dnsNotSetText
		}
		result[i] = data
	}
	return result
}
