ARG GO_VERSION=1.24.4

FROM golang:$GO_VERSION AS build
RUN go install github.com/a8m/envsubst/cmd/envsubst@v1.4.3

ARG IMAGE_TAG
COPY docker-compose.yml /docker-compose.yml.template
RUN IMAGE_TAG=${IMAGE_TAG} cat /docker-compose.yml.template | envsubst > /docker-compose.yml

FROM scratch
COPY --from=build /docker-compose.yml /docker-compose.yml
