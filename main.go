package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/markkurossi/tabulate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var exit = make(chan bool)

func hello(from string) {
	for i := 0; i < 3; i++ {
		fmt.Println(from, ":", i)
	}

}

func sort_and_remove_duplicates(container_list []string) []string {

	// sort the list
	sort.Strings(container_list)

	// remove duplicates
	for i := len(container_list) - 1; i > 0; i-- {
		if container_list[i] == container_list[i-1] {
			container_list = append(container_list[:i], container_list[i+1:]...)
		}
	}

	return container_list
}

func showTableSummary(podNames []string, containeNames []string, containerImages []string) {

	tab := tabulate.New(tabulate.Unicode)
	tab.Header("Pod Name").SetAlign(tabulate.MR)
	tab.Header("Containers / [Images]")

	for _, podName := range podNames {

		row := tab.Row()
		row.Column(podName)

		container_details := ""
		for index, containerName := range containeNames {
			container_details = container_details + containerName + " [" + containerImages[index] + "]\n"

		}
		row.Column(container_details)

	}

	tab.Print(os.Stdout)

}

func pod_changes(clientset *kubernetes.Clientset, namespace_to_watch string) {

	api := clientset.CoreV1()

	listOptions := metav1.ListOptions{
		LabelSelector: "",
		FieldSelector: "",
	}

	watcher, err := api.Pods(namespace_to_watch).Watch(context.TODO(), listOptions)

	if err != nil {

		log.Fatal(err)
	}

	// here we iterate all the events streamed by the watch.Interface
	for event := range watcher.ResultChan() {

		var added_pods []string
		var deleted_pods []string
		var modified_pods []string

		var added_pod_containers []string
		var deleted_pod_containers []string
		var modified_pod_containers []string

		var added_container_images []string
		var deleted_container_images []string
		var modified_container_images []string

		// retrieve the pod
		item, ok := event.Object.(*corev1.Pod)

		if !ok {
			log.Fatal("Unexpected Object Type")
		}

		switch event.Type {

		// when a pod is added...
		case watch.Added:

			// list all the containers in the pod
			for container := range item.Spec.Containers {
				added_pod_containers = append(added_pod_containers, item.Spec.Containers[container].Name)
				added_container_images = append(added_container_images, item.Spec.Containers[container].Image)
				added_pods = append(added_pods, item.GetName())
			}

			added_pod_containers = sort_and_remove_duplicates(added_pod_containers)
			added_pods = sort_and_remove_duplicates(added_pods)
			added_container_images = sort_and_remove_duplicates(added_container_images)

			log.Printf("- NEW POD '%s' %v -> %s  ✅", item.GetName(), event.Type, added_pod_containers)

			// timer1.Stop()

		// when a pod is modified...
		case watch.Modified:

			for container := range item.Spec.Containers {
				modified_pod_containers = append(modified_pod_containers, item.Spec.Containers[container].Name)
				modified_container_images = append(modified_container_images, item.Spec.Containers[container].Image)
				modified_pods = append(modified_pods, item.GetName())
			}
			modified_pod_containers = sort_and_remove_duplicates(modified_pod_containers)
			modified_pods = sort_and_remove_duplicates(modified_pods)
			modified_container_images = sort_and_remove_duplicates(modified_container_images)
			log.Printf("- EXISTING POD '%s' %v -> %s  ⚙️", item.GetName(), event.Type, modified_pod_containers)

		// when a pod is deleted...
		case watch.Deleted:

			for container := range item.Spec.Containers {
				deleted_pod_containers = append(deleted_pod_containers, item.Spec.Containers[container].Name)
				deleted_container_images = append(deleted_container_images, item.Spec.Containers[container].Image)
				deleted_pods = append(deleted_pods, item.GetName())
			}
			deleted_pod_containers = sort_and_remove_duplicates(deleted_pod_containers)
			deleted_pods = sort_and_remove_duplicates(deleted_pods)
			deleted_container_images = sort_and_remove_duplicates(deleted_container_images)

			log.Printf("- EXISTING POD '%s' %v -> %s  ❌", item.GetName(), event.Type, deleted_pod_containers)

			// diff := actual_pod_state(added_pod_containers, deleted_pod_containers)

			// fmt.Printf("Diff: %s\n", diff)

		}

		// show the information in tabular format
		// showTableSummary(added_pods, added_pod_containers, added_container_images)

	}

	exit <- true // Notify main() that this goroutine has finished

}

func actual_pod_state(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func main() {
	kubeConfig := os.Getenv("KUBECONFIG")

	var clusterConfig *rest.Config
	var err error
	if kubeConfig != "" {
		clusterConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	} else {
		log.Println("In-Cluster Configs Detected...")
		clusterConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		panic(err)
	}

	var namespace_to_watch = ""

	if os.Getenv("NAMESPACES") != "" {
		namespace_to_watch = os.Getenv("NAMESPACES")

	} else {
		namespace_to_watch = "ng"
	}

	log.Println("- Monitoring the namespace \"" + namespace_to_watch + "\" started...\n")

	go pod_changes(clientset, namespace_to_watch)
	<-exit // This blocks until the exit channel receives some input

}
