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

# Install Python and venv
RUN apt-get update && apt-get install -y python3 python3-venv

# Create a virtual environment
RUN python3 -m venv /app/slack-trading/src/cmd/stats/env

# Activate the virtual environment and install the Python dependencies
RUN /app/slack-trading/src/cmd/stats/env/bin/pip install -r /app/slack-trading/src/cmd/stats/requirements.txt

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]