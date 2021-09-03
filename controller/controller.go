package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/klog/v2"
)

type DeploymentController_Test struct {
	// client
	operatorClient v1helpers.OperatorClient
	// core kube
	deploymentClient appsclientv1.DeploymentsGetter
}

func NewDeploymentController_Test(
	// core kube
	deploymentClient appsclientv1.DeploymentsGetter,
	deploymentInformer appsinformersv1.DeploymentInformer,
	// events
	recorder events.Recorder,

) factory.Controller {
	ctrl := &DeploymentController_Test{
		deploymentClient: deploymentClient,
	}

	var ControllerResyncInterval = 5 * time.Second
	return factory.New().WithSync(ctrl.sync).ResyncEvery(ControllerResyncInterval).ToController("DeploymentController", recorder)
}

func (c *DeploymentController_Test) sync(ctx context.Context, controllerContext factory.SyncContext) error {

	klog.V(4).Infof("Sync csr")

	fmt.Println("running controller here")
	fmt.Println("********************************")
	controllerContext.Recorder().Eventf("CSRCreated", "here")

	return nil

}
