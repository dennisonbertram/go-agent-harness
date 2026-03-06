FROM golang:1.22-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    asciinema \
    ca-certificates \
    curl \
    git \
    python3 \
    python3-pytest \
    tmux \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY data /app/data
COPY scripts /app/scripts
COPY run-tests.sh /app/run-tests.sh

RUN chmod +x /app/run-tests.sh /app/scripts/render_incident_summary.sh
