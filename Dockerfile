# Start from the latest golang base image
FROM golang:1.20

# Add Maintainer Info
LABEL maintainer="Jamal Cole <jac475@cornell.edu>"

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Install necessary packages and Python 3.9 from the bullseye repositories
RUN apt-get update && apt-get install -y \
    software-properties-common \
    && echo 'deb http://deb.debian.org/debian bullseye main' > /etc/apt/sources.list.d/bullseye.list \
    && apt-get update \
    && apt-get install -y \
    python3.9 \
    python3.9-venv \
    python3.9-dev \
    && rm -rf /var/lib/apt/lists/*

# Create a virtual environment with Python 3.9
RUN python3.9 -m venv /app/slack-trading/src/cmd/stats/env

# Activate the virtual environment and install the Python dependencies
RUN /app/slack-trading/src/cmd/stats/env/bin/pip install -r /app/slack-trading/src/cmd/stats/requirements.txt

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]
