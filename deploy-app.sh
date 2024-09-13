#!/bin/bash

# Check if the path is provided as an argument
if [ -z "$1" ]; then
  echo "Usage: ./bump_version.sh <major/minor/patch>"
  exit 1
fi

CONFIG_FILE=${PROJECTS_DIR}/slack-trading/.bumpversion.grodt.cfg

# Check if the config file exists
if [ ! -f "$CONFIG_FILE" ]; then
  echo "Error: Config file $CONFIG_FILE not found!"
  exit 1
fi

# Run bump2version with the provided config file and patch version bump
bump2version patch --config-file $CONFIG_FILE

# Get the current version from the Dockerfile
VERSION=$(grep "version=" Dockerfile | cut -d'=' -f2)

# Build the images with the version tag
docker build -t grodt:$VERSION -f Dockerfile .

# Push the images to the Docker registry
docker push grodt:$VERSION

# Push to Github
git push