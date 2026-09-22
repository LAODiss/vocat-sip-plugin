# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install node for frontend build
RUN apk add --no-cache nodejs npm

# Copy go mod files
COPY backend/go.mod backend/go.sum* ./
RUN go mod download

# Copy frontend and build
COPY frontend/ ./frontend/
WORKDIR /app/frontend
RUN npm ci && npm run build

# Build backend
WORKDIR /app
COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o vocat-sip-backend .

# Final stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/vocat-sip-backend .
COPY --from=builder /app/frontend/assets/ ./assets/
COPY vocat-plugin.json .

EXPOSE 5060/udp 8080

ENTRYPOINT ["./vocat-sip-backend"]