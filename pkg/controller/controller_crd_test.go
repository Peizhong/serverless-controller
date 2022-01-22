package controller

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/peizhong/serverless-controller/pkg/apis/serverlesscontroller/v1alpha1"
	"github.com/peizhong/serverless-controller/pkg/generated/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTypes(t *testing.T) {
	var serverlessFunc v1alpha1.ServerlessFunc
	serverlessFunc.Name = "nop"
	serverlessFunc.Spec.Image = ""
	var defaultReplicas int32 = 1
	serverlessFunc.Spec.Replicas = &defaultReplicas
}

func TestRun(t *testing.T) {
	stopCh := make(chan struct{})
	ctrl := FromLocalFile(stopCh)
	go func() {
		<-time.After(time.Second * 10)
		close(stopCh)
	}()
	if err := ctrl.Run(1, stopCh); err != nil {
		log.Fatalf("Error running controller: %s", err.Error())
	}
}

func TestListCrd(t *testing.T) {
	restConfig := RestConfigFromLocal()
	crdClientSet, err := versioned.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	var serverlessFunc v1alpha1.ServerlessFunc
	serverlessFunc.Name = "nop"
	serverlessFunc.Spec.Image = ""
	var defaultReplicas int32 = 1
	serverlessFunc.Spec.Replicas = &defaultReplicas
	resp, err := crdClientSet.ServerlesscontrollerV1alpha1().ServerlessFuncs("default").List(context.Background(), v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	t.Log(len(resp.Items))
}

func TestAddCrd(t *testing.T) {
	restConfig := RestConfigFromLocal()
	crdClientSet, err := versioned.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	var serverlessFunc v1alpha1.ServerlessFunc
	serverlessFunc.Name = "hello"
	serverlessFunc.Spec.Image = "nop"
	var defaultReplicas int32 = 1
	serverlessFunc.Spec.Replicas = &defaultReplicas
	resp, err := crdClientSet.ServerlesscontrollerV1alpha1().ServerlessFuncs("default").Create(context.Background(), &serverlessFunc, v1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	t.Log(resp.Name)
}

func TestGetCrd(t *testing.T) {
	restConfig := RestConfigFromLocal()
	crdClientSet, err := versioned.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	resp, err := crdClientSet.ServerlesscontrollerV1alpha1().ServerlessFuncs("default").Get(context.Background(), "nop", v1.GetOptions{})
	if err != nil {
		panic(err)
	}
	t.Log(resp.Name, resp.Spec.Image, *resp.Spec.Replicas)
}
