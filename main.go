package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

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

const added_collection = "app1-added-pods"
const updated_collection = "app1-updated-pods"
const deleted_collection = "app1-deleted-pods"
const shared_events = "shared_events"

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

// Function to update the mongodb
func mongodb_action(namespace string, pod_name string, container_count int, containers_and_images [][]string, collection string, action string) {

	coll := mongoConnection.Database(mongo_db).Collection(collection)

	doc := bson.D{{"namespace", namespace}, {"pod_name", pod_name}, {"total_container_count", container_count}, {"containers_and_images", containers_and_images}, {"action", action}}

	_, err := coll.InsertOne(context.TODO(), doc)
	if err != nil {
		panic(err)
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

// Function to watcbh for mongodb change streams
func watch_for_events(namespace string, otherNamespace string) {

	type YourDocument struct {
		ID                  primitive.ObjectID `bson:"_id"`
		EventType           string             `bson:"operationType"`
		Namespace           string             `bson:"namespace"`
		TotalContainerCount int                `bson:"total_container_count"`
		ContainerAndImages  [][]string         `bson:"containers_and_images"`
		PodName             string             `bson:"pod_name"`
		Action              string             `bson:"action"`
	}

	var event struct {
		Doc YourDocument `bson:"fullDocument"`
	}

	// open a change stream with an empty pipeline parameter
	coll := mongoConnection.Database(mongo_db).Collection(shared_events)
	changeStream, err := coll.Watch(context.TODO(), mongo.Pipeline{})
	if err != nil {
		panic(err)
	}
	defer changeStream.Close(context.TODO())
	// iterate over the cursor to print the change stream events

	for changeStream.Next(context.TODO()) {
		// fmt.Println(changeStream.Current)

		if err := changeStream.Decode(&event); err != nil {
			fmt.Printf("Failed to decode event: %v", err)
			continue
		}

		if event.Doc.Namespace != namespace {

			if changeStream.Current.Lookup("operationType").String() == "\"insert\"" && event.Doc.Action == "added" {
				klog.Infof("⚪️🟢 NEWS FROM Namespace [%s]: POD CREATED: %s/%s\n\n", event.Doc.Namespace, event.Doc.Namespace, event.Doc.PodName)
			} else if event.Doc.Action == "updated" {
				klog.Infof("⚪️🟠 NEWS FROM Namespace [%s]: POD UPDATED: %s/%s\n\n", event.Doc.Namespace, event.Doc.Namespace, event.Doc.PodName)
			} else if event.Doc.Action == "deleted" {
				klog.Infof("⚪️🔴 NEWS FROM Namespace [%s]: POD DELETED: %s/%s\n\n", event.Doc.Namespace, event.Doc.Namespace, event.Doc.PodName)
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

	defer runtime.HandleCrash()

	// start informer ->
	go factory.Start(stopper)

	// start to sync and call list
	if !cache.WaitForCacheSync(stopper, podInformer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onAdd, // register add eventhandler
		UpdateFunc: onUpdate,
		DeleteFunc: onDelete,
	})

	watch_for_events(namespace_to_watch, other_namespace_to_accept)

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
		mongodb_action(pod.Namespace, pod.Name, container_count, containers_and_images, shared_events, "added")
		doNotMonitor = false
		containers = nil
	}

}

func onUpdate(oldObj interface{}, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	var doNotMonitor bool
	for _, label := range newPod.GetLabels() {
		if label == "notmonitor" {
			doNotMonitor = true
		}
	}

	if !doNotMonitor {
		klog.Infof(
			"🟠 POD UPDATED. %s/%s %s",
			oldPod.Namespace, oldPod.Name, oldPod.Status.Phase,
		)

		var old_pod_container_count = 0
		var old_pod_containers []string
		var old_pod_container_images []string
		for _, container := range oldPod.Spec.Containers {
			old_pod_container_count++
			old_pod_containers = append(old_pod_containers, container.Name)
			old_pod_container_images = append(old_pod_container_images, container.Image)
		}

		var new_pod_container_count = 0
		var new_pod_containers []string
		var new_pod_container_images []string
		for _, container := range newPod.Spec.Containers {
			new_pod_container_count++
			new_pod_containers = append(new_pod_containers, container.Name)
			new_pod_containers = append(new_pod_container_images, container.Image)
		}

		old_containers_and_images := [][]string{old_pod_containers, old_pod_containers}
		mongodb_action(oldPod.Namespace, oldPod.Name, old_pod_container_count, old_containers_and_images, shared_events, "updated")

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

		mongodb_action(pod.Namespace, pod.Name, container_count, containers_and_images, shared_events, "deleted")
	}

}
