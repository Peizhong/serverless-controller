package main

import (
	"log"
	"os"

	"github.com/peizhong/serverless-controller/pkg/controller"
	"github.com/peizhong/serverless-controller/pkg/signals"
	"k8s.io/klog"
)

func main() {
	klog.InitFlags(nil)
	klog.SetOutput(os.Stdout)

	stopCh := signals.SetupSignalHandler()
	ctrl := controller.FromLocalFile(stopCh)
	if err := ctrl.Run(1, stopCh); err != nil {
		log.Fatalf("Error running controller: %s", err.Error())
	}
}
