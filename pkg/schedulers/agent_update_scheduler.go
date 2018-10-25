package schedulers

import (
	"fmt"

	"github.com/kubebuild/agent/pkg/graphql"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// AgentUpdateScheduler schedule your agent updates
type AgentUpdateScheduler struct {
	log           *logrus.Logger
	graphqlClient *graphql.Client
	kubeClient    kubernetes.Interface
}

// NewAgentUpdateScheduler schedule builds
func NewAgentUpdateScheduler(kubeClient kubernetes.Interface, graphqlClient *graphql.Client, log *logrus.Logger) *AgentUpdateScheduler {
	return &AgentUpdateScheduler{
		log:           log,
		graphqlClient: graphqlClient,
		kubeClient:    kubeClient,
	}
}

// Start starts a timer loop querying builds.
func (a *AgentUpdateScheduler) Start() {
	utils.SetInterval(func() {
		a.updateDeployment("kubebuild", true)
		a.updateDeployment("kubebuild-agent", false)
	}, 60*5*1000, false)
}

func (a *AgentUpdateScheduler) updateDeployment(deploymentSuffix string, updateArgs bool) error {
	namespace := string(a.graphqlClient.Cluster.Name)
	deploymentName := fmt.Sprintf("%s-%s", namespace, deploymentSuffix)
	currentDeployment, err := a.kubeClient.ExtensionsV1beta1().Deployments(namespace).Get(deploymentName, v1.GetOptions{})
	if err != nil {
		a.log.WithError(err).Error("failed to get deployment")
		return err
	}
	clusterConfig, err := a.graphqlClient.GetClusterConfig()
	if err != nil {
		a.log.WithError(err).Error("failed to get cluster config")
		return err
	}
	updateNeeded := false
	currentImage := currentDeployment.Spec.Template.Spec.Containers[0].Image
	var newImage string
	if updateArgs {
		currentArgs := currentDeployment.Spec.Template.Spec.Containers[0].Args
		executorImage := string(clusterConfig.ExecutorImage)

		if currentArgs[3] != executorImage {
			currentArgs[3] = executorImage
			updateNeeded = true
		}
		newImage = string(clusterConfig.WorkflowImage)

		if currentImage != newImage {
			currentDeployment.Spec.Template.Spec.Containers[0].Image = newImage
			updateNeeded = true
		}
	} else {
		newImage = string(clusterConfig.LauncherImage)
		if currentImage != newImage {
			currentDeployment.Spec.Template.Spec.Containers[0].Image = newImage
			updateNeeded = true
		}
	}

	if updateNeeded {
		a.log.WithFields(logrus.Fields{
			"currentImage": currentImage,
			"newImage":     newImage,
		}).Info("updating ...")
		_, err := a.kubeClient.ExtensionsV1beta1().Deployments(namespace).Update(currentDeployment)
		if err != nil {
			a.log.WithError(err).Error("failed to update deployment")
			return err
		}
	}
	return nil
}
