# Prebuilt jq from deckhouse base images (v1.0.40).
FROM --platform=${TARGETPLATFORM:-linux/amd64} registry.deckhouse.io/container-factory@sha256:4b36dcf53c35b50e0afbc445232713aff15f788a61b832cd720bf9e88fc9fba8 AS libjq

# Go builder stage (builder/golang-alpine, Go 1.26.4 on alpine 3.22).
FROM --platform=${TARGETPLATFORM:-linux/amd64} registry.deckhouse.io/container-factory@sha256:193e8ed6cd7fc19015ab615ccf92d0fe02471e66e3e5abf560b3a87fb05bdb62 AS builder

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
ADD . /app

# Build the application
RUN GOOS=linux \
    go build \
    -ldflags="-s -w -X 'github.com/flant/shell-operator/pkg/app.Version=${appVersion}'" \
    -o shell-operator \
    ./cmd/shell-operator

# Final runtime image (builder/alpine 3.22 from deckhouse base images v1.0.40).
FROM --platform=${TARGETPLATFORM:-linux/amd64} registry.deckhouse.io/container-factory@sha256:8fa8cf713bf8cfc9038901e5b2fbc97d0403794d834dc4a619e9e81312a6feef

ARG TARGETPLATFORM
ARG kubectlVersion=v1.34.8

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    bash \
    sed \
    tini \
    curl

# Determine kubectl architecture and download
RUN kubectlArch=$(echo ${TARGETPLATFORM:-linux/amd64} | sed 's/\/v7//') && \
    echo "Downloading kubectl version ${kubectlVersion} for ${kubectlArch}" && \
    wget https://dl.k8s.io/release/${kubectlVersion}/bin/${kubectlArch}/kubectl -O /bin/kubectl && \
    chmod +x /bin/kubectl

# Create hooks directory
RUN mkdir /hooks

# Copy necessary files
ADD frameworks/shell /frameworks/shell
ADD shell_lib.sh /
COPY --from=libjq /bin/jq /usr/bin
COPY --from=builder /app/shell-operator /

# Set working directory
WORKDIR /

# Set environment variables
ENV SHELL_OPERATOR_HOOKS_DIR=/hooks
ENV LOG_TYPE=json

# Set entrypoint and default command
ENTRYPOINT ["/sbin/tini", "--", "/shell-operator"]
CMD ["start"]
