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

package k8sutil

import (
	"context"
	"fmt"
	"strings"
	"testing"

	api "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

func TestDefaultBusyboxImageName(t *testing.T) {
	policy := &api.PodPolicy{}
	image := imageNameBusybox(policy)
	expected := defaultBusyboxImage
	if image != expected {
		t.Errorf("expect image=%s, get=%s", expected, image)
	}
}

func TestDefaultNilBusyboxImageName(t *testing.T) {
	image := imageNameBusybox(nil)
	expected := defaultBusyboxImage
	if image != expected {
		t.Errorf("expect image=%s, get=%s", expected, image)
	}
}

func TestSetBusyboxImageName(t *testing.T) {
	policy := &api.PodPolicy{
		BusyboxImage: "myRepo/busybox:1.3.2",
	}
	image := imageNameBusybox(policy)
	expected := "myRepo/busybox:1.3.2"
	if image != expected {
		t.Errorf("expect image=%s, get=%s", expected, image)
	}
}

func TestEtcdCommandNewLocalCluster(t *testing.T) {
	dataDir := "/var/etcd/data"
	etcdMember := &etcdutil.Member{
		Name:          "etcd-test",
		Namespace:     "etcd",
		SecurePeer:    false,
		SecureClient:  false,
		ClusterDomain: ".local",
	}
	memberSet := etcdutil.NewMemberSet(etcdMember).PeerURLPairs()
	clusterState := "new"
	token := "token"
	service := v1.Service{}

	initialEtcdCommand, _ := setupEtcdCommand(dataDir, etcdMember, strings.Join(memberSet, ","), clusterState, token, "", service)

	expectedCommand := "/usr/local/bin/etcd --data-dir=/var/etcd/data --name=etcd-test --initial-advertise-peer-urls=http://etcd-test.etcd.etcd.svc.local:2380 " +
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=http://etcd-test.etcd.etcd.svc.local:2379 " +
		"--initial-cluster=etcd-test=http://etcd-test.etcd.etcd.svc.local:2380 --initial-cluster-state=new --initial-cluster-token=token"

	if initialEtcdCommand != expectedCommand {
		t.Errorf("expected command=%s, got=%s", expectedCommand, initialEtcdCommand)
	}
}

//TODO
func TestEtcdCommandExistingLocalCluster(t *testing.T) {
	dataDir := "/var/etcd/data"
	etcdMember1 := &etcdutil.Member{
		Name:          "etcd-test-1",
		Namespace:     "etcd",
		SecurePeer:    false,
		SecureClient:  false,
		ClusterDomain: ".local",
	}
	etcdMember2 := &etcdutil.Member{
		Name:          "etcd-test-2",
		Namespace:     "etcd",
		SecurePeer:    false,
		SecureClient:  false,
		ClusterDomain: ".local",
	}
	memberSet := etcdutil.NewMemberSet(etcdMember1)
	memberSet.Add(etcdMember2)
	memberSetURLs := memberSet.PeerURLPairs()
	token := "token"
	clusterState := "existing"
	service := v1.Service{}

	initialEtcdCommand, _ := setupEtcdCommand(dataDir, etcdMember2, strings.Join(memberSetURLs, ","), clusterState, token, "", service)

	commandBeforeClusterSet := "/usr/local/bin/etcd --data-dir=/var/etcd/data --name=etcd-test-2 --initial-advertise-peer-urls=http://etcd-test-2.etcd-test.etcd.svc.local:2380 " +
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=http://etcd-test-2.etcd-test.etcd.svc.local:2379 "
	commandClusterSet1 := "--initial-cluster=etcd-test-1=http://etcd-test-1.etcd-test.etcd.svc.local:2380,etcd-test-2=http://etcd-test-2.etcd-test.etcd.svc.local:2380 --initial-cluster-state=existing"
	commandClusterSet2 := "--initial-cluster=etcd-test-2=http://etcd-test-2.etcd-test.etcd.svc.local:2380,etcd-test-1=http://etcd-test-1.etcd-test.etcd.svc.local:2380 --initial-cluster-state=existing"
	expectedCommand1 := commandBeforeClusterSet+commandClusterSet1
	expectedCommand2 := commandBeforeClusterSet+commandClusterSet2

	if initialEtcdCommand != expectedCommand1 && initialEtcdCommand != expectedCommand2{
		t.Errorf("wrong etcd command, got=%s", initialEtcdCommand)
	}
}

//ToDo
func TestEtcdCommandInvalidClusterMode(t *testing.T) {
	dataDir := "/var/etcd/data"
	etcdMember := &etcdutil.Member{
		Name:          "etcd-test",
		Namespace:     "etcd",
		SecurePeer:    false,
		SecureClient:  false,
		ClusterDomain: ".local",
	}
	memberSet := etcdutil.NewMemberSet(etcdMember).PeerURLPairs()
	clusterState := "new"
	token := "token"
	clusteringMode := "invalid"
	service := v1.Service{}

	initialEtcdCommand, _ := setupEtcdCommand(dataDir, etcdMember, strings.Join(memberSet, ","), clusterState, token, clusteringMode, service)

	expectedCommand := "/usr/local/bin/etcd --data-dir=/var/etcd/data --name=etcd-test --initial-advertise-peer-urls=http://etcd-test.etcd.etcd.svc.local:2380 " +
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=http://etcd-test.etcd.etcd.svc.local:2379 " +
		"--initial-cluster=etcd-test=http://etcd-test.etcd.etcd.svc.local:2380 --initial-cluster-state=new --initial-cluster-token=token"

	if initialEtcdCommand != expectedCommand {
		t.Errorf("expected command=%s, got=%s", expectedCommand, initialEtcdCommand)
	}
}

func TestEtcdCommandDiscoveryCluster(t *testing.T) {
	dataDir := "/var/etcd/data"
	etcdMember := &etcdutil.Member{
		Name:          "etcd-test",
		Namespace:     "etcd",
		SecurePeer:    false,
		SecureClient:  false,
		ClusterDomain: ".local",
	}
	memberSet := etcdutil.NewMemberSet(etcdMember).PeerURLPairs()
	clusterState := "new"
	clusterToken := "token"
	clusteringMode := "discovery"
	service := v1.Service{
		Spec : v1.ServiceSpec{
			ExternalName: "etcd-peer",
		},
	}

	initialEtcdCommand, _ := setupEtcdCommand(dataDir, etcdMember, strings.Join(memberSet, ","), clusterState, clusterToken, clusteringMode, service)

	expectedCommand := "/usr/local/bin/etcd --data-dir=/var/etcd/data --name=etcd-test --initial-advertise-peer-urls=http://etcd-peer:2380 " +
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=http://etcd-peer:2379 " +
		"--discovery=https://discovery.etcd.io/token"

	if initialEtcdCommand != expectedCommand {
		t.Errorf("expected command=%s, got=%s", expectedCommand, initialEtcdCommand)
	}
}

func TestCreateTokenLocalCluster(t *testing.T) {
	clusterSpec := &api.ClusterSpec{
		Size:           1,
		ClusteringMode: "local",
		ClusterToken:   "testtoken",
	}

	token, _ := CreateToken(*clusterSpec)

	if token == "testtoken" {
		t.Errorf("token should be a randon uuid, instead got %s", token)
	}
}

func TestCreateTokenDiscoveryClusterNoTokenSent(t *testing.T) {
	clusterSpec := &api.ClusterSpec{
		Size:           1,
		ClusteringMode: "discovery",
	}

	_, err := CreateToken(*clusterSpec)

	if err == nil {
		t.Errorf("Expected an error to be thrown when discovery mode on and no token is set")
	}
}

func TestCreateTokenDiscoveryClusterTokenEmpty(t *testing.T) {
	clusterSpec := &api.ClusterSpec{
		Size:           1,
		ClusteringMode: "discovery",
		ClusterToken: "",
	}

	_, err := CreateToken(*clusterSpec)

	if err == nil {
		t.Errorf("Expected an error to be thrown when discovery mode on and no token is set")
	}
}

func TestCreateTokenDiscoveryCluster(t *testing.T) {
	clusterSpec := &api.ClusterSpec{
		Size:           1,
		ClusteringMode: "discovery",
		ClusterToken:   "testtoken",
	}

	token, _ := CreateToken(*clusterSpec)

	if token != "testtoken" {
		t.Errorf("expected token=%s, got=%s", clusterSpec.ClusterToken, token)
	}
}

func TestCreateTokenNoMode(t *testing.T) {
	clusterSpec := &api.ClusterSpec{
		Size:           1,
		ClusterToken:   "testtoken",
	}

	token, _ := CreateToken(*clusterSpec)

	if token == "testtoken" {
		t.Errorf("expected random uiid token, got=%s", clusterSpec.ClusterToken)
	}
}

func TestLabelsForCluster(t *testing.T) {
	clusterName := "test-cluster"

	labels := LabelsForCluster(clusterName)

	if labels["etcd_cluster"] != "test-cluster" {
		t.Errorf("expected cluster name to be %s, got=%s", clusterName, labels["etcd_cluster"])
	}

	if labels["app"] != "etcd" {
		t.Errorf("expected cluster name to be %s, got=%s", clusterName, labels["app"])
	}
}

func TestNewEtcdServiceManifestLocalCluster(t *testing.T) {
	clusterName := "test-cluster"
	svcName := "test-cluster-svc"
	ports := []v1.ServicePort{{
		Name:       "http-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}} 
	publishNotReadyAddresses := true
	annotations := map[string]string{
		"my-annotation": "its value",
	}

	svc := newEtcdServiceManifest(svcName, clusterName, ports, publishNotReadyAddresses, annotations)

	if svc.ObjectMeta.Name != svcName {
		t.Errorf("expected service name to be %s, got=%s", svcName, svc.ObjectMeta.Name)
	}
	if svc.ObjectMeta.Labels["etcd_cluster"] != clusterName {
		t.Errorf("expected label name to be %s, got=%s", clusterName, svc.ObjectMeta.Labels["etcd_cluster"])
	}
	if svc.ObjectMeta.Annotations["my-annotation"] != "its value" {
		t.Errorf("expected label name to be %s, got=%s", "its value", svc.ObjectMeta.Annotations["my-annotation"])
	}
	if len(svc.Spec.Ports) != 1 {
		t.Errorf("expected 1 service ports name, got=%d", len(svc.Spec.Ports))
	}
	if svc.Spec.Selector == nil {
		t.Errorf("selector not correctly assigned")
	}
	if svc.Spec.Selector["etcd_cluster"] != "test-cluster" {
		t.Errorf("expected selector name to be %s, got=%s", clusterName, svc.Spec.Selector["etcd_cluster"])
	}
	if svc.Spec.PublishNotReadyAddresses != publishNotReadyAddresses {
		t.Errorf("expected selector name to be %t, got=%t", publishNotReadyAddresses, svc.Spec.PublishNotReadyAddresses)
	}
}

// Peer service tests
func TestCreatePeerServiceNoTLSLocalCluster(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := clusterName
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := "local"
	expectedPorts := []v1.ServicePort{{
		Name:       "http-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}, {
		Name:       "peer",
		Port:       2380,
		TargetPort: intstr.FromInt(2380),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedService := &api.ServicePolicy{
		Type:      v1.ServiceTypeClusterIP,
		ClusterIP: v1.ClusterIPNone,
	}
	expectedAnnotations := map[string]string{}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != clusterName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		return nil
	}
	
	res := CreatePeerService(ctx, nil, clusterName, ns, owner, tls, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreatePeerServiceNoTLSNoClusteringMode(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := clusterName
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := ""
	expectedPorts := []v1.ServicePort{{
		Name:       "http-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}, {
		Name:       "peer",
		Port:       2380,
		TargetPort: intstr.FromInt(2380),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedService := &api.ServicePolicy{
		Type:      v1.ServiceTypeClusterIP,
		ClusterIP: v1.ClusterIPNone,
	}
	expectedAnnotations := map[string]string{}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != clusterName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		return nil
	}
	
	res := CreatePeerService(ctx, nil, clusterName, ns, owner, tls, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreatePeerServiceWithTLSLocalCluster(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := clusterName
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := true
	clusteringMode := "local"
	expectedPorts := []v1.ServicePort{{
		Name:       "https-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}, {
		Name:       "peer",
		Port:       2380,
		TargetPort: intstr.FromInt(2380),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedService := &api.ServicePolicy{
		Type:      v1.ServiceTypeClusterIP,
		ClusterIP: v1.ClusterIPNone,
	}
	expectedAnnotations := map[string]string{}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != clusterName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		return nil
	}
	
	res := CreatePeerService(ctx, nil, clusterName, ns, owner, tls, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreatePeerServiceDiscoveryCluster(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := clusterName
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := "discovery"
	expectedPorts := []v1.ServicePort{{
		Name:       "http-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}, {
		Name:       "peer",
		Port:       2380,
		TargetPort: intstr.FromInt(2380),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedService := &api.ServicePolicy{
		Type:      v1.ServiceTypeLoadBalancer,
	}
	expectedAnnotations := map[string]string{
		"service.beta.kubernetes.io/aws-load-balancer-nlb-target-type": "instance",
    	"service.beta.kubernetes.io/aws-load-balancer-type": "external",
	}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != clusterName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		return nil
	}
	
	res := CreatePeerService(ctx, nil, clusterName, ns, owner, tls, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

// Client service tests
func TestCreateClientServiceNoName(t *testing.T) {
	ctx := context.TODO()
	clusterName := "test-cluster"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	tls := false

	res := CreateClientService(ctx, nil, "", clusterName, ns, owner, tls, nil, "", nil)
	if res == nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceNoTLSLocalClusterNilService(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := "local"
	expectedPorts := []v1.ServicePort{{
		Name:       "http-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedAnnotations := map[string]string{}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if service != nil{
			return fmt.Errorf("expected service to be %v, got=%v", nil, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, nil, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceWithTLSLocalClusterNilService(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := true
	clusteringMode := "local"
	expectedPorts := []v1.ServicePort{{
		Name:       "https-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedAnnotations := map[string]string{}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if service != nil{
			return fmt.Errorf("expected service to be %v, got=%v", nil, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, nil, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceLocalClusterDefinedService(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := "local"
	ports := []v1.ServicePort{{
		Name:       "client",
      	Port: 		2379,
      	TargetPort: intstr.FromInt(2379),
	}}
	annotations := map[string]string{
		"service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout": "3600",
		"external-dns.alpha.kubernetes.io/hostname": "etcd-china.tennis.us-east-1.general.prod.wildlife.io",
		"external-dns.alpha.kubernetes.io/manage": "true",
	}
	service := &api.ServicePolicy {
		Name: 		 "etcd-cluster-client",
		Annotations: annotations,
		Type: 		 v1.ServiceTypeLoadBalancer,
		ClientPorts: ports,
		ClusterIP: 	 v1.ClusterIPNone,
	}
	expectedPorts := ports
	expectedAnnotations := annotations
	expectedService := &api.ServicePolicy{
		Name: 		 "etcd-cluster-client",
		Type:      	 v1.ServiceTypeLoadBalancer,
		ClientPorts: expectedPorts,
		Annotations: expectedAnnotations,
		ClusterIP:   v1.ClusterIPNone,
	}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		if publishNotReadyAddresses == true {
			return fmt.Errorf("expected publishNotReadyAddresses to be %v, got=%v", false, publishNotReadyAddresses)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, service, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceLocalClusterDefinedServiceNoPorts(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := "local"
	annotations := map[string]string{
		"service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout": "3600",
		"external-dns.alpha.kubernetes.io/hostname": "etcd-china.tennis.us-east-1.general.prod.wildlife.io",
		"external-dns.alpha.kubernetes.io/manage": "true",
	}
	service := &api.ServicePolicy {
		Name: 		 "etcd-cluster-client",
		Annotations: annotations,
		Type: 		 v1.ServiceTypeLoadBalancer,
		ClusterIP: 	 v1.ClusterIPNone,
	}
	expectedPorts := []v1.ServicePort{{
		Name:       "http-client",
      	Port: 		2379,
      	TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedAnnotations := annotations
	expectedService := &api.ServicePolicy{
		Name: 		 "etcd-cluster-client",
		Type:      	 v1.ServiceTypeLoadBalancer,
		Annotations: expectedAnnotations,
		ClusterIP:   v1.ClusterIPNone,
	}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		if publishNotReadyAddresses == true {
			return fmt.Errorf("expected publishNotReadyAddresses to be %v, got=%v", false, publishNotReadyAddresses)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, service, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceNoClusteringModeNilService(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := ""
	expectedPorts := []v1.ServicePort{{
		Name:       "http-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedAnnotations := map[string]string{}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if service != nil{
			return fmt.Errorf("expected service to be %v, got=%v", nil, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, nil, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceNoClusteringModeDefinedService(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := ""
	ports := []v1.ServicePort{{
		Name:       "client",
      	Port: 		2379,
      	TargetPort: intstr.FromInt(2379),
	}}
	annotations := map[string]string{
		"service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout": "3600",
		"external-dns.alpha.kubernetes.io/hostname": "etcd-china.tennis.us-east-1.general.prod.wildlife.io",
		"external-dns.alpha.kubernetes.io/manage": "true",
	}
	service := &api.ServicePolicy {
		Name: 		 "etcd-cluster-client",
		Annotations: annotations,
		Type: 		 v1.ServiceTypeLoadBalancer,
		ClientPorts: ports,
		ClusterIP: 	 v1.ClusterIPNone,
	}
	expectedPorts := ports
	expectedAnnotations := annotations
	expectedService := &api.ServicePolicy{
		Name: 		 "etcd-cluster-client",
		Type:      	 v1.ServiceTypeLoadBalancer,
		ClientPorts: expectedPorts,
		Annotations: expectedAnnotations,
		ClusterIP:   v1.ClusterIPNone,
	}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		if publishNotReadyAddresses == true {
			return fmt.Errorf("expected publishNotReadyAddresses to be %v, got=%v", false, publishNotReadyAddresses)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, service, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceDiscoveryClusterNilService(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := "discovery"
	expectedPorts := []v1.ServicePort{{
		Name:       "http-client",
		Port:       2379,
		TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedService := &api.ServicePolicy{
		Type:      v1.ServiceTypeLoadBalancer,
	}
	expectedAnnotations := map[string]string{
		"service.beta.kubernetes.io/aws-load-balancer-nlb-target-type": "instance",
    	"service.beta.kubernetes.io/aws-load-balancer-type": "external",
    	"service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
	}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, nil, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceDiscoveryClusterDefinedService(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := "discovery"
	ports := []v1.ServicePort{{
		Name:       "client",
      	Port: 		2379,
      	TargetPort: intstr.FromInt(2379),
	}}
	annotations := map[string]string{
		"service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout": "3600",
		"external-dns.alpha.kubernetes.io/hostname": "etcd-china.tennis.us-east-1.general.prod.wildlife.io",
		"external-dns.alpha.kubernetes.io/manage": "true",
	}
	service := &api.ServicePolicy {
		Name: 		 "etcd-cluster-client",
		Annotations: annotations,
		Type: 		 v1.ServiceTypeLoadBalancer,
		ClientPorts: ports,
		ClusterIP: 	 v1.ClusterIPNone,
	}
	expectedPorts := ports
	expectedAnnotations := annotations
	expectedService := &api.ServicePolicy{
		Name: 		 "etcd-cluster-client",
		Type:      	 v1.ServiceTypeLoadBalancer,
		ClientPorts: expectedPorts,
		Annotations: expectedAnnotations,
		ClusterIP:   v1.ClusterIPNone,
	}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		if publishNotReadyAddresses == true {
			return fmt.Errorf("expected publishNotReadyAddresses to be %v, got=%v", false, publishNotReadyAddresses)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, service, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}

func TestCreateClientServiceDiscoveryClusterDefinedServiceNoPorts(t *testing.T) {
	clusterName := "test-cluster"
	serviceName := "test-cluster-client"
	ns := "etcd"
	owner := metav1.OwnerReference{}
	ctx := context.TODO()
	tls := false
	clusteringMode := "discovery"
	annotations := map[string]string{
		"service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout": "3600",
		"external-dns.alpha.kubernetes.io/hostname": "etcd-china.tennis.us-east-1.general.prod.wildlife.io",
		"external-dns.alpha.kubernetes.io/manage": "true",
	}
	service := &api.ServicePolicy {
		Name: 		 "etcd-cluster-client",
		Annotations: annotations,
		Type: 		 v1.ServiceTypeLoadBalancer,
		ClusterIP: 	 v1.ClusterIPNone,
	}
	expectedPorts := []v1.ServicePort{{
		Name:       "http-client",
      	Port: 		2379,
      	TargetPort: intstr.FromInt(2379),
		Protocol:   v1.ProtocolTCP,
	}}
	expectedAnnotations := annotations
	expectedService := &api.ServicePolicy{
		Name: 		 "etcd-cluster-client",
		Type:      	 v1.ServiceTypeLoadBalancer,
		Annotations: expectedAnnotations,
		ClusterIP:   v1.ClusterIPNone,
	}

	createSvc := func(ctx context.Context, kubecli kubernetes.Interface, svcName string, clstrName string, ns string, ports []v1.ServicePort, owner metav1.OwnerReference, publishNotReadyAddresses bool, service *api.ServicePolicy, annotations map[string]string) error {		
		if svcName != serviceName {
			return fmt.Errorf("expected service name to be %s, got=%s", serviceName, svcName)
		}
		if clstrName != clusterName {
			return fmt.Errorf("expected cluster name to be %s, got=%s", clusterName, clstrName)
		}
		if diff := cmp.Diff(expectedPorts, ports); diff != ""{
			return fmt.Errorf("expected ports to be %v, got=%v", expectedPorts, ports)
		}
		if diff := cmp.Diff(expectedService, service); diff != ""{
			return fmt.Errorf("expected service to be %v, got=%v", expectedService, service)
		}
		if diff := cmp.Diff(expectedAnnotations, annotations); diff != ""{
			return fmt.Errorf("expected annotations to be %v, got=%v", expectedAnnotations, annotations)
		}
		if publishNotReadyAddresses == true {
			return fmt.Errorf("expected publishNotReadyAddresses to be %v, got=%v", false, publishNotReadyAddresses)
		}
		return nil
	}
	
	res := CreateClientService(ctx, nil, serviceName, clusterName, ns, owner, tls, service, clusteringMode, createSvc)

	if res != nil {
		t.Errorf("Got an error from tested function: %s", res.Error())
	}
}
