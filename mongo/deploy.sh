#!/bin/bash
kubectl apply -f sc.yaml
helm repo add ng https://charts.bitnami.com/bitnami
helm install v1 ng/mongodb --values values.yaml