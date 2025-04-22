#!/bin/bash

# Stop the script on any command failure
set -e

# Check if the current branch is main
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "Error: You must be on the main branch to deploy"
  exit 1
fi

# Check if the working directory is clean
if [ -n "$(git status --porcelain)" ]; then
  echo "Error: Your working directory is dirty. Please commit or stash your changes before deploying."
  exit 1
fi

# Merge the dev branch into main
echo "Merging dev branch into main..."
git fetch origin
git merge origin/dev

# Find the latest version of the Docker image
VERSION=$(docker images ewr.vultrcr.com/grodt/app --format "{{.Tag}}" | grep -v "latest" | grep -v "<none>" | sort -V | tail -n 1)

if [ -z "$VERSION" ]; then
  echo "Error: Unable to find the latest version of the Docker image"
  exit 1
fi

echo "Latest version found: $VERSION"

# Prompt the user for confirmation
read -p "Would you like to deploy ewr.vultrcr.com/grodt/app:$VERSION? (y/n): " CONFIRM
if [ "$CONFIRM" != "y" ]; then
  echo "Deployment cancelled."
  exit 0
fi

echo "Deploying version $VERSION ..."

# Update deployment.yaml with the new image version
sed -i.bak "s|image: ewr.vultrcr.com/grodt/app:[^ ]*|image: ewr.vultrcr.com/grodt/app:$VERSION|" ${PROJECTS_DIR}/slack-trading/.clusters/production/deployment.yaml

# Remove backup file created by sed
rm ${PROJECTS_DIR}/slack-trading/.clusters/production/deployment.yaml.bak

# Commit the updated deployment.yaml file and the version bump
git add ${PROJECTS_DIR}/slack-trading/.clusters/production/deployment.yaml
git commit -m "Bump app version to $VERSION in deployment.yaml"

# Push the changes to GitHub
git push

# Update kubernetes cluster
kubectl apply -f .clusters/production/configmap.yaml

kubectl apply -f .clusters/production/deployment.yaml

while true; do
    STATUS=$(kubectl get pods -l app=grodt -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}')
    if [ "$STATUS" == "True" ]; then
        echo "Pod is ready!"
        break
    fi
    sleep 5
done

echo "Changes applied. Waiting for the pod to be ready..."

TWIRP_HOST="http://45.77.223.21"
URL="$TWIRP_HOST/twirp/playground.PlaygroundService/GetAppVersion"

EXPECTED_VERSION=$VERSION
PAYLOAD="{}"
HEADERS=(-H "Content-Type: application/json")

while true; do
  RESPONSE=$(curl -s -X POST "${HEADERS[@]}" -d "$PAYLOAD" "$URL")

  # Use jq to safely parse JSON; fallback to grep if jq isn't installed
  if command -v jq &> /dev/null; then
    VERSION=$(echo "$RESPONSE" | jq -r '.version // empty')
  else
    VERSION=$(echo "$RESPONSE" | grep -oP '"version"\s*:\s*"\K[^"]+')
  fi

  if [[ "$VERSION" == "$EXPECTED_VERSION" ]]; then
    echo "Received expected version: $VERSION"
    break
  else
    echo "Waiting for expected version... got: ${VERSION:-"no version"}"
    sleep 1
  fi
done

echo "Deployment successful! Version $VERSION has been deployed."
