# version="2.0.14"

# Start from the latest Debian base image
FROM ewr.vultrcr.com/grodt/grodt-base-image-2:2.0.12

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -ldflags "-X github.com/jiaming2012/slack-trading/src/eventservices.Version=${VERSION}" -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]