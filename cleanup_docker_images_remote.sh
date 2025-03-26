#!/bin/bash

# Set your Vultr container registry URL
REGISTRY="ewr.vultrcr.com/grodt/app"

# Get the list of tags from the registry
TAGS=$(crane ls $REGISTRY)

# Define the version threshold
THRESHOLD="3.20.5"

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
