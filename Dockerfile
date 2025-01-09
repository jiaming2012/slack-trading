# version="2.0.21"

# Start from the latest Debian base image
FROM ewr.vultrcr.com/grodt/grodt-base-image-2:2.0.12

# Install git
RUN apt-get update && apt-get install -y git

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Ensure all dependencies are available
RUN go mod tidy

# Build the Go app
ARG VERSION
RUN go build -ldflags "-X github.com/jiaming2012/slack-trading/src/eventservices.Version=${VERSION}" -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]