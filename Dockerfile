# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git just in case dependencies need it
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o linklet .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/linklet .

# Expose the port defined in config.go (default 8080)
EXPOSE 8080

CMD ["./linklet"]
