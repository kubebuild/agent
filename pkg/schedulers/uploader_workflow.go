package schedulers

import (
	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/kubebuild/agent/pkg/graphql"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (b *BuildScheduler) createPipeUpTemplate(build graphql.ScheduledBuild) *wfv1.Workflow {
	container := &apiv1.Container{
		Image: "kubebuild/pipelineme:latest",
		Env:   getEnvVars(),
	}
	template := wfv1.Template{
		Name:      "pipeup",
		Container: container,
	}
	templates := []wfv1.Template{template}
	spec := wfv1.WorkflowSpec{
		Entrypoint: "pipeup",
		Templates:  templates,
	}

	if build.Pipeline.GitSecretName != nil {
		keyPath := apiv1.KeyToPath{
			Key:  "sshPrivateKey",
			Path: "id_rsa",
		}
		keyMode := int32(384)
		volumeSource := apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName:  "{{workflow.parameters.gitSecretName}}",
				DefaultMode: &keyMode,
				Items:       []apiv1.KeyToPath{keyPath},
			},
		}
		volume := apiv1.Volume{
			Name:         "key-volume",
			VolumeSource: volumeSource,
		}
		volumes := []apiv1.Volume{volume}
		spec.Volumes = volumes
		volumeMount := apiv1.VolumeMount{
			Name:      "key-volume",
			MountPath: "/root/.ssh",
			ReadOnly:  true,
		}
		volumeMounts := []apiv1.VolumeMount{volumeMount}
		container.VolumeMounts = volumeMounts
	}

	wf := &wfv1.Workflow{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Workflow",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pipeup-",
		},
		Spec: spec,
	}
	AddBuildLabels(build, true, wf)

	return wf
}

func getEnvVars() []apiv1.EnvVar {
	buildID := apiv1.EnvVar{
		Name:  "BUILD_ID",
		Value: "{{workflow.parameters.buildID}}",
	}
	repo := apiv1.EnvVar{
		Name:  "REPO",
		Value: "{{workflow.parameters.repo}}",
	}
	revision := apiv1.EnvVar{
		Name:  "REVISION",
		Value: "{{workflow.parameters.revision}}",
	}
	clusterToken := apiv1.EnvVar{
		Name:  "CLUSTER_TOKEN",
		Value: "{{workflow.parameters.clusterToken}}",
	}
	envVars := []apiv1.EnvVar{buildID, repo, revision, clusterToken}

	return envVars

}
