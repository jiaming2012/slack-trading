#!/bin/bash

# Set API Key
API_KEY="gC3qtcGKmvgm9AWWCfp3CgskYJoPTi6XP5KC"

# Get all repositories
REPOSITORIES=$(curl -s -X GET "https://ewr.vultrcr.com/grodt" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" | jq -r '.repositories[].name')

# Loop through each repository and list images
# echo "Listing all images in Vultr Container Registry..."
# for REPO in $REPOSITORIES; do
#     echo "Repository: $REPO"
#     curl -s -X GET "https://api.vultr.com/v2/container-registry/$REPO/tags" \
#         -H "Authorization: Bearer $API_KEY" \
#         -H "Content-Type: application/json" | jq -r '.tags[].name'
#     echo "------------------------------------"
# done
