FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Production image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata mysql-client bash

WORKDIR /app

COPY --from=builder /app/server /app/server
COPY --from=builder /app/scripts /app/scripts

# Download Swagger UI assets for offline use
RUN apk add --no-cache curl && \
    mkdir -p /app/static/swagger && \
    curl -sL "https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" -o /app/static/swagger/swagger-ui-bundle.js && \
    curl -sL "https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" -o /app/static/swagger/swagger-ui.css && \
    apk del curl

# Create directories
RUN mkdir -p /app/exports /app/backups

EXPOSE 8080

CMD ["/app/server"]
