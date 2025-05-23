#!/bin/bash

# Stop the script on any command failure
set -e

# Check if the version bump type (major/minor/patch) is provided as an argument
if [ -z "$1" ]; then
  echo "Usage: ./deploy-app.sh <major/minor/patch>"
  exit 1
fi

BUMP_TYPE=$1
CONFIG_FILE=${PROJECTS_DIR}/slack-trading/.bumpversion.app.cfg

# Check if the config file exists
if [ ! -f "$CONFIG_FILE" ]; then
  echo "Error: Config file $CONFIG_FILE not found!"
  exit 1
fi

# Run bump2version with the provided config file and bump type (major/minor/patch)
bump2version $BUMP_TYPE --config-file $CONFIG_FILE

# Get the current version from the Dockerfile
VERSION=$(grep -i "^# version=" Dockerfile | cut -d'=' -f2 | tr -d '" ')

if [ -z "$VERSION" ]; then
  echo "Error: Unable to extract version from Dockerfile"
  exit 1
fi

echo "Deploying version $VERSION ..."

# Update the app version in the source code
sed -i.bak "s/return \".*\"/return \"${VERSION}\"/" /Users/jamal/projects/slack-trading/src/eventservices/app_version.go
rm ${PROJECTS_DIR}/slack-trading/src/eventservices/app_version.go.bak
git add ${PROJECTS_DIR}/slack-trading/src/eventservices/app_version.go
git commit -m "Bump app version to $VERSION in app_version.go"

# Build the Docker image with the version tag
docker build -t ewr.vultrcr.com/grodt/app:$VERSION -f Dockerfile .

# Push the Docker image to the registry
docker push ewr.vultrcr.com/grodt/app:$VERSION

# Update the latest tags
docker tag ewr.vultrcr.com/grodt/app:$VERSION ewr.vultrcr.com/grodt/app:latest
docker tag ewr.vultrcr.com/grodt/app:$VERSION ewr.vultrcr.com/grodt/app:latest-dev