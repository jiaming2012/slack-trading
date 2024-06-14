# Start from the latest Ubuntu base image
FROM ubuntu:20.04

# Set environment variable to avoid interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install necessary packages
RUN apt-get update && apt-get install -y \
    software-properties-common \
    wget \
    build-essential \
    libssl-dev \
    zlib1g-dev \
    libncurses5-dev \
    libncursesw5-dev \
    libreadline-dev \
    libsqlite3-dev \
    libgdbm-dev \
    libdb5.3-dev \
    libbz2-dev \
    libexpat1-dev \
    liblzma-dev \
    tk-dev \
    libffi-dev \
    python3-venv \
    python3-dev \
    gfortran \
    libblas-dev \
    liblapack-dev \
    libjpeg-dev \
    libpng-dev \
    libfreetype6-dev \
    liblcms2-dev \
    libtiff5-dev \
    libopenjp2-7-dev \
    libwebp-dev \
    libharfbuzz-dev \
    libfribidi-dev \
    tcl-dev \
    tk-dev 

# Download and install Python 3.7.9
RUN wget https://www.python.org/ftp/python/3.7.9/Python-3.7.9.tgz && \
    tar xvf Python-3.7.9.tgz && \
    cd Python-3.7.9 && \
    ./configure --enable-optimizations && \
    make altinstall

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Ensure pip is installed and install venv
RUN python3.7 -m ensurepip && python3.7 -m pip install --upgrade pip setuptools wheel && python3.7 -m pip install virtualenv

# Create a virtual environment with Python 3.9
RUN python3 -m venv /app/slack-trading/src/cmd/stats/env

RUN /app/slack-trading/src/cmd/stats/env/bin/python3 -m pip install wheel

RUN /app/slack-trading/src/cmd/stats/env/bin/pip install Pillow

# Copy the source from the current directory to the Working Directory inside the container
COPY src/cmd/stats/requirements.txt /app/slack-trading/src/cmd/stats/requirements.txt

# Activate the virtual environment and install the Python dependencies
RUN /app/slack-trading/src/cmd/stats/env/bin/pip install -r /app/slack-trading/src/cmd/stats/requirements.txt

# Download and install Go 1.20
RUN wget https://golang.org/dl/go1.20.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.20.linux-amd64.tar.gz && \
    rm go1.20.linux-amd64.tar.gz

# Set up Go environment variables
ENV PATH="/usr/local/go/bin:${PATH}"

# Verify installation
RUN go version

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Set the Current Working Directory inside the container
WORKDIR /app/slack-trading

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -o main ./src/eventmain/main.go

# Command to run the executable
CMD ["./main"]