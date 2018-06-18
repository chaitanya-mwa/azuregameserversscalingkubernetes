package controller

import (
	"fmt"

	"github.com/dgkanatsios/azuregameserversscalingkubernetes/shared"

	dgsv1alpha1 "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/clientset/versioned/typed/azuregaming/v1alpha1"

	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	cache "k8s.io/client-go/tools/cache"

	record "k8s.io/client-go/tools/record"
	workqueue "k8s.io/client-go/util/workqueue"

	dgsclientset "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/clientset/versioned"
	kubernetes "k8s.io/client-go/kubernetes"

	informerdgs "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/informers/externalversions/azuregaming/v1alpha1"
	listerdgs "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/listers/azuregaming/v1alpha1"

	log "github.com/Sirupsen/logrus"
	dgsscheme "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/client/clientset/versioned/scheme"

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	apidgsv1alpha1 "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared/pkg/apis/azuregaming/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	dgsColControllerAgentName = "dedigated-game-server-collection-controller"
	labelDGSCol               = "dgsCol"
)

type DedicatedGameServerCollectionController struct {
	dgsColClient       dgsv1alpha1.DedicatedGameServerCollectionsGetter
	dgsColLister       listerdgs.DedicatedGameServerCollectionLister
	dgsColListerSynced cache.InformerSynced

	dgsClient       dgsv1alpha1.DedicatedGameServersGetter
	dgsLister       listerdgs.DedicatedGameServerLister
	dgsListerSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder
}

func NewDedicatedGameServerCollectionController(client *kubernetes.Clientset, dgsclient *dgsclientset.Clientset,
	dgsColInformer informerdgs.DedicatedGameServerCollectionInformer, dgsInformer informerdgs.DedicatedGameServerInformer) *DedicatedGameServerCollectionController {
	dgsscheme.AddToScheme(dgsscheme.Scheme)
	log.Info("Creating Event broadcaster for DedicatedGameServerCollection controller")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(dgsscheme.Scheme, corev1.EventSource{Component: dgsColControllerAgentName})

	c := &DedicatedGameServerCollectionController{
		dgsColClient:       dgsclient.AzuregamingV1alpha1(),
		dgsColLister:       dgsColInformer.Lister(),
		dgsColListerSynced: dgsColInformer.Informer().HasSynced,
		dgsClient:          dgsclient.AzuregamingV1alpha1(),
		dgsLister:          dgsInformer.Lister(),
		dgsListerSynced:    dgsInformer.Informer().HasSynced,
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DedicatedGameServerCollectionSync"),
		recorder:           recorder,
	}

	log.Info("Setting up event handlers for DedicatedGameServerCollection controller")

	dgsColInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				log.Print("DedicatedGameServerCollection controller - add")
				c.handleDedicatedGameServerCollection(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				log.Print("DedicatedGameServerCollection controller - update")
				oldDGSCol := oldObj.(*apidgsv1alpha1.DedicatedGameServerCollection)
				newDGSCol := newObj.(*apidgsv1alpha1.DedicatedGameServerCollection)

				if oldDGSCol.ResourceVersion == newDGSCol.ResourceVersion {
					return
				}

				// Moreover, if the number of replicas is the same, we should skip
				if oldDGSCol.Spec.Replicas == newDGSCol.Spec.Replicas {
					return
				}

				c.handleDedicatedGameServerCollection(newObj)
			},
			DeleteFunc: func(obj interface{}) {
				// IndexerInformer uses a delta nodeQueue, therefore for deletes we have to use this
				// key function.
				//key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
				log.Print("DedicatedGameServerCollection controller - delete")
			},
		},
	)

	return c
}

// RunWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *DedicatedGameServerCollectionController) RunWorker() {
	// hot loop until we're told to stop.  processNextWorkItem will
	// automatically wait until there's work available, so we don't worry
	// about secondary waits
	log.Info("Starting loop for DedicatedGameServerCollection controller")
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false
// when it's time to quit.
func (c *DedicatedGameServerCollectionController) processNextWorkItem() bool {
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
func (c *DedicatedGameServerCollectionController) syncHandler(key string) error {

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the DedicatedGameServerCollection resource with this namespace/name
	dgsCol, err := c.dgsColLister.DedicatedGameServerCollections(namespace).Get(name)
	if err != nil {
		// The DedicatedGameServerCollection resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("dgsCol '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// Find out how many DedicatedGameServerReplicas exist for this DedicatedGameServerCollection

	set := labels.Set{
		labelDGSCol: dgsCol.Name,
	}

	selector := labels.SelectorFromSet(set)
	dgsExisting, err := c.dgsLister.DedicatedGameServers(dgsCol.Namespace).List(selector)

	if err != nil {
		return err
	}

	dgsExistingCount := len(dgsExisting)

	// if there are less DGS than the ones we requested
	if dgsExistingCount < int(dgsCol.Spec.Replicas) {
		for i := 0; i < int(dgsCol.Spec.Replicas)-dgsExistingCount; i++ {

			port, errPort := shared.GetRandomPort()
			if errPort != nil {
				return errPort
			}

			dgs := shared.NewDedicatedGameServer(dgsCol, dgsCol.Name+"-"+shared.RandString(5), port, "sessionUrlexample", dgsCol.Spec.StartMap, dgsCol.Spec.Image)
			_, err := c.dgsClient.DedicatedGameServers(namespace).Create(dgs)

			if err != nil {
				log.Error(err.Error())
				return err
			}
		}
	} else if dgsExistingCount > int(dgsCol.Spec.Replicas) { //if there are more DGS than the ones we requested
		// we need to decrease our DGS for this collection
		// to accomplish this, we'll first find the number of DGS we need to decrease
		decreaseCount := dgsExistingCount - int(dgsCol.Spec.Replicas)
		indexesToDecrease := shared.GetRandomIndexes(dgsExistingCount, decreaseCount)

		for i := 0; i < len(indexesToDecrease); i++ {
			dgsToMarkForDeletion, err := c.dgsClient.DedicatedGameServers(namespace).Get(dgsExisting[indexesToDecrease[i]].Name, metav1.GetOptions{})
			if err != nil {
				log.Error(err.Error())
				return err
			}
			dgsToMarkForDeletionCopy := dgsToMarkForDeletion.DeepCopy()
			dgsToMarkForDeletionCopy.ObjectMeta.OwnerReferences = nil
			delete(dgsToMarkForDeletionCopy.ObjectMeta.Labels, labelDGSCol)
			_, err = c.dgsClient.DedicatedGameServers(namespace).Update(dgsToMarkForDeletionCopy)
			if err != nil {
				log.Error(err.Error())
				return err
			}
		}
	}

	c.recorder.Event(dgsCol, corev1.EventTypeNormal, shared.SuccessSynced, fmt.Sprintf(shared.MessageResourceSynced, "DedicatedGameServerCollection", dgsCol.Name))
	return nil
}

func (c *DedicatedGameServerCollectionController) handleDedicatedGameServerCollection(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding DedicatedGameServerCollection object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding DedicatedGameServerCollection object tombstone, invalid type"))
			return
		}
		log.Infof("Recovered deleted DedicatedGameServerCollection object '%s' from tombstone", object.GetName())
	}

	c.enqueueDedicatedGameServerCollection(obj)
}

// enqueueDedicatedGameServerCollection takes a DedicatedGameServerCollection resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than DedicatedGameServerCollection.
func (c *DedicatedGameServerCollectionController) enqueueDedicatedGameServerCollection(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c *DedicatedGameServerCollectionController) Workqueue() workqueue.RateLimitingInterface {
	return c.workqueue
}

func (c *DedicatedGameServerCollectionController) ListersSynced() []cache.InformerSynced {
	return []cache.InformerSynced{c.dgsColListerSynced, c.dgsListerSynced}
}
