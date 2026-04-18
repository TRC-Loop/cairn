# syntax=docker/dockerfile:1.6

FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /cairn ./cmd/cairn

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /cairn /cairn
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/cairn"]
