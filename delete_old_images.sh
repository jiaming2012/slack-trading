#!/bin/bash

# Define repository and cutoff version
REPO="ewr.vultrcr.com/grodt/app"
CUTOFF_VERSION="$1"

# Get list of images that match the repository
images=$(docker images --format "{{.Repository}} {{.Tag}} {{.ID}}" | grep "^$REPO " | awk '{print $2, $3}')

# Loop through images and delete those below the cutoff version
while read -r TAG IMAGE_ID; do
    if [[ "$TAG" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        if [[ "$(printf "%s\n%s" "$CUTOFF_VERSION" "$TAG" | sort -V | head -n1)" != "$CUTOFF_VERSION" ]]; then
            echo "Deleting image $REPO:$TAG ($IMAGE_ID)..."
            # docker rmi -f "$IMAGE_ID"
        fi
    fi
done <<< "$images"

