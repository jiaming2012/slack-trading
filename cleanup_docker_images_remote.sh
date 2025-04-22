#!/bin/bash

# Set your Vultr container registry URL
REGISTRY="ewr.vultrcr.com/grodt/app"

# Get the list of tags from the registry
TAGS=$(crane ls $REGISTRY)

# Find the max tag version number
if [ -z "$TAGS" ]; then
    echo "No tags found in the registry."
    exit 1
fi

# Sort the tags and get the latest version
LATEST_TAG=$(echo "$TAGS" | sort -V | tail -n 1)
if [ -z "$LATEST_TAG" ]; then
    echo "No valid tags found."
    exit 1
fi

# Define the version threshold
# Prompt user for the version threshold
echo "Current latest version: $LATEST_TAG"
read -p "Enter the version threshold (e.g., 1.2.3) to keep images newer than this version: " THRESHOLD

# Function to compare semantic versions
version_lt() {
    printf '%s\n%s\n' "$1" "$2" | sort -V | head -n 1 | grep -q "$1"
}

# Loop through tags and delete old versions
for TAG in $TAGS; do
    if version_lt "$TAG" "$THRESHOLD"; then
        echo "Fetching digest for $REGISTRY:$TAG..."
        
        # Get the digest (SHA256) for the image tag
        DIGEST=$(crane digest "$REGISTRY:$TAG" 2>/dev/null)
        
        if [[ -n "$DIGEST" ]]; then
            echo "Deleting $REGISTRY@$DIGEST..."
            crane delete "$REGISTRY@$DIGEST"
        else
            echo "Failed to retrieve digest for $REGISTRY:$TAG, skipping..."
        fi
    fi
done

echo "Cleanup complete."
