#!/bin/bash

# Define the name of the Kubernetes ConfigMap
CONFIGMAP_NAME="grodt-configmap"
NAMESPACE="default"

# Create the header for the ConfigMap YAML file
echo "apiVersion: v1
kind: ConfigMap
metadata:
  name: $CONFIGMAP_NAME
  namespace: $NAMESPACE
data:" > configmap.yaml

# Read the .env.production file and append each key-value pair to the ConfigMap YAML file
while IFS='=' read -r key value; do
  # Append the key-value pair to the ConfigMap YAML file
  echo "  $key: $value" >> configmap.yaml
done < ${PROJECTS_DIR}/slack-trading/src/.env.production