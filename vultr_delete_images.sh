#!/bin/bash

# Set Vultr API credentials
API_KEY="gC3qtcGKmvgm9AWWCfp3CgskYJoPTi6XP5KC"
REGISTRY="ewr.vultrcr.com/grodt/app"
CUTOFF_VERSION="3.14.3"

# Function to fetch all image tags
get_image_tags() {
    curl -s -X GET "https://api.vultr.com/v2/container-registry/$REGISTRY/tags" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" | jq -r '.tags[].name'
}

# Get list of image tags
IMAGE_TAGS=$(get_image_tags)

# Identify images to delete
images_to_delete=()
for TAG in $IMAGE_TAGS; do
    if [[ "$TAG" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        if [[ "$(printf "%s\n%s" "$CUTOFF_VERSION" "$TAG" | sort -V | head -n1)" != "$CUTOFF_VERSION" ]]; then
            images_to_delete+=("$TAG")
            echo "Will delete: $REGISTRY:$TAG"
        fi
    fi
done

# Confirm deletion
if [[ ${#images_to_delete[@]} -gt 0 ]]; then
    echo ""
    read -p "Do you want to delete all images before $CUTOFF_VERSION? (y/N) " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        for TAG in "${images_to_delete[@]}"; do
            echo "Deleting $REGISTRY:$TAG..."
            curl -s -X DELETE "https://api.vultr.com/v2/container-registry/$REGISTRY/tags/$TAG" \
                -H "Authorization: Bearer $API_KEY" \
                -H "Content-Type: application/json"
            echo "Deleted: $REGISTRY:$TAG"
        done
        echo "Deletion complete."
    else
        echo "Operation canceled."
    fi
else
    echo "No images found before $CUTOFF_VERSION."
fi
