package main

import (
	"context"
	"fmt"
	"os"

	configv1 "github.com/openshift/api/config/v1"
	opv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/spf13/cobra"
	"github.com/varshaprasad96/lib-go-operator/controller"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	// build a new cobra command
	command := NewOperator()
	// die on errors
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

}

func NewOperator() *cobra.Command {
	err := RunOperator(context.TODO(), &controllercmd.ControllerContext{})
	if err != nil {
		fmt.Errorf(err.Error())
	}

	return &cobra.Command{}
}

func RunOperator(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {

	// Use controller runtime only to get a config since it provides a warpper around the one given by library-go.
	cfg := ctrl.GetConfigOrDie()
	kubeclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	coreInformerFactory := coreinformers.NewSharedInformerFactory(kubeclient, 0 /*no resync */)

	testController := controller.NewTestController("memcached", kubeclient, coreInformerFactory.Apps().V1().Deployments(), events.NewInMemoryRecorder("memcached"), "default")

	for _, informer := range []interface {
		Start(stopCh <-chan struct{})
	}{
		coreInformerFactory,
	} {
		informer.Start(ctx.Done())
	}

	for _, controllerint := range []interface {
		Run(ctx context.Context, workers int)
	}{
		testController,
	} {
		go controllerint.Run(ctx, 1)
	}

	<-ctx.Done()
	return fmt.Errorf("stopped")

}

type operatorModifier func(instance *fakeOperatorInstance) *fakeOperatorInstance

func makeFakeOperatorInstance(modifiers ...operatorModifier) *fakeOperatorInstance {
	instance := &fakeOperatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cluster",
			Generation: 0,
		},
		Spec: opv1.OperatorSpec{
			ManagementState: opv1.Managed,
		},
		Status: opv1.OperatorStatus{},
	}
	for _, modifier := range modifiers {
		instance = modifier(instance)
	}
	return instance
}

type fakeOperatorInstance struct {
	metav1.ObjectMeta
	Spec   opv1.OperatorSpec
	Status opv1.OperatorStatus
}

// Infrastructure
func makeInfra() *configv1.Infrastructure {
	return &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: v1.NamespaceAll,
		},
		Status: configv1.InfrastructureStatus{
			InfrastructureName: "ID1234",
			Platform:           configv1.AWSPlatformType,
			PlatformStatus: &configv1.PlatformStatus{
				AWS: &configv1.AWSPlatformStatus{},
			},
		},
	}
}

func makeFakeManifest() []byte {
	return []byte(`
kind: Deployment
apiVersion: apps/v1
metadata:
  name: dummy-deployment
  namespace: openshift-dummy-test-deployment
spec:
  selector:
    matchLabels:
      app: test-csi-driver-controller
  serviceName: test-csi-driver-controller
  replicas: 1
  template:
    metadata:
      labels:
        app: test-csi-driver-controller
    spec:
      nodeSelector:
        node-role.kubernetes.io/master: ""
      containers:
        - name: csi-driver
          image: ${DRIVER_IMAGE}
          args:
            - --endpoint=$(CSI_ENDPOINT)
            - --k8s-tag-cluster-id=${CLUSTER_ID}
            - --logtostderr
            - --v=${LOG_LEVEL}
          env:
            - name: CSI_ENDPOINT
              value: unix:///var/lib/csi/sockets/pluginproxy/csi.sock
          ports:
            - name: healthz
              containerPort: 19808
              protocol: TCP
          volumeMounts:
            - name: socket-dir
              mountPath: /var/lib/csi/sockets/pluginproxy/
        - name: csi-provisioner
          image: ${PROVISIONER_IMAGE}
          args:
            - --provisioner=test.csi.openshift.io
            - --csi-address=$(ADDRESS)
            - --feature-gates=Topology=true
            - --http-endpoint=localhost:8202
            - --v=${LOG_LEVEL}
          env:
            - name: ADDRESS
              value: /var/lib/csi/sockets/pluginproxy/csi.sock
          volumeMounts:
            - name: socket-dir
              mountPath: /var/lib/csi/sockets/pluginproxy/
        # In reality, each sidecar needs its own kube-rbac-proxy. Using just one for the unit tests.
        - name: provisioner-kube-rbac-proxy
          args:
          - --secure-listen-address=0.0.0.0:9202
          - --upstream=http://127.0.0.1:8202/
          - --tls-cert-file=/etc/tls/private/tls.crt
          - --tls-private-key-file=/etc/tls/private/tls.key
          - --logtostderr=true
          image: ${KUBE_RBAC_PROXY_IMAGE}
          imagePullPolicy: IfNotPresent
          ports:
          - containerPort: 9202
            name: provisioner-m
            protocol: TCP
          resources:
            requests:
              memory: 20Mi
              cpu: 10m
          volumeMounts:
          - mountPath: /etc/tls/private
            name: metrics-serving-cert
        - name: csi-attacher
          image: ${ATTACHER_IMAGE}
          args:
            - --csi-address=$(ADDRESS)
            - --v=${LOG_LEVEL}
          env:
            - name: ADDRESS
              value: /var/lib/csi/sockets/pluginproxy/csi.sock
          volumeMounts:
            - name: socket-dir
              mountPath: /var/lib/csi/sockets/pluginproxy/
        - name: csi-resizer
          image: ${RESIZER_IMAGE}
          args:
            - --csi-address=$(ADDRESS)
            - --v=${LOG_LEVEL}
          env:
            - name: ADDRESS
              value: /var/lib/csi/sockets/pluginproxy/csi.sock
          volumeMounts:
            - name: socket-dir
              mountPath: /var/lib/csi/sockets/pluginproxy/
        - name: csi-snapshotter
          image: ${SNAPSHOTTER_IMAGE}
          args:
            - --csi-address=$(ADDRESS)
            - --v=${LOG_LEVEL}
          env:
          - name: ADDRESS
            value: /var/lib/csi/sockets/pluginproxy/csi.sock
          volumeMounts:
          - mountPath: /var/lib/csi/sockets/pluginproxy/
            name: socket-dir
      volumes:
        - name: socket-dir
          emptyDir: {}
        - name: metrics-serving-cert
          secret:
            secretName: gcp-pd-csi-driver-controller-metrics-serving-cert
`)
}
