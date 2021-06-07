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

package cluster

import (
	"context"
	"fmt"

	api "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/coreos/etcd-operator/pkg/util/k8sutil"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ErrLostQuorum indicates that the etcd cluster lost its quorum.
var ErrLostQuorum = newFatalError("lost quorum")

// reconcile reconciles cluster current state to desired state specified by spec.
// - it tries to reconcile the cluster to desired size.
// - if the cluster needs for upgrade, it tries to upgrade old member one by one.
func (c *Cluster) reconcile(ctx context.Context, pods []*v1.Pod, services map[string]*v1.Service) error {
	c.logger.Infoln("Start reconciling")
	defer c.logger.Infoln("Finish reconciling")

	defer func() {
		c.status.Size = c.members.Size()
	}()

	sp := c.cluster.Spec
	if c.cluster.Spec.Services != nil {
		c.reconcileServices(ctx, services)
		c.status.SetClusterServiceName(services)
	}

	running := podsToMemberSet(pods, c.isSecureClient())
	if !running.IsEqual(c.members) || c.members.Size() != sp.Size {
		return c.reconcileMembers(ctx, running)
	}
	c.status.ClearCondition(api.ClusterConditionScaling)

	if needUpgrade(pods, sp) {
		c.status.UpgradeVersionTo(sp.Version)

		m := pickOneOldMember(pods, sp.Version)
		return c.upgradeOneMember(ctx, m.Name)
	}
	c.status.ClearCondition(api.ClusterConditionUpgrading)

	c.status.SetVersion(sp.Version)
	c.status.SetReadyCondition()

	return nil
}

// reconcileMembers reconciles
// - running pods on k8s and cluster membership
// - cluster membership and expected size of etcd cluster
// Steps:
// 1. Remove all pods from running set that does not belong to member set.
// 2. L consist of remaining pods of runnings
// 3. If L = members, the current state matches the membership state. END.
// 4. If len(L) < len(members)/2 + 1, return quorum lost error.
// 5. Add one missing member. END.
func (c *Cluster) reconcileMembers(ctx context.Context, running etcdutil.MemberSet) error {
	c.logger.Infof("running members: %s", running)
	c.logger.Infof("cluster membership: %s", c.members)

	unknownMembers := running.Diff(c.members)
	if unknownMembers.Size() > 0 {
		c.logger.Infof("removing unexpected pods: %v", unknownMembers)
		for _, m := range unknownMembers {
			if err := c.removePod(ctx, m.Name); err != nil {
				return err
			}
		}
	}
	L := running.Diff(unknownMembers)

	if L.Size() == c.members.Size() {
		return c.resize(ctx)
	}

	if L.Size() < c.members.Size()/2+1 {
		return ErrLostQuorum
	}

	c.logger.Infof("removing one dead member")
	// remove dead members that doesn't have any running pods before doing resizing.
	return c.removeDeadMember(ctx, c.members.Diff(L).PickOne())
}

func (c *Cluster) diffServices(services map[string]*v1.Service) ([]*api.ServicePolicy, map[string]*v1.Service, error) {
	newServices := []*api.ServicePolicy{}
	unknownServices := make(map[string]*v1.Service)
	k8sutil.DeepCopyMap(services, unknownServices)

	for _, svcPolicy := range c.cluster.Spec.Services {
		if _, ok := services[svcPolicy.Name]; !ok {
			newServices = append(newServices, svcPolicy)
		}
		delete(unknownServices, svcPolicy.Name)
	}
	return newServices, unknownServices, nil
}

func (c *Cluster) reconcileServices(ctx context.Context, services map[string]*v1.Service) error {

	newServices, unknownServices, err := c.diffServices(services)
	if err != nil {
		return err
	}

	if len(newServices) > 0 {
		for _, service := range newServices {
			err := k8sutil.CreateClientService(ctx, c.config.KubeCli, service.Name, c.cluster.GetName(), c.cluster.GetNamespace(), c.cluster.AsOwner(), c.isSecureClient(), service)
			if err != nil {
				return err
			}
		}
	}

	if len(unknownServices) > 0 {
		for _, value := range unknownServices {
			err := c.removeService(ctx, value.GetName())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Cluster) resize(ctx context.Context) error {
	if c.members.Size() == c.cluster.Spec.Size {
		return nil
	}

	if c.members.Size() < c.cluster.Spec.Size {
		return c.addOneMember(ctx)
	}

	return c.removeOneMember(ctx)
}

func (c *Cluster) addOneMember(ctx context.Context) error {
	c.status.SetScalingUpCondition(c.members.Size(), c.cluster.Spec.Size)

	cfg := clientv3.Config{
		Endpoints:   c.members.ClientURLs(),
		DialTimeout: constants.DefaultDialTimeout,
		TLS:         c.tlsConfig,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("add one member failed: creating etcd client failed %v", err)
	}
	defer etcdcli.Close()

	newMember := c.newMember()
	memberCtx, cancel := context.WithTimeout(ctx, constants.DefaultRequestTimeout)
	resp, err := etcdcli.MemberAdd(memberCtx, []string{newMember.PeerURL()})
	cancel()
	if err != nil {
		return fmt.Errorf("fail to add new member (%s): %v", newMember.Name, err)
	}
	newMember.ID = resp.Member.ID
	c.members.Add(newMember)

	if err := c.createPod(ctx, c.members, newMember, "existing"); err != nil {
		return fmt.Errorf("fail to create member's pod (%s): %v", newMember.Name, err)
	}
	c.logger.Infof("added member (%s)", newMember.Name)
	_, err = c.eventsCli.Create(ctx, k8sutil.NewMemberAddEvent(newMember.Name, c.cluster), metav1.CreateOptions{})
	if err != nil {
		c.logger.Errorf("failed to create new member add event: %v", err)
	}
	return nil
}

func (c *Cluster) removeOneMember(ctx context.Context) error {
	c.status.SetScalingDownCondition(c.members.Size(), c.cluster.Spec.Size)

	return c.removeMember(ctx, c.members.PickOne())
}

func (c *Cluster) removeDeadMember(ctx context.Context, toRemove *etcdutil.Member) error {
	c.logger.Infof("removing dead member %q", toRemove.Name)
	_, err := c.eventsCli.Create(ctx, k8sutil.ReplacingDeadMemberEvent(toRemove.Name, c.cluster), metav1.CreateOptions{})
	if err != nil {
		c.logger.Errorf("failed to create replacing dead member event: %v", err)
	}

	return c.removeMember(ctx, toRemove)
}

func (c *Cluster) removeMember(ctx context.Context, toRemove *etcdutil.Member) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("remove member (%s) failed: %v", toRemove.Name, err)
		}
	}()

	err = etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, toRemove.ID)
	if err != nil {
		switch err {
		case rpctypes.ErrMemberNotFound:
			c.logger.Infof("etcd member (%v) has been removed", toRemove.Name)
		default:
			return err
		}
	}
	c.members.Remove(toRemove.Name)
	_, err = c.eventsCli.Create(ctx, k8sutil.MemberRemoveEvent(toRemove.Name, c.cluster), metav1.CreateOptions{})
	if err != nil {
		c.logger.Errorf("failed to create remove member event: %v", err)
	}
	if err := c.removePod(ctx, toRemove.Name); err != nil {
		return err
	}
	if c.isPodPVEnabled() {
		err = c.removePVC(ctx, k8sutil.PVCNameFromMember(toRemove.Name))
		if err != nil {
			return err
		}
	}
	c.logger.Infof("removed member (%v) with ID (%d)", toRemove.Name, toRemove.ID)
	return nil
}

func (c *Cluster) removePVC(ctx context.Context, pvcName string) error {
	err := c.config.KubeCli.CoreV1().PersistentVolumeClaims(c.cluster.Namespace).Delete(ctx, pvcName, *metav1.NewDeleteOptions(0))
	if err != nil && !k8sutil.IsKubernetesResourceNotFoundError(err) {
		return fmt.Errorf("remove pvc (%s) failed: %v", pvcName, err)
	}
	return nil
}

func needUpgrade(pods []*v1.Pod, cs api.ClusterSpec) bool {
	return len(pods) == cs.Size && pickOneOldMember(pods, cs.Version) != nil
}

func pickOneOldMember(pods []*v1.Pod, newVersion string) *etcdutil.Member {
	for _, pod := range pods {
		if k8sutil.GetEtcdVersion(pod) == newVersion {
			continue
		}
		return &etcdutil.Member{Name: pod.Name, Namespace: pod.Namespace}
	}
	return nil
}
