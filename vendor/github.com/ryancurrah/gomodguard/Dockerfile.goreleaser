# ---- App container
FROM golang:alpine
WORKDIR /
RUN apk --no-cache add ca-certificates
COPY gomodguard /gomodguard
ENTRYPOINT ./gomodguard
