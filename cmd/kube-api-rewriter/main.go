/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"github.com/deckhouse/kube-api-rewriter/pkg/app"
	"github.com/deckhouse/kube-api-rewriter/pkg/rewriter"
)

// This proxy is a proof-of-concept of proxying Kubernetes API requests
// with rewrites.
//
// It assumes presence of KUBERNETES_* environment variables and files
// in /var/run/secrets/kubernetes.io/serviceaccount (token and ca.crt).
//
// A client behind the proxy should connect to 127.0.0.1:$PROXY_PORT
// using plain http. Example of kubeconfig file:
// apiVersion: v1
// kind: Config
// clusters:
//   - cluster:
//     server: http://127.0.0.1:23915
//     name: proxy.api.server
//
// contexts:
//   - context:
//     cluster: proxy.api.server
//     name: proxy.api.server
//
// current-context: proxy.api.server

func main() {
	app.StartFromEnv(exampleRules())
}

// exampleRules can be used as a simple illustration of the setup that rewrites CRD
// with kind SomeConfig in the api group some.crd.group.io into InternalResourceSomeConfig
// kind in the api group internalresourcesomeconfigs.some.internalresource.renamedgroup.io.
func exampleRules() *rewriter.RewriteRules {
	return &rewriter.RewriteRules{
		KindPrefix:         "InternalResource",
		ResourceTypePrefix: "internalresource",
		ShortNamePrefix:    "intres",
		Categories:         []string{"intres"},
		Rules: map[string]rewriter.APIGroupRule{
			"some.crd.group.io": {
				GroupRule: rewriter.GroupRule{
					Group:            "some.crd.group.io",
					Versions:         []string{"v1beta1"},
					PreferredVersion: "v1beta1",
					Renamed:          "some.internalresource.renamedgroup.io",
				},
				ResourceRules: map[string]rewriter.ResourceRule{
					// someconfigs.some.crd.group.io
					"someconfigs": {
						Kind:             "SomeConfig",
						ListKind:         "SomeConfigList",
						Plural:           "someconfigs",
						Singular:         "someconfig",
						Versions:         []string{"v1beta1"},
						PreferredVersion: "v1beta1",
						Categories:       []string{},
						ShortNames:       []string{},
					},
				},
			},
		},
	}
}
