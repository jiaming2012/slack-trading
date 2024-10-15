#!/bin/bash

# Stop the script on any command failure
set -e

# Check if the path is provided as an argument
if [ -z "$1" ]; then
  echo "Usage: ./deploy-base-image-2.sh <major/minor/patch>"
  exit 1
fi

CONFIG_FILE=${PROJECTS_DIR}/slack-trading/.bumpversion.app.cfg

# Check if the config file exists
if [ ! -f "$CONFIG_FILE" ]; then
  echo "Error: Config file $CONFIG_FILE not found!"
  exit 1
fi

# Run bump2version with the provided config file and patch version bump
bump2version patch --config-file $CONFIG_FILE

# Get the current version from the Dockerfile
VERSION=$(grep -i "version=" Dockerfile | cut -d'=' -f2 | tr -d '" ')

# Build the images with the version tag
docker build -t ewr.vultrcr.com/grodt/grodt-base-image-2:$VERSION -f Dockerfile.base2 .

# Push the images to the Docker registry
docker push ewr.vultrcr.com/grodt/grodt-base-image-2:$VERSION

# Push to Github
git push