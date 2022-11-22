# client-go-watcher
This repo contains the code snippet that watch for various Kubernetes resource events in a namespace 



## 1. Deploy the `mongodb `

```
helm install ng-mongo helm/mongo --namespace ng-mongo --create-namespace
```


Then check the pod status / logs
```
kubectl get pods -n ng-mongo

kubectl logs -f mongo-0 -c mongo -n ng-mongo
```

Then initialize the replica set
```
kubectl exec -it mongo-0 -n ng-mongo mongosh
rs.initiate({_id: "rs0",members: [{_id: 0, host: "mongo-0.mongo.ng-mongo.svc.cluster.local:27017"}]})
```

## 2. Deploy the `app1 `
```
helm install app1 helm/app --namespace app1 --create-namespace
```

Then check the logs app1 logs 
```
kubectl logs -f <POD-NAME> -n app1
```


## 3. Deploy the `app2`
```
helm install app2 helm/app --namespace app2 --create-namespace
```

Then check the logs app1 logs 
```
kubectl logs -f <POD-NAME> -n app2
```
