# Build stage: compile the binary using a golang image.
FROM golang:latest AS builder

LABEL maintainer="Richard Lora <richard@pcscorp.dev>"

WORKDIR /app

# Copy go.mod/go.sum and download modules.
COPY go.mod go.sum ./
RUN go mod download

# Copy the source and build the binary.
COPY . .
RUN make build/unix

# Runtime stage: use a minimal Alpine image.
FROM alpine:latest

WORKDIR /app

# Create the downloads directory.
RUN mkdir /downloads

# Copy the built binary from the builder stage, renaming it to _comic-downloader.
COPY --from=builder /app/build/comic-downloader /usr/bin/_comic-downloader

# Copy the wrapper script into the image as /usr/bin/comic-downloader.
COPY docker/containers/comic-downloader/entrypoint-wrapper.sh /usr/bin/comic-downloader
RUN chmod +x /usr/bin/comic-downloader

# Set the wrapper as the container's entrypoint.
ENTRYPOINT ["/usr/bin/comic-downloader"]

# Set CMD to empty so no extra arguments are passed at startup.
CMD []
