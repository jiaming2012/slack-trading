#!/bin/bash

# Define the name of the Kubernetes secret
SECRET_NAME="app-secrets"
NAMESPACE="grodt"

# Create the header for the secrets YAML file
echo "apiVersion: v1
kind: Secret
metadata:
  name: $SECRET_NAME
  namespace: $NAMESPACE
type: Opaque
data:" > secret.yaml

# Read the .env.production-secrets file and append each key-value pair to the secrets YAML file
while IFS='=' read -r key value || [ -n "$key" ]; do
  # Encode the value in base64
  encoded_value=$(echo -n "$value" | base64)
  # Append the key-value pair to the secrets YAML file
  echo "  $key: $encoded_value" >> secret.yaml
done < ${PROJECTS_DIR}/slack-trading/src/.env.production-secrets

# Create the secret in the Kubernetes cluster
kubeseal --controller-name=sealed-secrets --controller-namespace=sealed-secrets --format yaml < secret.yaml > ${PROJECTS_DIR}/slack-trading/.clusters/production/sealedsecret.yaml