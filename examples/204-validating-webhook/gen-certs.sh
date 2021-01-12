#!/usr/bin/env bash

NAMESPACE=example-204
SERVICE_NAME=example-204-validating-service

CERT_NAME=${SERVICE_NAME}.${NAMESPACE}

set -eo pipefail

echo =================================================================
echo THIS SCRIPT IS NOT SECURE! USE IT ONLY FOR DEMONSTATION PURPOSES.
echo =================================================================
echo

mkdir -p validating-certs && cd validating-certs

if [[ -e cert.key  ]] ; then
  read -p "Regenerate certificates? (yes/no) [no]: "
  if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]
  then
    exit 0
  fi
fi

RM_FILES="cert.crt cert.key cert.csr ca.crt"
echo ">>> Remove ${RM_FILES}"
rm -f $RM_FILES

# generate cert.key, cert.csr
echo ">>> Generate cert.key and cert.csr"
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

mv server-key.pem cert.key
mv server.csr cert.csr

echo ">>> Delete previous CertificateSigningRequest"
kubectl delete certificatesigningrequest/${CERT_NAME} --ignore-not-found


echo ">>> Create CertificateSigningRequest"

# create CertificateSigningRequest resource
# name is in form serviceName.namespace
if [[ ${CSR_V1BETA1} == "yes" ]] ; then
  cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${CERT_NAME}
  labels:
    heritage: example-204
spec:
  request: $(cat cert.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF
else
  cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: ${CERT_NAME}
  labels:
    heritage: example-204
spec:
  request: $(cat cert.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF
fi

kubectl certificate approve $CERT_NAME

echo ">>> Retrieve server.crt"
kubectl get certificatesigningrequest $CERT_NAME -o jsonpath='{.status.certificate}' | base64 -d > cert.crt

echo ">>> Delete CertificateSigningRequest"
(kubectl delete certificatesigningrequest/${CERT_NAME} || true )

echo ">>> Retrieve ca.crt"
kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}' | base64 -d > ca.crt
