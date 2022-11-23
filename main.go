package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const mongo_db = "ng-db"

const added_collection = "added-pods"
const updated_collection = "updated-pods"
const deleted_collection = "deleted-pods"

// re-use the same connection without creating new one everytime
var mongoConnection = mongodb_connection()

func mongodb_connection() *mongo.Client {

	// var mongo_host = ""
	// value, present := os.LookupEnv("NAMESPACE_TO_WATCH")
	// if present && value != "" {
	// 	mongo_host = value
	// } else {
	// 	mongo_host = "app1"
	// }

	// Connection URI
	// var mongo_uri = "mongodb://" + mongo_host
	var mongo_uri = "mongodb://localhost:27017"

	// Set client options
	clientOptions := options.Client().ApplyURI(mongo_uri)

	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)

	if err != nil {
		log.Fatal(err)
	}

	klog.Infof("Connected to MongoDB\n")

	return client
}

// Function to add and delete the mongodb
func mongodb_action(namespace string, pod_name string, container_count int, containers_and_images [][]string, collection string, action string) {

	coll := mongoConnection.Database(mongo_db).Collection(collection)

	doc := bson.D{{"namespace", namespace}, {"pod_name", pod_name}, {"total_container_count", container_count}, {"containers_and_images", containers_and_images}, {"action", action}}

	_, err := coll.InsertOne(context.TODO(), doc)
	if err != nil {
		panic(err)
	}

}

// Function update deployment the mongodb
func mongodb_action_update(namespace string, pod_name string, param1 string, param2 string, action string) {

	coll := mongoConnection.Database(mongo_db).Collection(updated_collection)

	if action == "container_count_action" {
		doc := bson.D{{"namespace", namespace}, {"deployment_name", pod_name}, {"old_container_count", param1}, {"new_container_count", param2}, {"action", action}}

		_, err := coll.InsertOne(context.TODO(), doc)
		if err != nil {
			panic(err)
		}

	} else if action == "image_tag_action" {
		doc := bson.D{{"namespace", namespace}, {"deployment_name", pod_name}, {"old_image_tag", param1}, {"new_image_tag", param2}, {"action", action}}

		_, err := coll.InsertOne(context.TODO(), doc)
		if err != nil {
			panic(err)
		}

	}

}

