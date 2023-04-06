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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	api "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	"github.com/coreos/etcd-operator/pkg/generated/clientset/versioned"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/coreos/etcd-operator/pkg/util/k8sutil"
	"github.com/coreos/etcd-operator/pkg/util/retryutil"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var (
	reconcileInterval         = 8 * time.Second
	podTerminationGracePeriod = int64(5)
	errAllPodsDead            = newFatalError("all etcd pods are dead.")
)

type clusterEventType string

const (
	eventModifyCluster clusterEventType = "Modify"
)

type clusterEvent struct {
	typ     clusterEventType
	cluster *api.EtcdCluster
}

type Config struct {
	ServiceAccount string

	KubeCli   kubernetes.Interface
	EtcdCRCli versioned.Interface
}

type Cluster struct {
	logger *logrus.Entry

	config Config

	cluster *api.EtcdCluster

	// in memory state of the cluster
	// status is the source of truth after Cluster struct is materialized.
	status api.ClusterStatus

	eventCh chan *clusterEvent
	stopCh  chan struct{}

	// members repsersents the members in the etcd cluster.
	// the name of the member is the the name of the pod the member
	// process runs in.
	members etcdutil.MemberSet

	tlsConfig *tls.Config

	eventsCli corev1.EventInterface
}

func New(config Config, cl *api.EtcdCluster) *Cluster {
	lg := logrus.WithField("pkg", "cluster").WithField("cluster-name", cl.Name).WithField("cluster-namespace", cl.Namespace)
	if len(cl.Name) > k8sutil.MaxNameLength || len(cl.ClusterName) > k8sutil.MaxNameLength {
		return nil
	}

	c := &Cluster{
		logger:    lg,
		config:    config,
		cluster:   cl,
		eventCh:   make(chan *clusterEvent, 100),
		stopCh:    make(chan struct{}),
		status:    *(cl.Status.DeepCopy()),
		eventsCli: config.KubeCli.CoreV1().Events(cl.Namespace),
	}

	go func() {
		if err := c.setup(context.TODO()); err != nil {
			c.logger.Errorf("cluster failed to setup: %v", err)
			if c.status.Phase != api.ClusterPhaseFailed {
				c.status.SetReason(err.Error())
				c.status.SetPhase(api.ClusterPhaseFailed)
				if err := c.updateCRStatus(context.TODO()); err != nil {
					c.logger.Errorf("failed to update cluster phase (%v): %v", api.ClusterPhaseFailed, err)
				}
			}
			return
		}
		c.run(context.TODO())
	}()

	return c
}

func (c *Cluster) setup(ctx context.Context) error {
	var shouldCreateCluster bool
	switch c.status.Phase {
	case api.ClusterPhaseNone:
		shouldCreateCluster = true
	case api.ClusterPhaseCreating:
		return errCreatedCluster
	case api.ClusterPhaseRunning:
		shouldCreateCluster = false

	default:
		return fmt.Errorf("unexpected cluster phase: %s", c.status.Phase)
	}

	if c.isSecureClient() {
		timeoutRetry := 10 * time.Second
		timeoutInterval := 30
		for i := 1; ; i++ {
			d, err := k8sutil.GetTLSDataFromSecret(ctx, c.config.KubeCli, c.cluster.Namespace, c.cluster.Spec.TLS.Static.OperatorSecret)
			if err == nil {
				c.tlsConfig, err = etcdutil.NewTLSConfig(d.CertData, d.KeyData, d.CAData)
			}
			if err != nil {
				if i > timeoutInterval {
					return err
				}
				c.logger.Warningf("TLS Setup Failed: %v (Retry %d/%d)", err, i, timeoutInterval)
			} else {
				break
			}
			time.Sleep(timeoutRetry)
		}
	}

	if shouldCreateCluster {
		return c.create(ctx)
	}
	return nil
}

