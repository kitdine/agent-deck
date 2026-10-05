//go:build !darwin

package doctor

import "context"

func acquireRegistration(context.Context) RegistrationDetails {
	return RegistrationDetails{ApplicabilityReason: "unsupported_platform", Host: RegistrationSource{Source: "host_application_urls"}, Widget: RegistrationSource{Source: "widget_pluginkit"}}
}
