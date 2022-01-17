package tools

import (
	"fmt"

	"github.com/peizhong/serverless-controller/pkg/apis/serverlesscontroller/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

func GetAppName(foo *v1alpha1.ServerlessFunc) string {
	return fmt.Sprintf("func-%s", foo.Name)
}

func GetDeploymentName(foo *v1alpha1.ServerlessFunc) string {
	return fmt.Sprintf("func-%s-deployment", foo.Name)
}

func GetServiceName(foo *v1alpha1.ServerlessFunc) string {
	return fmt.Sprintf("func-%s-service", foo.Name)
}

type DiffResult struct {
	Field string
	Left  interface{}
	Right interface{}
}

func DiffServerlessFuncAndDeployment(foo *v1alpha1.ServerlessFunc, deployment *appsv1.Deployment) []DiffResult {
	var result []DiffResult
	if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
		result = append(result, DiffResult{
			Field: "Replicas",
			Left:  *foo.Spec.Replicas,
			Right: *deployment.Spec.Replicas,
		})
	}
	if foo.Spec.Image != deployment.Labels["serverlessfunc-images"] {
		result = append(result, DiffResult{
			Field: "Image",
			Left:  foo.Spec.Image,
			Right: deployment.Labels["serverlessfunc-images"],
		})
	}
	if foo.Spec.Version != deployment.Labels["serverlessfunc-version"] {
		result = append(result, DiffResult{
			Field: "Version",
			Left:  foo.Spec.Version,
			Right: deployment.Labels["serverlessfunc-version"],
		})
	}
	return result
}
