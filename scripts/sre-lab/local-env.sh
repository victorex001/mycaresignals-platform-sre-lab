#!/usr/bin/env bash
# Source this file in the lab terminal; no global shell or Docker settings change.
if [[ -n "${BASH_VERSION:-}" ]]; then
  if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    echo 'Source this file: source scripts/sre-lab/local-env.sh' >&2
    exit 64
  fi
  sre_lab_env_file="${BASH_SOURCE[0]}"
elif [[ -n "${ZSH_VERSION:-}" ]]; then
  sre_lab_env_file="${(%):-%x}"
else
  echo 'Source this file from Bash or zsh.' >&2
  return 64
fi
sre_lab_repo_root="$(cd "$(dirname "$sre_lab_env_file")/../.." && pwd)"
if [[ -n "${AWS_ACCESS_KEY_ID:-}${AWS_SECRET_ACCESS_KEY:-}${AWS_SESSION_TOKEN:-}${AWS_WEB_IDENTITY_TOKEN_FILE:-}" ]]; then
  echo 'Credential environment overrides are present (values hidden). Use a clean lab terminal before sourcing this file.' >&2
  return 65
fi
export AWS_PROFILE=sre-lab AWS_DEFAULT_PROFILE=sre-lab
export AWS_REGION=eu-west-2 AWS_DEFAULT_REGION=eu-west-2
export SRE_LAB_ACCOUNT_ID=266877689034
export DOCKER_CONTEXT=colima-sre-lab
export KUBECONFIG="$sre_lab_repo_root/.tmp/kubeconfig-sre-lab"
export PATH="$sre_lab_repo_root/.tmp/sre-lab-tools.nosync/bin:$PATH"
unset sre_lab_repo_root sre_lab_env_file
