ARG GO_VERSION=1.25.0

FROM golang:${GO_VERSION}-alpine AS dev-stage

RUN apk add --no-cache ca-certificates git

# Hot reload tool
RUN go install github.com/air-verse/air@latest

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ENV GIN_MODE=debug
ENV APP_PORT=8080

EXPOSE 8080

ENTRYPOINT ["air"]

FROM golang:${GO_VERSION}-alpine AS build-stage

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy entire source tree (subdirs included)
COPY . .

# Build the server binary from cmd/server
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags='-s -w' \
    -o /go-api-foundry \
    ./cmd/server

FROM golang:${GO_VERSION}-alpine AS run-test-stage

RUN apk add --no-cache ca-certificates
WORKDIR /app

# Reuse mod cache from build
COPY --from=build-stage /go/pkg/mod /go/pkg/mod
COPY go.mod go.sum ./
COPY . .

# Run tests across all packages
RUN go test -v -race ./...

FROM scratch AS production-stage

ARG CA_CERT_PATH=/etc/ssl/certs/ca-certificates.crt

# Copy the statically built binary and CA certs from build stage
COPY --from=build-stage /go-api-foundry /go-api-foundry
COPY --from=build-stage ${CA_CERT_PATH} /etc/ssl/certs/


ENV GIN_MODE=release
ENV APP_PORT=8080
ENV APP_ENV=docker

EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/go-api-foundry"]