func (c *Cluster) create(ctx context.Context) error {
	c.status.SetPhase(api.ClusterPhaseCreating)

	if err := c.updateCRStatus(ctx); err != nil {
		return fmt.Errorf("cluster create: failed to update cluster phase (%v): %v", api.ClusterPhaseCreating, err)
	}
	c.logClusterCreation()

	return c.prepareSeedMember(ctx)
}

func (c *Cluster) prepareSeedMember(ctx context.Context) error {
	c.status.SetScalingUpCondition(0, c.cluster.Spec.Size)

	err := c.bootstrap(ctx)
	if err != nil {
		return err
	}

	c.status.Size = 1
	return nil
}

func (c *Cluster) Delete() {
	c.logger.Info("cluster is deleted by user")
	close(c.stopCh)
}

func (c *Cluster) send(ev *clusterEvent) {
	select {
	case c.eventCh <- ev:
		l, ecap := len(c.eventCh), cap(c.eventCh)
		if l > int(float64(ecap)*0.8) {
			c.logger.Warningf("eventCh buffer is almost full [%d/%d]", l, ecap)
		}
	case <-c.stopCh:
	}
}

func (c *Cluster) run(ctx context.Context) {
	if err := c.setupServices(ctx); err != nil {
		c.logger.Errorf("fail to setup etcd services: %v", err)
	}

	c.status.ClientPort = k8sutil.EtcdClientPort

	c.status.SetPhase(api.ClusterPhaseRunning)
	if err := c.updateCRStatus(ctx); err != nil {
		c.logger.Warningf("update initial CR status failed: %v", err)
	}
	c.logger.Infof("start running...")

	var rerr error
	for {
		select {
		case <-c.stopCh:
			return
		case event := <-c.eventCh:
			switch event.typ {
			case eventModifyCluster:
				err := c.handleUpdateEvent(event)
				if err != nil {
					c.logger.Errorf("handle update event failed: %v", err)
					c.status.SetReason(err.Error())
					c.reportFailedStatus(ctx)
					return
				}
			default:
				panic("unknown event type" + event.typ)
			}

		case <-time.After(reconcileInterval):
			start := time.Now()

			if c.cluster.Spec.Paused {
				c.status.PauseControl()
				c.logger.Infof("control is paused, skipping reconciliation")
				continue
			} else {
				c.status.Control()
			}

			running, pending, err := c.pollPods(ctx)
			if err != nil {
				c.logger.Errorf("fail to poll pods: %v", err)
				reconcileFailed.WithLabelValues("failed to poll pods").Inc()
				continue
			}

			if len(pending) > 0 {
				// Pod startup might take long, e.g. pulling image. It would deterministically become running or succeeded/failed later.
				c.logger.Infof("skip reconciliation: running (%v), pending (%v)", k8sutil.GetPodNames(running), k8sutil.GetPodNames(pending))
				reconcileFailed.WithLabelValues("not all pods are running").Inc()
				continue
			}
			if len(running) == 0 {
				// All pods are dead, cluster cannot recover so return error
				rerr = errAllPodsDead
				break
			}

			// On controller restore, we could have "members == nil"
			if rerr != nil || c.members == nil {
				rerr = c.updateMembers(podsToMemberSet(running, c.isSecureClient()))
				if rerr != nil {
					c.logger.Errorf("failed to update members: %v", rerr)
					break
				}
			}

			services, err := c.pollServices(ctx)
			if err != nil {
				c.logger.Errorf("fail to poll services: %v", err)
				reconcileFailed.WithLabelValues("failed to poll services").Inc()
				continue
			}

			rerr = c.reconcile(ctx, running, services)
			if rerr != nil {
				c.logger.Errorf("failed to reconcile: %v", rerr)
				break
			}
			c.updateMemberStatus(running)
			if err := c.updateCRStatus(ctx); err != nil {
				c.logger.Warningf("periodic update CR status failed: %v", err)
			}

			reconcileHistogram.WithLabelValues(c.name()).Observe(time.Since(start).Seconds())
		}

		if rerr != nil {
			reconcileFailed.WithLabelValues(rerr.Error()).Inc()
		}

		if isFatalError(rerr) {
			c.status.SetReason(rerr.Error())
			c.logger.Errorf("cluster failed: %v", rerr)
			c.reportFailedStatus(ctx)
			return
		}
	}
}

