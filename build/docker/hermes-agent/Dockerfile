ARG HERMES_AGENT_IMAGE=nousresearch/hermes-agent:v2026.8.3

FROM ${HERMES_AGENT_IMAGE}

RUN apt-get update \
    && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
