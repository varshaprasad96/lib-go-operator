package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
)

type TestController struct {
	name           string
	kubeclient     kubernetes.Interface
	recorder       events.Recorder
	deployInformer appsinformersv1.DeploymentInformer
	namespace      string
}

func NewTestController(name string,
	kubeclient kubernetes.Interface,
	deployInformer appsinformersv1.DeploymentInformer, recorder events.Recorder, ns string) factory.Controller {
	c := &TestController{
		name:           name,
		kubeclient:     kubeclient,
		deployInformer: deployInformer,
		recorder:       recorder,
		namespace:      ns,
	}

	return factory.New().WithInformers(deployInformer.Informer()).WithSync(c.sync).ResyncEvery(time.Minute).ToController(c.name, recorder.WithComponentSuffix(strings.ToLower(name)+"-deployment-controller-"))
}

func (c *TestController) sync(ctx context.Context, syncContext factory.SyncContext) error {
	fmt.Println("*******reconciling************")

	var size int32
	size = 3

	found, err := c.kubeclient.AppsV1().Deployments(c.namespace).Get(ctx, "memcached", v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Println("object not found creating one")
			toCreate := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      c.name,
					Namespace: c.namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &size,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "memcached",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "memcached",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Image: "nginx:1.14.2",
								Name:  "nginx",
								Ports: []corev1.ContainerPort{{
									ContainerPort: 80,
								}},
							}},
						},
					},
				},
			}

			_, err := c.kubeclient.AppsV1().Deployments(c.namespace).Create(ctx, toCreate, v1.CreateOptions{})
			if err != nil {
				return err
			}
			return nil
		} else if err != nil {
			return err
		}
	}

	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		updated, err := c.kubeclient.AppsV1().Deployments(c.namespace).Update(ctx, found, v1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}
