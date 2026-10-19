# ---------- Build stage ----------
FROM golang:1.24 AS build

ENV GOTOOLCHAIN=auto
ARG TARGETARCH

WORKDIR /src

# Go modules
COPY go.mod go.sum ./
RUN go mod download

# App source
COPY . .

# Build the API binary (cmd/api/main.go)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -o /app/qr-feedback-api ./cmd/api

# ---------- Runtime stage ----------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy only the compiled binary
COPY --from=build /app/qr-feedback-api /app/qr-feedback-api

# App listens on 8080 (from your config)
EXPOSE 8080

ENTRYPOINT ["/app/qr-feedback-api"]
