#!/bin/bash

CONTAINER_NAME="neo4j-graphdb"
BASE_PATH="$(pwd)/.gemini/graph_data/neo4j"

WIPE_DATA=false

# Parse arguments
for arg in "$@"; do
    if [ "$arg" == "--wipe-data" ]; then
        WIPE_DATA=true
    fi
done

echo "Checking for container '${CONTAINER_NAME}'..."

# Check if container exists
if podman ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    # Check if it's running and stop it
    if podman ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Stopping container '${CONTAINER_NAME}'..."
        podman stop "${CONTAINER_NAME}"
    fi
    
    echo "Removing container '${CONTAINER_NAME}'..."
    podman rm "${CONTAINER_NAME}"
    echo "Container '${CONTAINER_NAME}' successfully removed."
else
    echo "Container '${CONTAINER_NAME}' does not exist."
fi

# Handle data wiping
if [ "$WIPE_DATA" = true ]; then
    echo "Wiping Neo4j data directories at ${BASE_PATH}..."
    rm -rf "${BASE_PATH}/data"
    rm -rf "${BASE_PATH}/logs"
    rm -rf "${BASE_PATH}/conf"
    echo "Data wiped."
else
    echo "Note: Local graph data in ${BASE_PATH} was NOT removed."
    echo "To completely start fresh and delete the database files, run this script with the --wipe-data flag."
fi