func (c *Cluster) handleUpdateEvent(event *clusterEvent) error {
	oldSpec := c.cluster.Spec.DeepCopy()
	c.cluster = event.cluster

	if isSpecEqual(event.cluster.Spec, *oldSpec) {
		// We have some fields that once created could not be mutated.
		if !reflect.DeepEqual(event.cluster.Spec, *oldSpec) {
			c.logger.Infof("ignoring update event: %#v", event.cluster.Spec)
		}
		return nil
	}
	// TODO: we can't handle another upgrade while an upgrade is in progress

	c.logSpecUpdate(*oldSpec, event.cluster.Spec)
	return nil
}

func isSpecEqual(s1, s2 api.ClusterSpec) bool {
	if s1.Size != s2.Size || s1.Paused != s2.Paused || s1.Version != s2.Version {
		return false
	}
	if !reflect.DeepEqual(s1.Services, s2.Services) {
		return false
	}

	return true
}

func (c *Cluster) startSeedMember(ctx context.Context) error {
	m := &etcdutil.Member{
		Name:         k8sutil.UniqueMemberName(c.cluster.Name),
		Namespace:    c.cluster.Namespace,
		SecurePeer:   c.isSecurePeer(),
		SecureClient: c.isSecureClient(),
	}
	if c.cluster.Spec.Pod != nil {
		m.ClusterDomain = c.cluster.Spec.Pod.ClusterDomain
	}
	ms := etcdutil.NewMemberSet(m)
	if err := c.createPod(ctx, ms, m, "new"); err != nil {
		return fmt.Errorf("failed to create seed member (%s): %v", m.Name, err)
	}
	c.members = ms
	c.logger.Infof("cluster created with seed member (%s)", m.Name)
	_, err := c.eventsCli.Create(ctx, k8sutil.NewMemberAddEvent(m.Name, c.cluster), metav1.CreateOptions{})
	if err != nil {
		c.logger.Errorf("failed to create new member add event: %v", err)
	}

	return nil
}

func (c *Cluster) isSecurePeer() bool {
	return c.cluster.Spec.TLS.IsSecurePeer()
}

func (c *Cluster) isSecureClient() bool {
	return c.cluster.Spec.TLS.IsSecureClient()
}

// bootstrap creates the seed etcd member for a new cluster.
func (c *Cluster) bootstrap(ctx context.Context) error {
	return c.startSeedMember(ctx)
}

func (c *Cluster) Update(cl *api.EtcdCluster) {
	c.send(&clusterEvent{
		typ:     eventModifyCluster,
		cluster: cl,
	})
}

func (c *Cluster) setupServices(ctx context.Context) error {
	if c.cluster.Spec.Services != nil {
		for _, service := range c.cluster.Spec.Services {
			err := k8sutil.CreateClientService(ctx, c.config.KubeCli, service.Name, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner(), c.isSecureClient(), service)
			if err != nil {
				return err
			}
			c.status.ServiceName = append(c.status.ServiceName, service.Name)
		}
	} else {
		err := k8sutil.CreateClientService(ctx, c.config.KubeCli, k8sutil.ClientServiceName(c.cluster.Name), c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner(), c.isSecureClient(), nil)
		if err != nil {
			return err
		}
		c.status.ServiceName = append(c.status.ServiceName, k8sutil.ClientServiceName(c.cluster.Name))
	}

	return k8sutil.CreatePeerService(ctx, c.config.KubeCli, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner(), c.isSecureClient())
}

func (c *Cluster) isPodPVEnabled() bool {
	if podPolicy := c.cluster.Spec.Pod; podPolicy != nil {
		return podPolicy.PersistentVolumeClaimSpec != nil
	}
	return false
}

