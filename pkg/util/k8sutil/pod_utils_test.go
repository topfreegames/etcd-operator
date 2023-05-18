package k8sutil

import (
	"testing"
)

func TestNewEtcdProbe(t *testing.T) {
	isSecure := false
	isTLSSecret := false
	expectedCommand := []string{"etcdctl", "endpoint", "status"}

	probe := newEtcdProbe(isSecure, isTLSSecret)

	if len(probe.Handler.Exec.Command) != len(expectedCommand){
		t.Errorf("expected command=%s, got=%s", expectedCommand, probe.Handler.Exec.Command)
	}
	for i, v := range expectedCommand {
		if v != probe.Handler.Exec.Command[i] {
			t.Errorf("expected piece=%s, got=%s", v, probe.Handler.Exec.Command[i])
		}
	}
}

func TestNewEtcdProbeWithTLS(t *testing.T) {
	isSecure := true
	isTLSSecret := false
	expectedCommand := []string{"etcdctl", "--endpoints=https://localhost:2379", "--cert=/etc/etcdtls/operator/etcd-tls/etcd-client.crt", "--key=/etc/etcdtls/operator/etcd-tls/etcd-client.key", "--cacert=/etc/etcdtls/operator/etcd-tls/etcd-client-ca.crt", "endpoint", "status"}

	probe := newEtcdProbe(isSecure, isTLSSecret)

	if len(probe.Handler.Exec.Command) != len(expectedCommand){
		t.Errorf("expected command=%v, got=%v", len(expectedCommand), len(probe.Handler.Exec.Command))
	}
	for i, v := range expectedCommand {
		if v != probe.Handler.Exec.Command[i] {
			t.Errorf("expected piece=%s, got=%s", v, probe.Handler.Exec.Command[i])
		}
	}
}

func TestNewEtcdProbeWithTLSWithTLSSecret(t *testing.T) {
	isSecure := true
	isTLSSecret := true
	expectedCommand := []string{"etcdctl", "--endpoints=https://localhost:2379", "--cert=/etc/etcdtls/operator/etcd-tls/tls.crt", "--key=/etc/etcdtls/operator/etcd-tls/tls.key", "--cacert=/etc/etcdtls/operator/etcd-tls/ca.crt", "endpoint", "status"}

	probe := newEtcdProbe(isSecure, isTLSSecret)

	if len(probe.Handler.Exec.Command) != len(expectedCommand){
		t.Errorf("expected command=%v, got=%v", len(expectedCommand), len(probe.Handler.Exec.Command))
	}
	for i, v := range expectedCommand {
		if v != probe.Handler.Exec.Command[i] {
			t.Errorf("expected piece=%s, got=%s", v, probe.Handler.Exec.Command[i])
		}
	}
}