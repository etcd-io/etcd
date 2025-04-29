FROM ubuntu:latest

# Install dependencies
RUN apt-get update -y && apt-get install -y pip wget

# GO:

# Install go
RUN wget https://golang.org/dl/go1.22.5.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin 

# Set working directory
WORKDIR /opt/antithesis/entrypoint

# Copy your entrypoint script
COPY ./entrypoint/go.mod ./go.mod
COPY ./entrypoint/go.sum ./go.sum
COPY ./entrypoint/entrypoint.go ./entrypoint.go

# Run go mod tidy to ensure the dependencies are correct
RUN go mod tidy

# Building go executable and putting it in Test Composer directory
RUN go build -o ./entrypoint -race

# Copying go script and building an executable into Test Composer directory
WORKDIR /opt/antithesis/go-delete-keys

COPY ./go-delete-keys/go.mod ./go.mod
COPY ./go-delete-keys/go.sum ./go.sum
COPY ./go-delete-keys/serial_driver_delete_keys.go ./serial_driver_delete_keys.go

# Building go executable and putting it in Test Composer directory
RUN go build -o /opt/antithesis/test/v1/main/serial_driver_delete_keys -race