# version="3.26.0"

# Start from the latest Debian base image
FROM grodt-base-image-2:3.9.0

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Install pandas_ta_classic (not in base image yet)
RUN /opt/conda/envs/grodt/bin/pip install pandas_ta_classic==0.4.47 --no-deps

# Build the Go app
RUN go build -o main ./cmd/main.go

# Command to run the executable
CMD ["./main"]