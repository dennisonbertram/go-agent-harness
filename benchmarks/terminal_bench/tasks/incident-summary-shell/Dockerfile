FROM go-agent-harness-tb-base:latest

COPY data /app/data
COPY scripts /app/scripts
COPY run-tests.sh /app/run-tests.sh

RUN chmod +x /app/run-tests.sh /app/scripts/render_incident_summary.sh
