FROM go-agent-harness-tb-base:latest

COPY go.mod /app/go.mod
COPY main.go /app/main.go
COPY storage.go /app/storage.go
COPY file_storage.go /app/file_storage.go
COPY consumer.go /app/consumer.go
COPY storage_test.go /app/storage_test.go
COPY run-tests.sh /app/run-tests.sh

RUN chmod +x /app/run-tests.sh
