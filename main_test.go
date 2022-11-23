package main

import (
	"context"
	"log"
	"os"
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoClient(t *testing.T) {
	var mongo_host = ""
	value, present := os.LookupEnv("NAMESPACE_TO_WATCH")
	if present && value != "" {
		mongo_host = value
	} else {
		mongo_host = "app1"
	}

	// Connection URI
	var mongo_uri = "mongodb://" + mongo_host

	// when running tests, you need to have a local mongo cluster running, please specify it below
	// var mongo_uri = "mongodb://localhost:27017"

	// Set client options
	clientOptions := options.Client().ApplyURI(mongo_uri)

	expected_client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	result := mongodb_connection()

	if reflect.TypeOf(result).String() != reflect.TypeOf(expected_client).String() {
		t.Errorf("\"mongodb_connection()\" FAILED, expected -> %v, got -> %v", reflect.TypeOf(expected_client).String(), reflect.TypeOf(result).String())
	} else {
		t.Logf("\"mongodb_connection()\" SUCCEDED, expected -> %v, got -> %v", reflect.TypeOf(expected_client).String(), reflect.TypeOf(result).String())
	}
}

func TestMongoInsertAction(t *testing.T) {
	expected_output := "Insert Action Successful"
	containers_and_images := [][]string{{"nginx"}, {"nginx"}}
	result := mongodb_action("test_namespace", "test_pod_name", 1, containers_and_images, "test_collection", "test_action")
	if result != expected_output {
		t.Errorf("\"mongodb_action_update()\" FAILED, expected -> %v, got -> %v", expected_output, result)
	} else {
		t.Logf("\"mongodb_action_update()\" SUCCEDED, expected -> %v, got -> %v", expected_output, result)
	}
}

func TestMNongoUpdateAction(t *testing.T) {
	expected_output := "Update Action Successful"
	result := mongodb_action_update("test_namespace", "test_pod_name", "param1", "param2", "container_count_action")
	if result != expected_output {
		t.Errorf("\"mongodb_action_update()\" FAILED, expected -> %v, got -> %v", expected_output, result)
	} else {
		t.Logf("\"mongodb_action_update()\" SUCCEDED, expected -> %v, got -> %v", expected_output, result)
	}

}
