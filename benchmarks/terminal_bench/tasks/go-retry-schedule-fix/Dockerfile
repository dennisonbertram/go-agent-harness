FROM go-agent-harness-tb-base:latest

COPY go.mod /app/go.mod
COPY retry.go /app/retry.go
COPY retry_test.go /app/retry_test.go
COPY run-tests.sh /app/run-tests.sh

RUN chmod +x /app/run-tests.sh
