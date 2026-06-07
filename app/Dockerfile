# syntax=docker/dockerfile:1.17

# Build stage for frontend
FROM node:22-alpine AS frontend-builder

WORKDIR /app/web

ENV CI=true

# Copy package files
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./

# Install pnpm and dependencies
RUN npm install -g pnpm && pnpm install --frozen-lockfile

# Copy source files and build
COPY web/ .
RUN pnpm run build

# Build stage for backend
FROM golang:1.26-alpine AS backend-builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mqtt-huawei .

# Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy the binary from builder stage
COPY --from=backend-builder /app/mqtt-huawei /mqtt-huawei

# Copy the frontend build from frontend-builder stage
COPY --from=frontend-builder /app/web/dist /web/dist

# Create a non-root user (distroless already provides this)
USER nonroot:nonroot

ENTRYPOINT ["/mqtt-huawei", "/var/lib/mqtt-huawei/config.json"]
