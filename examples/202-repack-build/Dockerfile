FROM flant/shell-operator:v1.0.0-beta.14 as shell-operator

# Final image with kubectl 1.17 and curl
FROM ubuntu:20.04
RUN apt-get update && \
    apt-get install -y jq ca-certificates bash tini curl && \
    wget https://storage.googleapis.com/kubernetes-release/release/v1.17.4/bin/linux/amd64/kubectl -O /bin/kubectl && \
    chmod +x /bin/kubectl && \
    mkdir /hooks

COPY --from=shell-operator /shell-operator /shell-operator
COPY --from=shell-operator /frameworks /
COPY --from=shell-operator /shell_lib.sh /

COPY hooks /hooks

WORKDIR /
ENV SHELL_OPERATOR_HOOKS_DIR /hooks
ENV LOG_TYPE json
ENTRYPOINT ["/sbin/tini", "--", "/shell-operator"]
CMD ["start"]
