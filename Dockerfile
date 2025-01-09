# version="3.0.0"

# Start from the latest Debian base image
FROM ewr.vultrcr.com/grodt/grodt-base-image-2:1.1.14

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -ldflags "-X github.com/jiaming2012/slack-trading/src/eventservices.Version=1.4.2 -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]