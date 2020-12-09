#!/usr/bin/env bash

set -eo pipefail

echo =================================================================
echo THIS SCRIPT IS NOT SECURE! USE IT ONLY FOR DEMONSTATION PURPOSES.
echo =================================================================
echo

mkdir -p validating-certs

cd validating-certs

if [[ -e .generated  ]] ; then
  read -p "Regenerate certificates? (yes/no) [no]: "
  if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]
  then
    exit 0
  fi
fi

rm -f server.crt server.csr server-key.pem cluster-ca.pem


# generate server-key.pem server.csr
echo ">>> Generate server-key.pem"
cat ../csr.json| cfssl genkey - | cfssljson -bare server

echo ">>> Delete CertificateSigningRequest"
(kubectl delete certificatesigningrequest/shell-operator-validating-service.shell-test || true )


echo ">>> Create CertificateSigningRequest"

# create CertificateSigningRequest resource
# name is in form serviceName.namespace
cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: shell-operator-validating-service.shell-test
spec:
  request: $(cat server.csr | base64 | tr -d '\n')
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF

# certificatesigningrequest.certificates.k8s.io/shell-operator-validating-service.shell-test created

kubectl certificate approve shell-operator-validating-service.shell-test

kubectl get certificatesigningrequest

echo ">>> Retrieve server.crt from cluster"
kubectl get certificatesigningrequest shell-operator-validating-service.shell-test -o jsonpath='{.status.certificate}' | base64 -d > server.crt

echo ">>> Retrieve cluster-ca.pem from cluster"
kubectl config view --raw --minify --flatten -o jsonpath='{.clusters[].cluster.certificate-authority-data}' > cluster-ca.pem

touch .generated
