#!/bin/bash

# Get the version threshold from user input
read -p "Enter the minimum version to keep (e.g., 3.21.9): " INPUT_VERSION

# Extract major, minor, and patch versions from input
if [[ "$INPUT_VERSION" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    MIN_MAJOR=${BASH_REMATCH[1]}
    MIN_MINOR=${BASH_REMATCH[2]}
    MIN_PATCH=${BASH_REMATCH[3]}
else
    echo "Invalid version format. Use x.y.z (e.g., 3.21.9)"
    exit 1
fi

# Get list of all images
IMAGES=$(docker images --format "{{.Repository}} {{.Tag}} {{.ID}}" | awk '{print $1, $2, $3}')

# Loop through images
while read -r REPO TAG ID; do
    # Skip if it's the latest tag (to prevent unintended deletions)
    if [[ "$TAG" == "latest" ]]; then
        continue
    fi

    # Skip images referenced by digest
    if [[ "$REPO" == *"@sha256"* ]]; then
        echo "Skipping digest-referenced image: $REPO"
        continue
    fi

    # Delete all untagged images
    if [[ "$TAG" == "<none>" ]]; then
        echo "Deleting untagged image: $ID"
        docker rmi -f "$ID"
        continue
    fi

    # Delete images from "ewr.vultrcr.com/grodt/app" that are lower than the threshold version
    if [[ "$REPO" == "ewr.vultrcr.com/grodt/app" && "$TAG" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        MAJOR=${BASH_REMATCH[1]}
        MINOR=${BASH_REMATCH[2]}
        PATCH=${BASH_REMATCH[3]}

        # Compare versions
        if (( MAJOR < MIN_MAJOR )) || 
           (( MAJOR == MIN_MAJOR && MINOR < MIN_MINOR )) || 
           (( MAJOR == MIN_MAJOR && MINOR == MIN_MINOR && PATCH < MIN_PATCH )); then
            echo "Deleting old app image: $REPO:$TAG ($ID)"
            docker rmi -f "$ID"
        fi
    fi
done <<< "$IMAGES"

echo "Cleanup complete!"
