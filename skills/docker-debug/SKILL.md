---
name: docker-debug
description: "Debug running Docker containers: logs, exec, inspect, stats, network troubleshooting, resource usage. Trigger: when debugging Docker containers, viewing container logs, docker exec, docker inspect, container stats, container troubleshooting"
version: 1
argument-hint: "[container-name or image:tag]"
allowed-tools:
  - bash
  - read
  - grep
  - glob
---
# Docker Debug

You are now operating in Docker container debugging mode.

## Container Logs

```bash
# View logs for a running container
docker logs myapp

# Follow logs in real time
docker logs -f myapp

# Last N lines
docker logs --tail=100 myapp

# With timestamps
docker logs --timestamps myapp

# Logs since a time (minutes ago)
docker logs --since=10m myapp

# Logs within a time window
docker logs --since="2026-03-10T14:00:00" --until="2026-03-10T15:00:00" myapp

# Filter stderr only
docker logs myapp 2>&1 1>/dev/null | grep ERROR
```

## Execute Commands in Running Containers

```bash
# Interactive shell in a running container
docker exec -it myapp sh
docker exec -it myapp bash

# Run a specific command
docker exec myapp ls -la /app
docker exec myapp cat /etc/resolv.conf

# Run as root (even if container uses non-root user)
docker exec -it --user root myapp sh

# Check environment variables
docker exec myapp env

# Check running processes
docker exec myapp ps aux
```

## Inspect Container Details

```bash
# Full container metadata (JSON)
docker inspect myapp

# Extract specific fields with format
docker inspect --format='{{.State.Status}}' myapp
docker inspect --format='{{.NetworkSettings.IPAddress}}' myapp
docker inspect --format='{{json .Config.Env}}' myapp | jq .
docker inspect --format='{{json .Mounts}}' myapp | jq .

# Show exposed ports and bindings
docker inspect --format='{{json .NetworkSettings.Ports}}' myapp | jq .

# Show restart policy
docker inspect --format='{{.HostConfig.RestartPolicy.Name}}' myapp

# Show resource limits
docker inspect --format='{{.HostConfig.Memory}}' myapp
docker inspect --format='{{.HostConfig.NanoCPUs}}' myapp
```

## Resource Monitoring (Stats)

```bash
# Live resource usage (CPU, memory, network, disk)
docker stats myapp

# One-shot (no live update)
docker stats --no-stream myapp

# All containers, formatted
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}"

# Check disk usage by container
docker system df -v
```

## Network Troubleshooting

```bash
# Show container network info
docker network inspect bridge

# List networks and which containers are on them
docker network ls
docker network inspect <network-name>

# Test DNS resolution from inside a container
docker exec myapp nslookup postgres
docker exec myapp ping -c 3 postgres

# Test connectivity to another container
docker exec myapp curl -s http://redis:6379 || echo "no HTTP on redis port"
docker exec myapp nc -zv postgres 5432 && echo "port open" || echo "port closed"

# Check if a container can reach the internet
docker exec myapp curl -s https://httpbin.org/ip
```

## Image Debugging

```bash
# Examine image layers
docker history myapp:latest
docker history --no-trunc myapp:latest

# Run a shell in a stopped or crashing image
docker run --rm -it --entrypoint sh myapp:latest

# Override entrypoint for debugging
docker run --rm -it --entrypoint bash myapp:latest

# Inspect what changed in a container vs its base image
docker diff myapp
```

## Common Crash Patterns

```bash
# Container keeps restarting? Get exit code
docker inspect myapp --format='{{.State.ExitCode}}'
# Exit 1 = application error
# Exit 137 = OOM kill (out of memory)
# Exit 139 = segfault

# Check restart count
docker inspect myapp --format='{{.RestartCount}}'

# View last log lines after a crash
docker logs --tail=50 myapp

# Run with more verbose logging
docker run --rm -e LOG_LEVEL=debug myapp:latest
```

## Filesystem Debugging

```bash
# Copy a file from a container for inspection
docker cp myapp:/app/config.yaml ./config-from-container.yaml

# Copy a file into a container
docker cp ./fix.yaml myapp:/app/config.yaml

# Check disk usage inside a container
docker exec myapp df -h
docker exec myapp du -sh /app/*
```

## Container Events

```bash
# Monitor Docker events (start, stop, die, OOM)
docker events --filter container=myapp

# Events in the last 10 minutes
docker events --since=10m --filter container=myapp
```

## Cleanup After Debugging

```bash
# Remove all stopped containers
docker container prune

# Remove dangling images (untagged)
docker image prune

# Full system cleanup (removes stopped containers, unused images, unused networks)
docker system prune

# Aggressive cleanup including volumes (DESTROYS DATA)
docker system prune -a --volumes
```

## Interpreting Common Errors

| Error | Likely Cause | Fix |
|-------|-------------|-----|
| `No such container` | Container not running or wrong name | `docker ps -a` to find it |
| `Permission denied` | Non-root user lacks access | Use `--user root` or fix file permissions |
| `exec: not found` | Shell not in image | Try `sh` instead of `bash` |
| `Error response from daemon: OOMKilled` | Container ran out of memory | Increase `--memory` limit |
| `connection refused` | Service not listening on expected port | Check service startup logs |
| `no route to host` | Network misconfiguration | Verify container network assignment |
