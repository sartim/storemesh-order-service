# syntax=docker/dockerfile:1.7

FROM golang:1.26.6-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/storemesh-order-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/storemesh-order-service /app/storemesh-order-service
USER nonroot:nonroot
ENTRYPOINT ["/app/storemesh-order-service"]
