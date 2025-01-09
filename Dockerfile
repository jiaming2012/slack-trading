# version="2.4.7"

# Start from the latest Debian base image
FROM ewr.vultrcr.com/grodt/grodt-base-image-2:2.0.12

# Install git
RUN apt-get update && apt-get install -y git

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]