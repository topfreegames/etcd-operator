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
	"testing"

	"reflect"

	api "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApplyNilServicePolicy(t *testing.T) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "service",
			Annotations: map[string]string{},
		},
	}
	var policy *api.ServicePolicy = nil
	expected := svc.GetObjectMeta().GetAnnotations()
	applyServicePolicy(svc, policy)
	actual := svc.GetObjectMeta().GetAnnotations()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expect expected=%v, got=%v", expected, actual)
	}
}

func TestApplyEmptyServicePolicy(t *testing.T) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "service",
			Annotations: map[string]string{},
		},
	}
	policy := &api.ServicePolicy{}
	expected := svc.GetObjectMeta().GetAnnotations()
	applyServicePolicy(svc, policy)
	actual := svc.GetObjectMeta().GetAnnotations()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expect expected=%v, got=%v", expected, actual)
	}
}

func TestApplyEmptyAnnotationsServicePolicy(t *testing.T) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "service",
			Annotations: map[string]string{},
		},
	}
	policy := &api.ServicePolicy{
		Annotations: map[string]string{},
	}
	expected := svc.GetObjectMeta().GetAnnotations()
	applyServicePolicy(svc, policy)
	actual := svc.GetObjectMeta().GetAnnotations()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expect expected=%v, got=%v", expected, actual)
	}
}

func TestApplyServicePolicyWithAnnotation(t *testing.T) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "service",
			Annotations: map[string]string{},
		},
	}
	annotations := map[string]string{
		"key1": "value2",
		"key2": "value2",
	}

	policy := &api.ServicePolicy{
		Annotations: annotations,
	}
	applyServicePolicy(svc, policy)
	actual := svc.GetObjectMeta().GetAnnotations()
	if !reflect.DeepEqual(annotations, actual) {
		t.Errorf("expect expected=%v, got=%v", annotations, actual)
	}
}

func TestApplyServicePolicyEmptySelector(t *testing.T) {
	selector := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "service",
		},
		Spec: v1.ServiceSpec{
			Selector: selector,
		},
	}
	policy := &api.ServicePolicy{
		Selector: map[string]string{},
	}
	applyServicePolicy(svc, policy)
	actual := svc.Spec.Selector
	if !reflect.DeepEqual(selector, actual) {
		t.Errorf("expect expected=%v, got=%v", selector, actual)
	}
}

func TestApplyServicePolicyWithSelector(t *testing.T) {
	selector := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "service",
		},
		Spec: v1.ServiceSpec{
			Selector: selector,
		},
	}
	customSelector := map[string]string{
		"key3": "value3",
		"key4": "value4",
	}

	policy := &api.ServicePolicy{
		Selector: customSelector,
	}
	applyServicePolicy(svc, policy)
	actual := svc.Spec.Selector
	if !reflect.DeepEqual(customSelector, actual) {
		t.Errorf("expect expected=%v, got=%v", selector, actual)
	}
}

func TestApplyServicePolicyWithClusterIP(t *testing.T) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "service",
		},
		Spec: v1.ServiceSpec{},
	}

	serviceType := v1.ServiceTypeClusterIP

	clusterIP := "10.x.x.x"

	policy := &api.ServicePolicy{
		Type:      serviceType,
		ClusterIP: clusterIP,
	}
	applyServicePolicy(svc, policy)
	actualType := svc.Spec.Type
	actualClusterIP := svc.Spec.Type
	if !reflect.DeepEqual(serviceType, actualType) {
		t.Errorf("expect expected=%v, got=%v", serviceType, actualType)
	}

	if !reflect.DeepEqual(clusterIP, actualClusterIP) {
		t.Errorf("expect expected=%v, got=%v", clusterIP, actualClusterIP)
	}
}
