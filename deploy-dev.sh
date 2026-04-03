#!/bin/bash

# Stop the script on any command failure
set -e

# Check if the version bump type (major/minor/patch) is provided as an argument
if [ -z "$1" ]; then
  echo "Usage: ./deploy-app.sh <major/minor/patch>"
  exit 1
fi

BUMP_TYPE=$1
CONFIG_FILE=${TRADING_PROJECT_DIR}/.bumpversion.app.cfg

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
rm ${TRADING_PROJECT_DIR}/src/eventservices/app_version.go.bak
git add ${TRADING_PROJECT_DIR}/src/eventservices/app_version.go
git commit -m "Bump app version to $VERSION in app_version.go"

# Build the Docker image with the version tag
docker build -t grodt/app:$VERSION -f Dockerfile .

# Update the latest tags
docker tag grodt/app:$VERSION grodt/app:latest
docker tag grodt/app:$VERSION grodt/app:latest-dev