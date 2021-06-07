// Copyright 2016 The etcd-operator Authors
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

package k8sutil

import (
	"k8s.io/api/core/v1"
	api "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	v1 "k8s.io/api/core/v1"
)

func applyServicePolicy(service *v1.Service, policy *api.ServicePolicy) {
	if policy == nil {
		return
	}

	if len(policy.Type) != 0 {
		service.Spec.Type = policy.Type
	}

	if len(policy.ClusterIP) != 0 {
		service.Spec.ClusterIP = policy.ClusterIP
	}

	if len(policy.Selector) != 0 {
		service.Spec.Selector = policy.Selector
	}

	for key, value := range policy.Annotations {
		service.ObjectMeta.Annotations[key] = value
	}
}
