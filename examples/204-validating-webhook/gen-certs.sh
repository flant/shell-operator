#!/usr/bin/env bash

NAMESPACE=example-204
SERVICE_NAME=example-204-validating-service

CERT_NAME=${SERVICE_NAME}.${NAMESPACE}

set -eo pipefail

echo =================================================================
echo THIS SCRIPT IS NOT SECURE! USE IT ONLY FOR DEMONSTATION PURPOSES.
echo =================================================================
echo

mkdir -p validating-certs

cd validating-certs

if [[ -e server-key.pem  ]] ; then
  read -p "Regenerate certificates? (yes/no) [no]: "
  if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]
  then
    exit 0
  fi
fi

echo ">>> Remove server.crt server.csr server-key.pem ca.pem"
rm -f server.crt server.csr server-key.pem ca.pem

# generate server-key.pem server.csr
echo ">>> Generate server-key.pem"
cat <<EOF | cfssl genkey - | cfssljson -bare server
{
  "hosts": [
    "${CERT_NAME}.svc",
    "${CERT_NAME}.svc.cluster.local"
  ],
  "CN": "${CERT_NAME}.svc",
  "key": {
    "algo": "ecdsa",
    "size": 256
  }
}
EOF

echo ">>> Delete previous CertificateSigningRequest"
(kubectl delete certificatesigningrequest/${CERT_NAME} || true )


echo ">>> Create CertificateSigningRequest"

# create CertificateSigningRequest resource
# name is in form serviceName.namespace
cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${CERT_NAME}
  labels:
    heritage: example-204
spec:
  request: $(cat server.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF

kubectl certificate approve $CERT_NAME

echo ">>> Retrieve server.crt"
kubectl get certificatesigningrequest $CERT_NAME -o jsonpath='{.status.certificate}' | base64 -d > server.crt

echo ">>> Delete CertificateSigningRequest"
(kubectl delete certificatesigningrequest/${CERT_NAME} || true )

echo ">>> Retrieve ca.pem"
kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}' | base64 -d > ca.pem
