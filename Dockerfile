# version="3.25.11"

# Start from the latest Debian base image
FROM ewr.vultrcr.com/grodt/grodt-base-image-2:3.9.0

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]