# Multi-stage build.
FROM golang:1.26.5-alpine3.24 as build-stage

# Copy source code and build binary.
ARG version
RUN mkdir /usr/app
COPY . /usr/app
WORKDIR /usr/app
RUN CGO_ENABLED=0 go build -ldflags="-s -X main.version=${version}" -o portal ./cmd/portal/

# Copy binary from build container and build image.
FROM alpine:3.24
RUN mkdir /usr/app
WORKDIR /usr/app
COPY --from=build-stage /usr/app/portal .

ENTRYPOINT [ "./portal", "serve"]
