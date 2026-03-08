FROM go-agent-harness-tb-base:latest

COPY go.mod /app/go.mod
COPY main.go /app/main.go
COPY counter.go /app/counter.go
COPY counter_test.go /app/counter_test.go
COPY run-tests.sh /app/run-tests.sh

RUN chmod +x /app/run-tests.sh
