#!/usr/bin/env bash
# Generate self-signed TLS certificates for local development.
set -euo pipefail

CERT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Generating self-signed TLS certificates in ${CERT_DIR}..."

# Generate CA key and certificate.
openssl genrsa -out "${CERT_DIR}/ca.key" 2048

openssl req -x509 -new -nodes \
  -key "${CERT_DIR}/ca.key" \
  -sha256 -days 365 \
  -out "${CERT_DIR}/ca.crt" \
  -subj "/C=US/ST=Dev/L=Local/O=ReverseProxy/CN=ReverseProxyCA"

# Generate server key.
openssl genrsa -out "${CERT_DIR}/server.key" 2048

# Generate server CSR.
openssl req -new \
  -key "${CERT_DIR}/server.key" \
  -out "${CERT_DIR}/server.csr" \
  -subj "/C=US/ST=Dev/L=Local/O=ReverseProxy/CN=localhost"

# Create extensions file for SAN.
cat > "${CERT_DIR}/server_ext.cnf" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = proxy
IP.1 = 127.0.0.1
EOF

# Sign the server certificate with the CA.
openssl x509 -req \
  -in "${CERT_DIR}/server.csr" \
  -CA "${CERT_DIR}/ca.crt" \
  -CAkey "${CERT_DIR}/ca.key" \
  -CAcreateserial \
  -out "${CERT_DIR}/server.crt" \
  -days 365 \
  -sha256 \
  -extfile "${CERT_DIR}/server_ext.cnf"

# Clean up intermediate files.
rm -f "${CERT_DIR}/server.csr" "${CERT_DIR}/server_ext.cnf" "${CERT_DIR}/ca.srl"

echo "Certificates generated:"
echo "  CA:     ${CERT_DIR}/ca.crt"
echo "  Cert:   ${CERT_DIR}/server.crt"
echo "  Key:    ${CERT_DIR}/server.key"