func (c *Cluster) createPod(ctx context.Context, members etcdutil.MemberSet, m *etcdutil.Member, state string) error {
	token, err := k8sutil.CreateToken(c.cluster.Spec)
	if err != nil {
		return err
	}
	
	pod, err := k8sutil.NewEtcdPod(ctx, c.config.KubeCli, m, members.PeerURLPairs(), c.cluster.Name, c.cluster.Namespace, state, token, c.cluster.Spec, c.cluster.AsOwner())
	if c.isPodPVEnabled() {
		pvc := k8sutil.NewEtcdPodPVC(m, *c.cluster.Spec.Pod.PersistentVolumeClaimSpec, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner())
		_, err := c.config.KubeCli.CoreV1().PersistentVolumeClaims(c.cluster.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create PVC for member (%s): %v", m.Name, err)
		}
		k8sutil.AddEtcdVolumeToPod(pod, pvc, false)
	} else {
		tmpfs := false
		if podPolicy := c.cluster.Spec.Pod; podPolicy != nil {
			tmpfs = podPolicy.Tmpfs
		}
		k8sutil.AddEtcdVolumeToPod(pod, nil, tmpfs)
	}
	_, err = c.config.KubeCli.CoreV1().Pods(c.cluster.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	return err
}

func (c *Cluster) removePod(ctx context.Context, name string) error {
	ns := c.cluster.Namespace
	opts := *metav1.NewDeleteOptions(podTerminationGracePeriod)
	err := c.config.KubeCli.CoreV1().Pods(ns).Delete(ctx, name, opts)
	if err != nil {
		if !k8sutil.IsKubernetesResourceNotFoundError(err) {
			return err
		}
	}
	return nil
}

func (c *Cluster) removeService(ctx context.Context, name string) error {
	ns := c.cluster.Namespace
	opts := *metav1.NewDeleteOptions(podTerminationGracePeriod)
	err := c.config.KubeCli.CoreV1().Services(ns).Delete(ctx, name, opts)
	if err != nil {
		if !k8sutil.IsKubernetesResourceNotFoundError(err) {
			return err
		}
	}
	return nil
}

func (c *Cluster) pollPods(ctx context.Context) (running, pending []*v1.Pod, err error) {
	podList, err := c.config.KubeCli.CoreV1().Pods(c.cluster.Namespace).List(ctx, k8sutil.ClusterListOpt(c.cluster.Name))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list running pods: %v", err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		// Avoid polling deleted pods. k8s issue where deleted pods would sometimes show the status Pending
		// See https://github.com/coreos/etcd-operator/issues/1693
		if pod.DeletionTimestamp != nil {
			continue
		}
		if len(pod.OwnerReferences) < 1 {
			c.logger.Warningf("pollPods: ignore pod %v: no owner", pod.Name)
			continue
		}
		if pod.OwnerReferences[0].UID != c.cluster.UID {
			c.logger.Warningf("pollPods: ignore pod %v: owner (%v) is not %v",
				pod.Name, pod.OwnerReferences[0].UID, c.cluster.UID)
			continue
		}
		switch pod.Status.Phase {
		case v1.PodRunning:
			running = append(running, pod)
		case v1.PodPending:
			pending = append(pending, pod)
		}
	}

	return running, pending, nil
}

func (c *Cluster) pollServices(ctx context.Context) (map[string]*v1.Service, error) {
	services := make(map[string]*v1.Service)
	serviceList, err := c.config.KubeCli.CoreV1().Services(c.cluster.GetNamespace()).List(ctx, k8sutil.ClusterListOpt(c.cluster.GetName()))
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %v", err)
	}

	for _, service := range serviceList.Items {
		if service.Spec.ClusterIP == v1.ClusterIPNone {
			c.logger.Debugf("pollServices: ignore service %v: peer service", service.GetName())
			continue
		}
		if len(service.OwnerReferences) < 1 {
			c.logger.Debugf("pollServices: ignore service %v: no owner", service.GetName())
			continue
		}
		if service.OwnerReferences[0].UID != c.cluster.UID {
			c.logger.Debugf("pollServices: ignore service %v: owner (%v) is not %v",
				service.Name, service.OwnerReferences[0].UID, c.cluster.UID)
			continue
		}
		services[service.Name] = &service
	}

	return services, nil
}

