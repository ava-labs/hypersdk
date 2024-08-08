#!/bin/bash

# Get the directory of the script
scriptsDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Change to the docker directory
cd "$scriptsDir/../docker" || exit 1

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo "Error: .env file not found. Please copy .env.example to .env and configure it."
    exit 1
fi

# Build and start the Docker containers
echo "Building Docker containers..."
docker compose build

echo "Starting Docker containers..."
docker compose up -d

echo "Docker compose up completed successfully."
