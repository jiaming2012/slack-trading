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

# Install software-properties-common, and a specific version of Python
RUN apt-get update && apt-get install -y software-properties-common && \
    add-apt-repository ppa:deadsnakes/ppa && \
    apt-get update && apt-get install -y python3.7=3.7.9-1+bionic1

# Create a virtual environment with the specific version of Python
RUN python3.7 -m venv /app/slack-trading/src/cmd/stats/env

# Activate the virtual environment and install the Python dependencies
RUN /app/slack-trading/src/cmd/stats/env/bin/pip install -r /app/slack-trading/src/cmd/stats/requirements.txt

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]