package tools

import (
	"fmt"

	"github.com/peizhong/serverless-controller/pkg/apis/serverlesscontroller/v1alpha1"
)

func GetIngressName() string {
	return "serverlessfunc-ingress"
}

func GetIngressPath(foo *v1alpha1.ServerlessFunc) string {
	return fmt.Sprintf("/serverlessfunc/%s(/|$)(.*)", foo.Name)
}

func GetAppName(foo *v1alpha1.ServerlessFunc) string {
	return fmt.Sprintf("func-%s", foo.Name)
}

func GetDeploymentName(foo *v1alpha1.ServerlessFunc) string {
	return fmt.Sprintf("func-%s-deployment", foo.Name)
}

func GetServiceName(foo *v1alpha1.ServerlessFunc) string {
	return fmt.Sprintf("func-%s-service", foo.Name)
}
