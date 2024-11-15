# syntax=docker/dockerfile:1.7-labs

FROM golang:1.23 AS build-stage

# Install some network related stuff for debugging
RUN apt update && apt -y install iproute2

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod ./
RUN go mod download

COPY --exclude=go.mod --exclude=config.json . ./
# Build
# WORKDIR dockertest

# To bind to a TCP port, runtime parameters must be supplied to the docker command.
# But we can (optionally) document in the Dockerfile what ports
# the application is going to listen on by default.
# https://docs.docker.com/engine/reference/builder/#expose
EXPOSE 8080

# Download IP configs
COPY config.json ./

FROM build-stage AS compile-stage
RUN CGO_ENABLED=0 GOOS=linux go build -o ./go-main ./dockertest/main.go

# Run
CMD ["./go-main"]