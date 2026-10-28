FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.26.4-alpine3.23@sha256:18b460dd17542c2ba43299a633cf6ebfc1115101509531471d7cfce1019af083 AS builder

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

FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.23@sha256:ff7f0c727dd796aa36787bfcd9a64bf35a6ce72a73451b3359fde849b6703d63

ARG TARGETPLATFORM
ARG kubectlVersion=v1.35.8

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    bash \
    sed \
    tini \
    jq \
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
COPY --from=builder /app/shell-operator /

# Reject incorrect architectures before the image can be published.
RUN --mount=type=bind,source=scripts/ci/verify-image.sh,target=/tmp/verify-image.sh \
    apk add --no-cache --virtual .image-verification file && \
    sh /tmp/verify-image.sh "${TARGETPLATFORM:-linux/amd64}" && \
    apk del .image-verification

# Set working directory
WORKDIR /

# Set environment variables
ENV SHELL_OPERATOR_HOOKS_DIR=/hooks
ENV LOG_TYPE=json

# Set entrypoint and default command
ENTRYPOINT ["/sbin/tini", "--", "/shell-operator"]
CMD ["start"]
