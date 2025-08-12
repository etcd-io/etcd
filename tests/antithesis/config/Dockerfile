ARG GO_VERSION=1.24.6

FROM golang:$GO_VERSION AS build
RUN go install github.com/a8m/envsubst/cmd/envsubst@v1.4.3

ARG IMAGE_TAG
ARG NODE_COUNT
COPY docker-compose-${NODE_COUNT}-node.yml /docker-compose.yml.template
RUN IMAGE_TAG=${IMAGE_TAG} cat /docker-compose.yml.template | envsubst > /docker-compose.yml

FROM scratch
COPY --from=build /docker-compose.yml /docker-compose.yml
