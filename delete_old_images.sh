#!/bin/bash

# Define repository and cutoff version
REPO="ewr.vultrcr.com/grodt/app"
CUTOFF_VERSION="$1"

# Get list of images that match the repository and extract tag + image ID
images=$(docker images --format "{{.Repository}} {{.Tag}} {{.ID}}" | grep "^$REPO " | awk '{print $2, $3}')

# Find images to delete
images_to_delete=()
while read -r TAG IMAGE_ID; do
    if [[ "$TAG" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        if [[ "$(printf "%s\n%s" "$CUTOFF_VERSION" "$TAG" | sort -V | head -n1)" != "$CUTOFF_VERSION" ]]; then
            images_to_delete+=("$IMAGE_ID")
            echo "Will delete: $REPO:$TAG ($IMAGE_ID)"
        fi
    fi
done <<< "$images"

# Ask for confirmation before deleting
if [[ ${#images_to_delete[@]} -gt 0 ]]; then
    echo ""
    read -p "Do you want to delete all images before $CUTOFF_VERSION? (y/N) " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        for IMAGE_ID in "${images_to_delete[@]}"; do
            echo "Deleting $IMAGE_ID..."
            docker rmi -f "$IMAGE_ID"
        done
        echo "Deletion complete."
    else
        echo "Operation canceled."
    fi
else
    echo "No images found before $CUTOFF_VERSION."
fi