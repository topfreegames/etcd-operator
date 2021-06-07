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
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	api "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// When EtcdCluster update event happens, local object ref should be updated.
func TestUpdateEventUpdateLocalClusterObj(t *testing.T) {
	oldVersion := "123"
	newVersion := "321"

	oldObj := &api.EtcdCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: oldVersion,
			Name:            "test",
			Namespace:       metav1.NamespaceDefault,
		},
	}
	newObj := oldObj.DeepCopy()
	newObj.ResourceVersion = newVersion

	c := &Cluster{
		cluster: oldObj,
	}
	e := &clusterEvent{
		typ:     eventModifyCluster,
		cluster: newObj,
	}

	err := c.handleUpdateEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	if c.cluster.ResourceVersion != newVersion {
		t.Errorf("expect version=%s, get=%s", newVersion, c.cluster.ResourceVersion)
	}
}

func TestNewLongClusterName(t *testing.T) {
	clus := &api.EtcdCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-etcd-cluster123456789123456789123456789123456789123456",
			Namespace: metav1.NamespaceDefault,
		},
	}
	clus.SetClusterName("example-etcd-cluster123456789123456789123456789123456789123456")
	if c := New(Config{}, clus); c != nil {
		t.Errorf("expect c to be nil")
	}
}

func TestPollServices(t *testing.T) {
	clus := &api.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-etcd-cluster123456789123456789123456789123456789123456",
			Namespace: metav1.NamespaceDefault,
			UID:       "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
		},
	}

	tests := []struct {
		name     string
		services runtime.Object
		want     int
	}{
		{
			"should return 1 service",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						"etcd_cluster": clus.GetName(),
						"app":          "etcd",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "etcd.database.coreos.com/v1beta2",
							Kind:       "EtcdCluster",
							Name:       clus.GetName(),
							UID:        "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
						},
					},
				},
				Spec: v1.ServiceSpec{
					ClusterIP: "x.x.x.x",
				},
			},
			1,
		},
		{
			"should skip headless service",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "headless-service",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						"etcd_cluster": clus.GetName(),
						"app":          "etcd",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "etcd.database.coreos.com/v1beta2",
							Kind:       "EtcdCluster",
							Name:       clus.GetName(),
							UID:        "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
						},
					},
				},
				Spec: v1.ServiceSpec{
					ClusterIP: v1.ClusterIPNone,
				},
			},
			0,
		},
		{
			"should skip service without owner",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "headless-service",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						"etcd_cluster": clus.GetName(),
						"app":          "etcd",
					},
					OwnerReferences: []metav1.OwnerReference{},
				},
				Spec: v1.ServiceSpec{
					ClusterIP: "x.x.x.x",
				},
			},
			0,
		},
		{
			"should skip service without owner",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "headless-service",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						"etcd_cluster": clus.GetName(),
						"app":          "etcd",
					},
					OwnerReferences: []metav1.OwnerReference{},
				},
				Spec: v1.ServiceSpec{
					ClusterIP: "x.x.x.x",
				},
			},
			0,
		},
		{
			"should skip service with owner different UID",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "headless-service",
					Namespace: metav1.NamespaceDefault,
					Labels: map[string]string{
						"etcd_cluster": clus.GetName(),
						"app":          "etcd",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "etcd.database.coreos.com/v1beta2",
							Kind:       "EtcdCluster",
							Name:       clus.GetName(),
							UID:        "xxxxxxxx-xxxx-yyyy-xxxx-xxxxxxxxxxxx",
						},
					},
				},
				Spec: v1.ServiceSpec{
					ClusterIP: v1.ClusterIPNone,
				},
			},
			0,
		},
	}
	for _, tt := range tests {
		config := Config{
			KubeCli: fake.NewSimpleClientset(tt.services),
		}
		c := &Cluster{
			config:  config,
			cluster: clus,
			logger:  logrus.WithField("pkg", "test"),
		}
		services, _ := c.pollServices(context.Background())
		if len(services) != tt.want {
			t.Fatalf("%s: expected: %v, got: %v", tt.name, tt.want, len(services))
		}
	}
}
