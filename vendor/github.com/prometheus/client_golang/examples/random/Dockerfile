# This Dockerfile builds an image for a client_golang example.
#
# Use as (from the root for the client_golang repository):
#    docker build -f examples/$name/Dockerfile -t prometheus/golang-example-$name .

# Builder image, where we build the example.
FROM golang:1.9.0 AS builder
WORKDIR /go/src/github.com/prometheus/client_golang
COPY . .
WORKDIR /go/src/github.com/prometheus/client_golang/prometheus
RUN go get -d
WORKDIR /go/src/github.com/prometheus/client_golang/examples/random
RUN CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w'

# Final image.
FROM scratch
LABEL maintainer "The Prometheus Authors <prometheus-developers@googlegroups.com>"
COPY --from=builder /go/src/github.com/prometheus/client_golang/examples/random .
EXPOSE 8080
ENTRYPOINT ["/random"]
