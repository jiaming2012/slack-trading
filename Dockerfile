# version="2.1.0"

# Start from the latest Debian base image
FROM ewr.vultrcr.com/grodt/grodt-base-image-2:2.0.12

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Ensure all dependencies are available
RUN go mod tidy

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]