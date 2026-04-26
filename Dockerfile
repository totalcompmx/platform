# Build stage
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/bin/web ./cmd/web

# Runtime stage
FROM --platform=$TARGETPLATFORM alpine:latest

WORKDIR /app

# Install runtime dependencies: ca-certificates for HTTPS + Chromium for PDF generation
RUN apk --no-cache add \
    ca-certificates \
    chromium \
    chromium-chromedriver \
    nss \
    freetype \
    harfbuzz \
    ttf-freefont

# Set Chrome environment variables for headless operation
ENV CHROME_BIN=/usr/bin/chromium-browser \
    CHROME_PATH=/usr/lib/chromium/ \
    CHROMIUM_FLAGS="--disable-software-rasterizer --disable-dev-shm-usage --no-sandbox --disable-gpu"

# Copy binary from builder
COPY --from=builder /app/bin/web .

# Copy assets (templates, migrations, static files)
COPY --from=builder /app/assets ./assets

# Expose port
EXPOSE 3080

# Run the application
CMD ["./web"]

