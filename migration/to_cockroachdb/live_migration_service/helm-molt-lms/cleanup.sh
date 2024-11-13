#!/bin/bash -x
# Copyright 2023 Cockroach Labs Inc.

kubectl delete pvc -l app.kubernetes.io/name=cockroachdb
kubectl delete pvc -l app.kubernetes.io/name=mysql
kubectl delete pvc -l app.kubernetes.io/name=alertmanager
kubectl delete pvc -l app.kubernetes.io/name=loki
kubectl delete pod -l app.kubernetes.io/name=cockroachdb --force
kubectl delete pod -l app.kubernetes.io/instance=lms-loki --force
