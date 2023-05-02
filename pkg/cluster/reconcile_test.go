// Copyright 2017 The etcd-operator Authors
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

package cluster

import (
	"context"
	"reflect"
	"testing"

	api "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	"github.com/coreos/etcd-operator/pkg/util/k8sutil"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestReconcileServices(t *testing.T) {
	config := Config{
		KubeCli: fake.NewSimpleClientset(),
	}

	c := &Cluster{
		config:  config,
		logger:  logrus.WithField("pkg", "test"),
		cluster: &api.EtcdCluster{},
	}

	tests := []struct {
		name            string
		desiredServices []*api.ServicePolicy
		currentServices map[string]*v1.Service
		expectedActions []string
	}{
		{
			"shouldn't do any action",
			[]*api.ServicePolicy{
				{
					Name: "Test",
				},
			},
			map[string]*v1.Service{
				"Test": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Test",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			[]string{},
		},
		{
			"should remove a service",
			[]*api.ServicePolicy{},
			map[string]*v1.Service{
				"Test": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Test",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			[]string{
				"delete",
			},
		},
		{
			"should create a service",
			[]*api.ServicePolicy{
				{
					Name: "Test",
				},
			},
			map[string]*v1.Service{},
			[]string{
				"create",
			},
		},
		{
			"should create 2 services and delete 2 services",
			[]*api.ServicePolicy{
				{
					Name: "Test",
				},
				{
					Name: "Test2",
				},
				{
					Name: "Test3",
				},
			},
			map[string]*v1.Service{
				"Test": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Test",
						Namespace: metav1.NamespaceDefault,
					},
				},
				"Test4": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Test4",
						Namespace: metav1.NamespaceDefault,
					},
				},
				"Test5": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Test5",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			[]string{
				"create",
				"create",
				"delete",
				"delete",
			},
		},
	}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {
		labels := k8sutil.LabelsForCluster(clstrName)
		svc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        svcName,
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: v1.ServiceSpec{
				Ports:                    ports,
				Selector:                 labels,
				PublishNotReadyAddresses: publishNotReadyAddresses,
			},
		}
		kubecli.(*fake.Clientset).CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
		return nil
	}

	for _, tt := range tests {
		c.config.KubeCli.(*fake.Clientset).ClearActions()

		c.cluster.Spec.Services = tt.desiredServices
		err := c.reconcileServices(context.Background(), tt.currentServices, createSvc)
		if err != nil {
			t.Fatalf("%s: %s", tt.name, err)
		}
		actions := c.config.KubeCli.(*fake.Clientset).Actions()
		actionsVerbs := []string{}
		for _, action := range actions {
			actionsVerbs = append(actionsVerbs, action.GetVerb())
		}
		if !reflect.DeepEqual(actionsVerbs, tt.expectedActions) {
			t.Fatalf("%s: expected:%v, got:%v", tt.name, tt.expectedActions, actionsVerbs)
		}

	}
}

func TestDiffServices(t *testing.T) {
	c := &Cluster{
		logger: logrus.WithField("pkg", "test"),
		cluster: &api.EtcdCluster{
			Spec: api.ClusterSpec{},
		},
	}

	tests := []struct {
		name            string
		desiredServices []*api.ServicePolicy
		currentServices map[string]*v1.Service
		expectedNew     []*api.ServicePolicy
		expectedUnknown map[string]*v1.Service
	}{
		{
			"should return empty",
			[]*api.ServicePolicy{
				{
					Name: "Test",
				},
			},
			map[string]*v1.Service{
				"Test": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Test",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			[]*api.ServicePolicy{},
			map[string]*v1.Service{},
		},
		{
			"should return one to add",
			[]*api.ServicePolicy{
				{
					Name: "Test",
				},
			},
			map[string]*v1.Service{},
			[]*api.ServicePolicy{
				{
					Name: "Test",
				},
			},
			map[string]*v1.Service{},
		},
		{
			"should return one to remove",
			[]*api.ServicePolicy{},
			map[string]*v1.Service{
				"Test": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Test",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			[]*api.ServicePolicy{},
			map[string]*v1.Service{
				"Test": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Test",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		c.cluster.Spec.Services = tt.desiredServices
		new, unknown, err := c.diffServices(tt.currentServices)
		if err != nil {
			t.Fatalf("%s: %s", tt.name, err)
		}
		if !reflect.DeepEqual(new, tt.expectedNew) {
			t.Fatalf("%s: expected %v, got: %v", tt.name, tt.expectedNew, new)
		}
		if !reflect.DeepEqual(unknown, tt.expectedUnknown) {
			t.Fatalf("%s: expected %v, got: %v", tt.name, tt.expectedUnknown, unknown)
		}
	}

}
