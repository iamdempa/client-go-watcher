#!/bin/bash


# 1. Deploy the mongodb 
helm install ng-mongo helm/app --namespace ng-mongo --create-namespace

    # a.) Then initialize the replicaset 
    kubectl exec -it mongo-0 mongosh
    rs.initiate({_id: "rs0",members: [{_id: 0, host: "localhost"}]})

    # b.) Check the logs mongodb logs 
    kubectl logs -f mongo-0 -c mongo -n ng-mongo

# 2. Deploy the app1 
helm install app1 helm/app --namespace app1 --create-namespace

    # a.) Check the logs app1 logs 
    kubectl logs -f <POD-NAME> -n app1


# 3. Deploy the app2
helm install app2 helm/app --namespace app2 --create-namespace

    # a.) Check the logs app1 logs 
    kubectl logs -f <POD-NAME> -n app2
