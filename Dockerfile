# Go builder stage
FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.25.5-alpine3.23 AS builder

ARG appVersion=latest

# Install build dependencies
RUN apk --no-cache add \
    git \
    ca-certificates \
    gcc \
    musl-dev \
    libc-dev \
    binutils-gold

# Set working directory
WORKDIR /app

# Cache-friendly dependency download
ADD go.mod go.sum ./
RUN go mod download

# Add source code (changes here invalidate cache less frequently)
ADD cmd ./cmd
ADD pkg ./pkg

# Build the application
RUN GOOS=linux \
    go build \
    -ldflags="-s -w -X 'github.com/flant/shell-operator/pkg/app.Version=${appVersion}'" \
    -o shell-operator \
    ./cmd/shell-operator

# Final runtime image
FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.23

ARG TARGETPLATFORM

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    bash \
    sed \
    tini \
    curl

# Determine kubectl architecture and download
RUN kubectlArch=$(echo ${TARGETPLATFORM:-linux/amd64} | sed 's/\/v7//') && \
    echo "Downloading kubectl for ${kubectlArch}" && \
    wget https://dl.k8s.io/release/v1.32.10/bin/${kubectlArch}/kubectl -O /bin/kubectl && \
    chmod +x /bin/kubectl

# Create hooks directory
RUN mkdir /hooks

# Copy necessary files
ADD frameworks/shell /frameworks/shell
ADD shell_lib.sh /
COPY --from=builder /app/shell-operator /

# Set working directory
WORKDIR /

# Set environment variables
ENV SHELL_OPERATOR_HOOKS_DIR=/hooks
ENV LOG_TYPE=json

# Set entrypoint and default command
ENTRYPOINT ["/sbin/tini", "--", "/shell-operator"]
CMD ["start"]
