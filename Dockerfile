# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY backend/go.mod backend/go.sum* ./
RUN go mod download

# Copy backend and build
COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o vocat-sip-backend .

# Final stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy pre-built frontend assets (from build context)
COPY assets/ ./assets/

# Copy backend binary
COPY --from=builder /app/vocat-sip-backend .
COPY vocat-plugin.json .

EXPOSE 5060/udp 8080

ENTRYPOINT ["./vocat-sip-backend"]