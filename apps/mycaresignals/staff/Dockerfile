# Stage 1 - Build
FROM golang:1.25-bookworm AS build

WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/therapeutic-engine ./cmd/api

# Stage 2 - Run
FROM gcr.io/distroless/base-debian11

WORKDIR /app
COPY --from=build /out/therapeutic-engine /app/

ENV GIN_MODE=release
EXPOSE 8081

USER nonroot:nonroot
ENTRYPOINT ["/app/therapeutic-engine"]