func mongodb_delete(pod_name string, collection string) {

	coll := mongoConnection.Database(mongo_db).Collection(collection)
	filter := bson.D{{"pod_name", bson.D{{"$eq", pod_name}}}}

	_, err := coll.DeleteOne(context.TODO(), filter)
	if err != nil {
		panic(err)
	}

}
func watch_for_updated_events(namespace string, otherNamespace string) {

	// for updated-pods - image tag
	type UpdatedDocumentImageTag struct {
		ID          primitive.ObjectID `bson:"_id"`
		EventType   string             `bson:"operationType"`
		Namespace   string             `bson:"namespace"`
		Deployment  string             `bson:"deployment_name"`
		OldImageTag string             `bson:"old_image_tag"`
		NewImageTag string             `bson:"new_image_tag"`
		Action      string             `bson:"action"`
	}
	// for updated-pods - container count
	type UpdatedDocumentContainerCount struct {
		ID                primitive.ObjectID `bson:"_id"`
		EventType         string             `bson:"operationType"`
		Namespace         string             `bson:"namespace"`
		Deployment        string             `bson:"deployment_name"`
		OldContainerCount string             `bson:"old_container_count"`
		NewContainerCount string             `bson:"new_container_count"`
		Action            string             `bson:"action"`
	}

	var eventUpdateIT struct {
		UpdatedDocIT UpdatedDocumentImageTag `bson:"fullDocument"`
	}
	var eventUpdateCC struct {
		UpdatedDocCC UpdatedDocumentContainerCount `bson:"fullDocument"`
	}

	// open a change stream with an empty pipeline parameter - for updated-pods
	updated_coll := mongoConnection.Database(mongo_db).Collection(updated_collection)

	updatedChangeStream, err := updated_coll.Watch(context.TODO(), mongo.Pipeline{})
	if err != nil {
		panic(err)

	}
	defer updatedChangeStream.Close(context.TODO())

	for updatedChangeStream.Next(context.TODO()) {

		if err := updatedChangeStream.Decode(&eventUpdateIT); err != nil {
			fmt.Printf("Failed to decode event: %v", err)
			continue
		}
		if err := updatedChangeStream.Decode(&eventUpdateCC); err != nil {
			fmt.Printf("Failed to decode event: %v", err)
			continue
		}

		if eventUpdateIT.UpdatedDocIT.Namespace != namespace {

			if updatedChangeStream.Current.Lookup("operationType").String() == "\"insert\"" && eventUpdateIT.UpdatedDocIT.Action == "image_tag_action" {
				klog.Infof("⚪️🟠 NEWS FROM Namespace [%s]: Image tag of the DEPLOYMENT '%s' changed from '%s' to '%s'\n\n", eventUpdateIT.UpdatedDocIT.Namespace, eventUpdateIT.UpdatedDocIT.Deployment, eventUpdateIT.UpdatedDocIT.OldImageTag, eventUpdateIT.UpdatedDocIT.NewImageTag)
			}
		}

		if eventUpdateCC.UpdatedDocCC.Namespace != namespace {

			if updatedChangeStream.Current.Lookup("operationType").String() == "\"insert\"" && eventUpdateCC.UpdatedDocCC.Action == "container_count_action" {
				klog.Infof("⚪️🟠 NEWS FROM Namespace [%s]: A new Container is Added to the DEPLOYMENT '%s' and container count changed from '%s' to '%s'\n\n", eventUpdateCC.UpdatedDocCC.Namespace, eventUpdateCC.UpdatedDocCC.Deployment, eventUpdateCC.UpdatedDocCC.OldContainerCount, eventUpdateCC.UpdatedDocCC.NewContainerCount)
			}
		}
	}

}

func watch_for_deleted_events(namespace string, otherNamespace string) {
	// for deleted-pods
	type DeletedDocument struct {
		ID        primitive.ObjectID `bson:"_id"`
		EventType string             `bson:"operationType"`
		Namespace string             `bson:"namespace"`
		PodName   string             `bson:"pod_name"`
		Action    string             `bson:"action"`
	}
	var eventDelete struct {
		DeletedDoc DeletedDocument `bson:"fullDocument"`
	}

	// open a change stream with an empty pipeline parameter - for added-pods
	deleted_coll := mongoConnection.Database(mongo_db).Collection(deleted_collection)
	deletedChangeStream, err := deleted_coll.Watch(context.TODO(), mongo.Pipeline{})
	if err != nil {
		panic(err)
	}
	defer deletedChangeStream.Close(context.TODO())
	// iterate over the cursor to print the change stream events
	for deletedChangeStream.Next(context.TODO()) {

		if err := deletedChangeStream.Decode(&eventDelete); err != nil {
			fmt.Printf("Failed to decode event: %v", err)
			continue
		}

		if eventDelete.DeletedDoc.Namespace != namespace {

			if deletedChangeStream.Current.Lookup("operationType").String() == "\"insert\"" && eventDelete.DeletedDoc.Action == "deleted" {
				klog.Infof("⚪️🔴 NEWS FROM Namespace [%s]: POD DELETED: %s/%s\n\n", eventDelete.DeletedDoc.Namespace, eventDelete.DeletedDoc.Namespace, eventDelete.DeletedDoc.PodName)
			}
		}
	}

}

