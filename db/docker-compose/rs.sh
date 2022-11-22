 
docker network create mongoCluster

docker run -d --rm -p 27020:27017 --name mongo3 --network mongoCluster mongo:5.0.3 mongod --replSet myReplicaSet --bind_ip localhost,mongo1
docker run -d --rm -p 27019:27017 --name mongo2 --network mongoCluster mongo:5.0.3 mongod --replSet myReplicaSet --bind_ip localhost,mongo2
docker run -d --rm -p 27018:27017 --name mongo1 --network mongoCluster mongo:5.0.3 mongod --replSet myReplicaSet --bind_ip localhost,mongo3

docker exec -it mongo1 mongosh --eval "rs.initiate({
 _id: \"myReplicaSet\",
 members: [
   {_id: 0, host: \"mongo1\"},
   {_id: 1, host: \"mongo2\"},
   {_id: 2, host: \"mongo3\"}
 ]
})"




docker network create mongo-cluster
docker run --name mongo1 -d --net mongo-cluster -p 27018:27017 mongo:5.0.3 mongod --replSet docker-rs --port 27017
docker run --name mongo2 -d --net mongo-cluster -p 27019:27017 mongo:5.0.3 mongod --replSet docker-rs --port 27017
docker run --name mongo3 -d --net mongo-cluster -p 27020:27017 mongo:5.0.3 mongod --replSet docker-rs --port 27017
docker exec -it mongo1 mongosh
config = {"_id" : "docker-rs", "members" : [{"_id" : 0,"host" : "mongo1:27017"},{"_id" : 1,"host" : "mongo2:27017"},{"_id" : 2,"host" : "mongo3:27017"}]}
rs.initiate(config)
rs.status()




# ----------
kubectl exec -it mongo-0 mongosh --eval "rs.initiate({_id: \"rs0\",members: [{_id: 0, host: \"mongo-0\"},{_id: 1, host: \"mongo-1\"},]})"

mongosh rs.initiate({_id: "rs0",members: [{_id: 0, host: "localhost"}]})
# connect using 
# mongodb://localhost:27017