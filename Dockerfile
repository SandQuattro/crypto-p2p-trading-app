# Multi-stage Dockerfile for Crypto P2P Trading App

# Stage 1: Build the React frontend
FROM node:18-alpine AS frontend-builder
WORKDIR /app/frontend

# Declare the build argument
ARG REACT_APP_API_URL

COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./

# Use the build argument when running the build command
# This makes the ARG available as process.env.REACT_APP_API_URL during the build
RUN REACT_APP_API_URL=$REACT_APP_API_URL npm run build

# Stage 2: Build the Go backend
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download the Go module dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# go app build
RUN CGO_ENABLED=0 GOOS=linux go build -o crypto-trading-server ./cmd/trading

# Stage 3: Final image
FROM alpine:3.18

WORKDIR /app

RUN apk --no-cache add ca-certificates curl

# Copy the compiled backend
COPY --from=backend-builder /app/crypto-trading-server /app/

# Copy the frontend build
COPY --from=frontend-builder /app/frontend/build /app/static

# Copy migrations
COPY --from=backend-builder /app/migrations /app/migrations

# Copy config directory
COPY --from=backend-builder /app/config /app/config

# Expose the port
EXPOSE 8080

# Command to run the application
CMD ["./crypto-trading-server"]
