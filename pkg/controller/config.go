package controller

import (
	"path/filepath"
	"time"

	"github.com/peizhong/serverless-controller/pkg/generated/clientset/versioned"
	informers "github.com/peizhong/serverless-controller/pkg/generated/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func RestConfigFromLocal() *rest.Config {
	homeDir := homedir.HomeDir()
	if len(homeDir) == 0 {
		homeDir = "~"
	}
	kubeConfigPath := filepath.Join(homeDir, ".kube", "config")
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		panic(err)
	}
	return restConfig
}

// FromLocalFile kubectl proxy --address=0.0.0.0 --port=8700 --disable-filter=true
func FromLocalFile(stopCh <-chan struct{}) *Controller {
	restConfig := RestConfigFromLocal()
	kubeclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	crdClientSet, err := versioned.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeclient, time.Minute)
	crdInformerFactory := informers.NewSharedInformerFactory(crdClientSet, time.Minute)
	ctrl := NewController(kubeclient, crdClientSet,
		kubeInformerFactory.Apps().V1().Deployments(),
		crdInformerFactory.Serverlesscontroller().V1alpha1().ServerlessFuncs())

	kubeInformerFactory.Start(stopCh)
	crdInformerFactory.Start(stopCh)
	return ctrl
}
