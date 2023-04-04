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
	"strings"
	"testing"

	api "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
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

	initialEtcdCommand, _ := setupEtcdCommand(dataDir, etcdMember, strings.Join(memberSet, ","), clusterState, token, "")

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

	initialEtcdCommand, _ := setupEtcdCommand(dataDir, etcdMember2, strings.Join(memberSetURLs, ","), clusterState, token, "")

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

	initialEtcdCommand, _ := setupEtcdCommand(dataDir, etcdMember, strings.Join(memberSet, ","), clusterState, token, clusteringMode)

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

	initialEtcdCommand, _ := setupEtcdCommand(dataDir, etcdMember, strings.Join(memberSet, ","), clusterState, clusterToken, clusteringMode)

	expectedCommand := "/usr/local/bin/etcd --data-dir=/var/etcd/data --name=etcd-test --initial-advertise-peer-urls=http://etcd-test.etcd.etcd.svc.local:2380 " +
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=http://etcd-test.etcd.etcd.svc.local:2379 " +
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

func TestCreateTokenDistributedCluster(t *testing.T) {
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