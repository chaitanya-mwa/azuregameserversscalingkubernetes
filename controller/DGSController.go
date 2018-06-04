package controller

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	shared "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared"
	dgsclientset "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/clientset/versioned"
	dgsscheme "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/clientset/versioned/scheme"
	dgsv1 "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/clientset/versioned/typed/azuregaming/v1alpha1"
	informerdgs "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/informers/externalversions/azuregaming/v1alpha1"
	listerdgs "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/listers/azuregaming/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	record "k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const dgsControllerAgentName = "dedigated-game-server-controller"

type DedicatedGameServerController struct {
	dgsClient       dgsv1.DedicatedGameServersGetter
	dgsLister       listerdgs.DedicatedGameServerLister
	dgsListerSynced cache.InformerSynced
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

func NewDedicatedGameServerController(client *kubernetes.Clientset, dgsclient *dgsclientset.Clientset,
	dgsInformer informerdgs.DedicatedGameServerInformer) *DedicatedGameServerController {
	// Create event broadcaster
	// Add DedicatedGameServerController types to the default Kubernetes Scheme so Events can be
	// logged for DedicatedGameServerController types.
	dgsscheme.AddToScheme(dgsscheme.Scheme)
	log.Info("Creating event broadcaster for DedicatedGameServer controller")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(dgsscheme.Scheme, apiv1.EventSource{Component: dgsControllerAgentName})

	c := &DedicatedGameServerController{
		dgsClient:       dgsclient.AzuregamingV1alpha1(),
		dgsLister:       dgsInformer.Lister(),
		dgsListerSynced: dgsInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DedicatedGameServerSync"),
		recorder:        recorder,
	}

	log.Info("Setting up event handlers for DedicatedGameServer controller")

	dgsInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				log.Print("DedicatedGameServer controller - add")
				//c.enqueueDedicatedGameServer(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				log.Print("DedicatedGameServer controller - update")
				//c.enqueueDedicatedGameServer(newObj)
			},
			DeleteFunc: func(obj interface{}) {
				log.Print("DedicatedGameServer controller - delete")
			},
		},
	)

	return c
}

// RunWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *DedicatedGameServerController) RunWorker() {
	// hot loop until we're told to stop.  processNextWorkItem will
	// automatically wait until there's work available, so we don't worry
	// about secondary waits
	log.Info("Starting loop for DedicatedGameServer controller")
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false
// when it's time to quit.
func (c *DedicatedGameServerController) processNextWorkItem() bool {
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
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// DedicatedGameServer resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		log.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the DedicatedGameServer resource
// with the current status of the resource.
func (c *DedicatedGameServerController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	dgs, err := c.dgsLister.DedicatedGameServers(namespace).Get(name)

	if err != nil {
		log.Print(err.Error())
		return err
	}

	c.recorder.Event(dgs, corev1.EventTypeNormal, shared.SuccessSynced, shared.MessageResourceSynced)
	return nil
}

func (c *DedicatedGameServerController) Workqueue() workqueue.RateLimitingInterface {
	return c.workqueue
}

func (c *DedicatedGameServerController) ListersSynced() []cache.InformerSynced {
	return []cache.InformerSynced{c.dgsListerSynced}
}