package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	serverlessv1alpha1 "github.com/peizhong/serverless-controller/pkg/apis/serverlesscontroller/v1alpha1"
	clientset "github.com/peizhong/serverless-controller/pkg/generated/clientset/versioned"
	samplescheme "github.com/peizhong/serverless-controller/pkg/generated/clientset/versioned/scheme"
	informers "github.com/peizhong/serverless-controller/pkg/generated/informers/externalversions/serverlesscontroller/v1alpha1"
	listers "github.com/peizhong/serverless-controller/pkg/generated/listers/serverlesscontroller/v1alpha1"
	"github.com/peizhong/serverless-controller/pkg/tools"
)

const controllerAgentName = "serverless-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "Foo synced successfully"
)

// Controller is the controller implementation for CRD resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// crdclientset is a clientset for our own API group
	crdClientSet clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced

	crdLister listers.ServerlessFuncLister
	crdSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

func NewController(
	kubeclientset kubernetes.Interface,
	crdclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	crdInformer informers.ServerlessFuncInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		crdClientSet:      crdclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		crdLister:         crdInformer.Lister(),
		crdSynced:         crdInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Foos"),
		recorder:          recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Foo resources change
	crdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueCrd,
		UpdateFunc: func(old, new interface{}) {
			klog.Info("update crd")
			controller.enqueueCrd(new)
		},
		DeleteFunc: func(obj interface{}) {
			klog.Info("delete crd")
		},
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Foo resource will enqueue that Foo resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Info("i don't care deployments added")
			return
			controller.handleObject(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			klog.Info("i don't care deployments updated")
			return
			controller.handleObject(new)
		},
		DeleteFunc: func(obj interface{}) {
			klog.Info("i don't care deployments deleted")
			return
			controller.handleObject(obj)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Foo controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.crdSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Foo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			err = fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
			klog.Info(err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Foo resource with this namespace/name
	foo, err := c.crdLister.ServerlessFuncs(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	deploymentName := tools.GetDeploymentName(foo)
	// Get the deployment with the name specified in Foo.spec
	deployment, err := c.deploymentsLister.Deployments(foo.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Create(context.TODO(), newDeployment(foo), metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this Foo resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(deployment, foo) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(foo, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If this number of the replicas on the Foo resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	klog.Infof("DiffServerlessFuncAndDeployment")
	diff := tools.DiffServerlessFuncAndDeployment(foo, deployment)
	if len(diff) > 0 {
		for _, item := range diff {
			klog.Infof("Foo: [%s].[%s] expect: %v, deployment: %v", foo.Name, item.Field, item.Left, item.Right)
		}
		deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Update(context.TODO(), newDeployment(foo), metav1.UpdateOptions{})
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	{
		// service
		service, err := c.kubeclientset.CoreV1().Services(foo.Namespace).Get(context.TODO(), tools.GetServiceName(foo), metav1.GetOptions{})
		if errors.IsNotFound(err) {
			klog.Info("create service ", service.Name)
			service, err = c.kubeclientset.CoreV1().Services(foo.Namespace).Create(context.TODO(), newService(foo), metav1.CreateOptions{})
		}
		if err != nil {
			return err
		}
	}

	{
		// ingress
		ingress, err := c.kubeclientset.NetworkingV1().Ingresses(foo.Namespace).Get(context.TODO(), tools.GetIngressName(), metav1.GetOptions{})
		if err != nil {
			klog.Infof("Get Ingresses err: %v", err.Error())
		}
		if errors.IsNotFound(err) {
			// 创建已有ingress
			ingress, err = c.kubeclientset.NetworkingV1().Ingresses(foo.Namespace).Create(context.TODO(), newIngress(foo.Namespace), metav1.CreateOptions{})
		}
		if err != nil {
			klog.Infof("Create Ingresses err: %v", err.Error())
			return err
		}
		// 比较ingress是否不一致
		klog.Infof("DiffServerlessFuncAndIngress")
		diff = tools.DiffServerlessFuncAndIngress(foo, ingress)
		if len(diff) > 0 {
			for _, item := range diff {
				klog.Infof("Foo: [%s].[%s] expect: %v, ingress: %v", foo.Name, item.Field, item.Left, item.Right)
			}
			// 本次foo，更新到ingress
			_, err = c.kubeclientset.NetworkingV1().Ingresses(foo.Namespace).Update(context.TODO(), updateIngress(ingress, foo), metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	err = c.updateCrdStatus(foo, deployment)
	if err != nil {
		err = fmt.Errorf("updateCrdStatus err: %v", err.Error())
		return err
	}

	c.recorder.Event(foo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	klog.Infof("syncHandler: %s complete", key)
	return nil
}

func (c *Controller) updateCrdStatus(foo *serverlessv1alpha1.ServerlessFunc, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	fooCopy := foo.DeepCopy()
	fooCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Foo resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.crdClientSet.ServerlesscontrollerV1alpha1().ServerlessFuncs(foo.Namespace).Update(context.TODO(), fooCopy, metav1.UpdateOptions{})
	if err != nil {
		err = fmt.Errorf("update foo(%s/%s) err :%v", foo.Namespace, fooCopy.Name, err.Error())
	}
	return err
}

// enqueueCrd takes a Foo resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Foo.
func (c *Controller) enqueueCrd(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Foo, we should not do anything more
		// with it.
		if ownerRef.Kind != "ServerlessFunc" {
			return
		}

		foo, err := c.crdLister.ServerlessFuncs(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueCrd(foo)
		return
	}
}

var (
	DefaultRevisionHistoryLimit int32 = 2
	DefaultRunAsUser            int64 = 1000
	DefaultRunAsGroup           int64 = 3000
)

// newDeployment creates a new Deployment for a Foo resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Foo resource that 'owns' it.
func newDeployment(foo *serverlessv1alpha1.ServerlessFunc) *appsv1.Deployment {
	labels := map[string]string{
		"serverlessfunc": tools.GetAppName(foo),
	}
	resourceLimit := func(cpu, memory int) corev1.ResourceList {
		resp := corev1.ResourceList{}
		if cpu > 0 {
			if limit, err := resource.ParseQuantity(fmt.Sprintf("%dm", cpu)); err == nil {
				resp[corev1.ResourceCPU] = limit
			}
		}
		if memory > 0 {
			if limit, err := resource.ParseQuantity(fmt.Sprintf("%dMi", memory)); err == nil {
				resp[corev1.ResourceMemory] = limit
			}
		}
		return resp
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tools.GetDeploymentName(foo),
			Namespace: foo.Namespace,
			// 根对象
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(foo, serverlessv1alpha1.SchemeGroupVersion.WithKind("ServerlessFunc")),
			},
			Labels: map[string]string{
				"serverlessfunc":         tools.GetAppName(foo),
				"serverlessfunc-images":  foo.Spec.Image,
				"serverlessfunc-version": foo.Spec.Version,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             foo.Spec.Replicas,
			RevisionHistoryLimit: &DefaultRevisionHistoryLimit,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  &DefaultRunAsUser,
						RunAsGroup: &DefaultRunAsGroup,
					},
					Volumes: []corev1.Volume{
						{
							Name: "ide-workspaces",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "ide-workspaces-pvc",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "pilot",
							Image: "localhost:32000/serverless-pilot:v0.0.1",
							Env: []corev1.EnvVar{
								{
									Name:  "SERVERLESS_FUNC",
									Value: foo.Name,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ping",
										Port: intstr.Parse("8080"),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       30,
							},
							Resources: corev1.ResourceRequirements{
								Limits: resourceLimit(10, 20),
							},
						}, {
							Name:  "rpcserver",
							Image: "localhost:32000/alpine:v0.0.1",
							Env: []corev1.EnvVar{
								{
									Name:  "SERVERLESS_FUNC",
									Value: foo.Name,
								},
							},
							Command: []string{
								fmt.Sprintf("/app/%s", foo.Spec.Image),
								"-v",
								foo.Spec.Version,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "rpc",
									ContainerPort: 30000,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ide-workspaces",
									MountPath: "/app",
									SubPath:   "serverless-functions",
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: resourceLimit(20, 40),
							},
						},
					},
				},
			},
		},
	}
	return deployment
}

func newService(foo *serverlessv1alpha1.ServerlessFunc) *corev1.Service {
	labels := map[string]string{
		"serverlessfunc": tools.GetAppName(foo),
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tools.GetServiceName(foo),
			Namespace: foo.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(foo, serverlessv1alpha1.SchemeGroupVersion.WithKind("ServerlessFunc")),
			},
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "pilot",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.Parse("8080"),
				},
			},
		},
	}
}

// newIngress should be one ingress
func newIngress(namespace string) *networkingv1.Ingress {
	labels := map[string]string{}
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            tools.GetIngressName(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{
				// *metav1.NewControllerRef(foo, serverlessv1alpha1.SchemeGroupVersion.WithKind("ServerlessFunc")),
			},
			Labels: labels,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				// 只用1个
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							// todo: 必须有值 spec.rules[0].http.paths: Required value
							Paths: make([]networkingv1.HTTPIngressPath, 0),
						},
					},
				},
			},
		},
	}
}

