FROM hashicorp/terraform:1.9

RUN apk update && apk add --no-cache \
      bash curl git jq unzip python3 py3-virtualenv ca-certificates \
  && update-ca-certificates

# AWS CLI (venv)
RUN python3 -m venv /opt/venv \
  && /opt/venv/bin/pip install --upgrade pip \
  && /opt/venv/bin/pip install awscli
ENV PATH="/opt/venv/bin:${PATH}"

# Inject build arch from Docker
ARG TARGETARCH
# Map Docker arch values to release asset names
# (amd64 -> amd64, arm64 -> arm64)
RUN set -eux; \
    case "$TARGETARCH" in \
      amd64)  TFLINT_ARCH="amd64"; TFSEC_ARCH="amd64" ;; \
      arm64)  TFLINT_ARCH="arm64"; TFSEC_ARCH="arm64" ;; \
      *)      echo "Unsupported arch: $TARGETARCH"; exit 1 ;; \
    esac; \
    TFLINT_VERSION="$(curl -s https://api.github.com/repos/terraform-linters/tflint/releases/latest | jq -r .tag_name)"; \
    curl -L "https://github.com/terraform-linters/tflint/releases/download/${TFLINT_VERSION}/tflint_linux_${TFLINT_ARCH}.zip" -o /tmp/tflint.zip; \
    unzip /tmp/tflint.zip -d /usr/local/bin; \
    chmod +x /usr/local/bin/tflint; \
    rm -f /tmp/tflint.zip; \
    TFSEC_VERSION="$(curl -s https://api.github.com/repos/aquasecurity/tfsec/releases/latest | jq -r .tag_name)"; \
    curl -L "https://github.com/aquasecurity/tfsec/releases/download/${TFSEC_VERSION}/tfsec-linux-${TFSEC_ARCH}" -o /usr/local/bin/tfsec; \
    chmod +x /usr/local/bin/tfsec

RUN terraform --version && aws --version && tflint --version && tfsec --version
CMD ["bash"]