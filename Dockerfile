# version="1.0.20"

# Start from the latest Debian base image
FROM docker.io/jiamingnj/grodt-base-image-2

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]