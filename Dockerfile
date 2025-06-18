FROM eclipse-mosquitto:2 AS builder

# Set the working directory in the container to /app
WORKDIR /app

# Copy only the go.mod and go.sum files from the root
COPY go.mod go.sum ./

# Download the dependencies
RUN go mod download

# Install curl
RUN apk add --no-cache curl

# Run stage
FROM eclipse-mosquitto:2

WORKDIR /app

# Add timezone data
RUN apk add --no-cache tzdata
