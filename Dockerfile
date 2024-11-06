<<<<<<< HEAD
# version="1.1.18"
=======
# version="1.3.0"
>>>>>>> dev

# Start from the latest Debian base image
FROM ewr.vultrcr.com/grodt/grodt-base-image-2:1.1.14

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]