func (c *Cluster) updateMemberStatus(running []*v1.Pod) {
	var unready []string
	var ready []string
	for _, pod := range running {
		if k8sutil.IsPodReady(pod) {
			ready = append(ready, pod.Name)
			continue
		}
		unready = append(unready, pod.Name)
	}

	c.status.Members.Ready = ready
	c.status.Members.Unready = unready
}

func (c *Cluster) updateCRStatus(ctx context.Context) error {
	if reflect.DeepEqual(c.cluster.Status, c.status) {
		return nil
	}

	newCluster := c.cluster
	newCluster.Status = c.status
	newCluster, err := c.config.EtcdCRCli.EtcdV1beta2().EtcdClusters(c.cluster.Namespace).Update(ctx, c.cluster, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update CR status: %v", err)
	}

	c.cluster = newCluster

	return nil
}

func (c *Cluster) reportFailedStatus(ctx context.Context) {
	c.logger.Info("cluster failed. Reporting failed reason...")

	retryInterval := 5 * time.Second
	f := func() (bool, error) {
		c.status.SetPhase(api.ClusterPhaseFailed)
		err := c.updateCRStatus(ctx)
		if err == nil || k8sutil.IsKubernetesResourceNotFoundError(err) {
			return true, nil
		}

		if !apierrors.IsConflict(err) {
			c.logger.Warningf("retry report status in %v: fail to update: %v", retryInterval, err)
			return false, nil
		}

		cl, err := c.config.EtcdCRCli.EtcdV1beta2().EtcdClusters(c.cluster.Namespace).
			Get(ctx, c.cluster.Name, metav1.GetOptions{})
		if err != nil {
			// Update (PUT) will return conflict even if object is deleted since we have UID set in object.
			// Because it will check UID first and return something like:
			// "Precondition failed: UID in precondition: 0xc42712c0f0, UID in object meta: ".
			if k8sutil.IsKubernetesResourceNotFoundError(err) {
				return true, nil
			}
			c.logger.Warningf("retry report status in %v: fail to get latest version: %v", retryInterval, err)
			return false, nil
		}
		c.cluster = cl
		return false, nil
	}

	retryutil.Retry(retryInterval, math.MaxInt64, f)
}

func (c *Cluster) name() string {
	return c.cluster.GetName()
}

func (c *Cluster) logClusterCreation() {
	specBytes, err := json.MarshalIndent(c.cluster.Spec, "", "    ")
	if err != nil {
		c.logger.Errorf("failed to marshal cluster spec: %v", err)
	}

	c.logger.Info("creating cluster with Spec:")
	for _, m := range strings.Split(string(specBytes), "\n") {
		c.logger.Info(m)
	}
}

func (c *Cluster) logSpecUpdate(oldSpec, newSpec api.ClusterSpec) {
	oldSpecBytes, err := json.MarshalIndent(oldSpec, "", "    ")
	if err != nil {
		c.logger.Errorf("failed to marshal cluster spec: %v", err)
	}
	newSpecBytes, err := json.MarshalIndent(newSpec, "", "    ")
	if err != nil {
		c.logger.Errorf("failed to marshal cluster spec: %v", err)
	}

	c.logger.Infof("spec update: Old Spec:")
	for _, m := range strings.Split(string(oldSpecBytes), "\n") {
		c.logger.Info(m)
	}

	c.logger.Infof("New Spec:")
	for _, m := range strings.Split(string(newSpecBytes), "\n") {
		c.logger.Info(m)
	}

}
