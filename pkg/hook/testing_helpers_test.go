// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hook

import (
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	"github.com/flant/shell-operator/pkg/webhook/server"
)

// testAdmissionSettings replaces what used to be admission.DefaultSettings.
// Tests construct their own *admission.WebhookSettings instead of relying on
// a package-level singleton; this keeps every test self-contained and lets the
// production code drop the global entirely.
func testAdmissionSettings() *admission.WebhookSettings {
	return &admission.WebhookSettings{
		Settings: server.Settings{
			ServerCertPath: "/validating-certs/tls.crt",
			ServerKeyPath:  "/validating-certs/tls.key",
			ServiceName:    "shell-operator-validating-svc",
			ListenAddr:     "0.0.0.0",
			ListenPort:     "9680",
		},
		CAPath:               "/validating-certs/ca.crt",
		ConfigurationName:    "shell-operator-hooks",
		DefaultFailurePolicy: "Fail",
	}
}

// testConversionSettings replaces what used to be conversion.DefaultSettings.
func testConversionSettings() *conversion.WebhookSettings {
	return &conversion.WebhookSettings{
		Settings: server.Settings{
			ServerCertPath: "/conversion-certs/tls.crt",
			ServerKeyPath:  "/conversion-certs/tls.key",
			ServiceName:    "shell-operator-conversion-svc",
			ListenAddr:     "0.0.0.0",
			ListenPort:     "9681",
		},
		CAPath: "/conversion-certs/ca.crt",
	}
}