// Function to watcbh for mongodb change streams
func watch_for_events(namespace string, otherNamespace string) {

	// for added-pods
	type AddedDocument struct {
		ID                  primitive.ObjectID `bson:"_id"`
		EventType           string             `bson:"operationType"`
		Namespace           string             `bson:"namespace"`
		TotalContainerCount int                `bson:"total_container_count"`
		ContainerAndImages  [][]string         `bson:"containers_and_images"`
		PodName             string             `bson:"pod_name"`
		Action              string             `bson:"action"`
	}

	var eventAdded struct {
		AddedDoc AddedDocument `bson:"fullDocument"`
	}

	// open a change stream with an empty pipeline parameter - for added-pods
	added_coll := mongoConnection.Database(mongo_db).Collection(added_collection)

	addedChangeStream, err := added_coll.Watch(context.TODO(), mongo.Pipeline{})
	if err != nil {
		panic(err)
	}

	defer addedChangeStream.Close(context.TODO())

	for addedChangeStream.Next(context.TODO()) {
		// fmt.Println(addedChangeStream.Current)

		if err := addedChangeStream.Decode(&eventAdded); err != nil {
			fmt.Printf("Failed to decode event: %v", err)
			continue
		}

		if eventAdded.AddedDoc.Namespace != namespace {

			if addedChangeStream.Current.Lookup("operationType").String() == "\"insert\"" && eventAdded.AddedDoc.Action == "added" {
				klog.Infof("⚪️🟢 NEWS FROM Namespace [%s]: POD CREATED: %s/%s\n\n", eventAdded.AddedDoc.Namespace, eventAdded.AddedDoc.Namespace, eventAdded.AddedDoc.PodName)
			}
		}
	}

	fmt.Println("Watching Ended....")
}

func main() {

	kubeConfig := os.Getenv("KUBECONFIG")

	var namespace_to_watch string
	var other_namespace_to_accept string

	value1, present1 := os.LookupEnv("NAMESPACE_TO_WATCH")

	if present1 && value1 != "" {
		namespace_to_watch = value1
		fmt.Printf("\nMonitoring the Namespace '%s' started...", namespace_to_watch)
	} else {
		namespace_to_watch = "default"
		fmt.Printf("\nNo \"NAMESPACE_TO_WATCH\" set, hence watching the events from the '%s' namespace Started.... \n", namespace_to_watch)
	}

	value2, present2 := os.LookupEnv("other_namespace_to_watch")

	if present2 && value2 != "" {
		other_namespace_to_accept = value2
		fmt.Printf("\nListening for the events from the Namespace '%s' started...\n\n", other_namespace_to_accept)
	} else {
		other_namespace_to_accept = "default"
		fmt.Printf("\nNo \"other_namespace_to_watch\" set, hence ready to accept any incoming streams from the '%s' namespace Started.... \n", other_namespace_to_accept)
	}

	var clusterConfig *rest.Config
	var err error
	if kubeConfig != "" {
		klog.Infof("Out-Cluster Configs Detected...\n\n")
		clusterConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	} else {
		klog.Infof("In-Cluster Configs Detected...\n\n")
		clusterConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientSet, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		panic(err)
	}

	// stop signal for the informer
	stopper := make(chan struct{})
	defer close(stopper)

	// create shared informers for resources in all known API group versions with a reSync period and namespace
	factory := informers.NewSharedInformerFactoryWithOptions(clientSet, 1*time.Hour, informers.WithNamespace(namespace_to_watch))
	podInformer := factory.Core().V1().Pods().Informer()

	deploymentInfomer := factory.Apps().V1().Deployments().Informer()

	defer runtime.HandleCrash()

	// start informer ->
	go factory.Start(stopper)

	// start to sync and call list
	if !cache.WaitForCacheSync(stopper, podInformer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	// start to sync and call list
	if !cache.WaitForCacheSync(stopper, deploymentInfomer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: onAdd, // register add eventhandler
		// UpdateFunc: onUpdate,
		DeleteFunc: onDelete,
	})

	deploymentInfomer.AddEventHandler(cache.ResourceEventHandlerFuncs{

		UpdateFunc: onUpdateDeployment,
	})

	go watch_for_events(namespace_to_watch, other_namespace_to_accept)
	go watch_for_updated_events(namespace_to_watch, other_namespace_to_accept)
	go watch_for_deleted_events(namespace_to_watch, other_namespace_to_accept)

	// block the main go routine from exiting
	<-stopper
}

func onAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)

	var doNotMonitor bool
	for _, label := range pod.GetLabels() {
		if label == "notmonitor" {
			doNotMonitor = true
		}
	}
	if !doNotMonitor {
		fmt.Printf("\n\n")
		klog.Infof("🟢 POD CREATED: %s/%s\n\n", pod.Namespace, pod.Name)

		var containers []string
		var container_images []string

		var container_count = 0

		for _, container := range pod.Spec.Containers {
			container_count++
			containers = append(containers, container.Name)
			container_images = append(container_images, container.Image)
		}

		containers_and_images := [][]string{containers, container_images}
		mongodb_action(pod.Namespace, pod.Name, container_count, containers_and_images, added_collection, "added")
		doNotMonitor = false
		containers = nil
	}

}

func onDelete(obj interface{}) {
	pod := obj.(*corev1.Pod)

	var doNotMonitor bool
	for _, label := range pod.GetLabels() {
		if label == "notmonitor" {
			doNotMonitor = true
		}
	}

	if !doNotMonitor {
		fmt.Printf("\n\n")
		klog.Infof("🔴 POD DELETED: %s/%s\n\n", pod.Namespace, pod.Name)

		var containers []string
		var container_images []string

		var container_count = 0

		for _, container := range pod.Spec.Containers {
			container_count++
			containers = append(containers, container.Name)
			container_images = append(container_images, container.Image)
		}

		containers_and_images := [][]string{containers, container_images}

		// add to the deleted-pods collection
		mongodb_action(pod.Namespace, pod.Name, container_count, containers_and_images, deleted_collection, "deleted")

		// and remove from the added collection
		mongodb_delete(pod.Name, added_collection)
	}

}

func onUpdateDeployment(oldObj interface{}, newObj interface{}) {
	oldDeployment := oldObj.(*appsv1.Deployment)
	newDeployment := newObj.(*appsv1.Deployment)

	var doNotMonitor bool
	for _, label := range oldDeployment.GetLabels() {
		if label == "notmonitor" {
			doNotMonitor = true
		}
	}

	if !doNotMonitor {

		var old_pod_container_images []string
		var new_pod_container_images []string

		for _, old := range oldDeployment.Spec.Template.Spec.Containers {
			old_pod_container_images = append(old_pod_container_images, old.Image)
		}

		for _, new := range newDeployment.Spec.Template.Spec.Containers {
			new_pod_container_images = append(new_pod_container_images, new.Image)
		}

		var old_pod_container_count = 0
		var new_pod_container_count = 0

		for _, container := range oldDeployment.Spec.Template.Spec.Containers {
			old_pod_container_count++
			_ = container
		}

		for _, container := range newDeployment.Spec.Template.Spec.Containers {
			new_pod_container_count++
			_ = container
		}

		for i := 0; i < old_pod_container_count; i++ {
			if old_pod_container_count != new_pod_container_count {
				klog.Infof("🟠 A new Container is Added to the DEPLOYMENT '%s' and container count changed from '%d' to '%d'\n", oldDeployment.Name, old_pod_container_count, new_pod_container_count)
				mongodb_action_update(oldDeployment.Namespace, oldDeployment.Name, strconv.Itoa(old_pod_container_count), strconv.Itoa(new_pod_container_count), "container_count_action")
				break
			}
		}

		if !reflect.DeepEqual(old_pod_container_images, new_pod_container_images) {
			for i, v := range old_pod_container_images {
				if v != new_pod_container_images[i] && old_pod_container_count == new_pod_container_count {
					klog.Infof("🟠 Image tag of the DEPLOYMENT '%s' changed from '%s' to '%s'\n", oldDeployment.Name, v, new_pod_container_images[i])
					mongodb_action_update(oldDeployment.Namespace, oldDeployment.Name, v, new_pod_container_images[i], "image_tag_action")

				}

			}
		}

	}
}