func updateIngress(current *networkingv1.Ingress, foo *serverlessv1alpha1.ServerlessFunc) *networkingv1.Ingress {
	result := current.DeepCopy()
	if len(result.Spec.Rules) != 1 {
		// 修复数据
		return result
	}
	currentRule := current.Spec.Rules[0]
	if currentRule.IngressRuleValue.HTTP == nil {
		return result
	}
	newRule := currentRule.DeepCopy()
	if len(newRule.HTTP.Paths) > 0 {
		newRule.HTTP.Paths = newRule.HTTP.Paths[:0]
	}
	var updateExitRule bool
	ingressPath := tools.GetIngressPath(foo)
	for _, currentPath := range currentRule.IngressRuleValue.HTTP.Paths {
		if currentPath.Path == ingressPath {
			updateExitRule = true
			updatePath := currentPath.DeepCopy()
			updatePath.Backend.Service = &networkingv1.IngressServiceBackend{
				Name: tools.GetServiceName(foo),
				Port: networkingv1.ServiceBackendPort{
					Number: 80,
				},
			}
			newRule.HTTP.Paths = append(newRule.HTTP.Paths, *updatePath)
		} else {
			newRule.HTTP.Paths = append(newRule.HTTP.Paths, currentPath)
		}
	}
	pathTypePrefix := networkingv1.PathTypePrefix
	if !updateExitRule {
		newRule.HTTP.Paths = append(newRule.HTTP.Paths, networkingv1.HTTPIngressPath{
			Path:     ingressPath,
			PathType: &pathTypePrefix,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: tools.GetServiceName(foo),
					Port: networkingv1.ServiceBackendPort{
						Number: 80,
					},
				},
			},
		})
	}
	result.Spec.Rules = result.Spec.Rules[:0]
	result.Spec.Rules = append(result.Spec.Rules, *newRule)
	return result
}
