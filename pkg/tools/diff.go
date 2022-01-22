package tools

import (
	"github.com/peizhong/serverless-controller/pkg/apis/serverlesscontroller/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

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

func DiffServerlessFuncAndIngress(foo *v1alpha1.ServerlessFunc, ingress *networkingv1.Ingress) []DiffResult {
	var result []DiffResult
	if ruleLength := len(ingress.Spec.Rules); ruleLength != 1 {
		result = append(result, DiffResult{
			Field: "Spec.Rules",
			Left:  1,
			Right: ruleLength,
		})
		return result
	}
	rule := ingress.Spec.Rules[0]
	serviceName := GetServiceName(foo)
	for _, path := range rule.HTTP.Paths {
		if path.Backend.Service.Name == serviceName {
			return nil
		}
	}
	result = append(result, DiffResult{
		Field: "Spec.Rules[0].Http.Paths.Backend.ServiceName",
		Left:  serviceName,
		Right: "",
	})
	return result
}
