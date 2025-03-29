# Build from a golang based image
FROM golang:latest as builder

LABEL maintainer="Richard Lora <richard@pcscorp.dev>"

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN make build/unix

# Start a new stage from scratch
FROM alpine:latest

WORKDIR /app

RUN mkdir /downloads

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/comic-downloader /usr/bin/comic-downloader

COPY /docker/entrypoint.sh /usr/bin/entrypoint.sh
RUN chmod +x /usr/bin/entrypoint.sh

# Set comic-downloader as the entrypoint
ENTRYPOINT ["/usr/bin/entrypoint.sh", "-o", "/downloads"]
