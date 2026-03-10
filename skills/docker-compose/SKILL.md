---
name: docker-compose
description: "Manage multi-container applications with docker compose: up/down/logs/exec, networking, volumes, environment overrides. Trigger: when using docker compose, starting services with compose, docker compose logs, compose networking, compose volumes"
version: 1
argument-hint: "[up|down|logs|exec|ps|restart] [service]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Docker Compose

You are now operating in Docker Compose management mode.

## Start and Stop Services

```bash
# Start all services (detached)
docker compose up -d

# Start specific services
docker compose up -d postgres redis

# Start with a rebuild (after code changes)
docker compose up -d --build

# Start and follow logs immediately
docker compose up

# Stop all services (keep containers)
docker compose stop

# Stop and remove containers, networks
docker compose down

# Stop, remove containers, networks, AND volumes (destroys data)
docker compose down -v
```

## Service Status and Logs

```bash
# List running services and their status
docker compose ps

# Show all services including stopped
docker compose ps -a

# Tail logs for all services
docker compose logs -f

# Tail logs for a specific service
docker compose logs -f postgres

# Last 100 lines of logs
docker compose logs --tail=100 api

# Timestamps in logs
docker compose logs -f --timestamps api
```

## Execute Commands in Running Containers

```bash
# Open an interactive shell
docker compose exec postgres bash
docker compose exec api sh

# Run a one-off command
docker compose exec postgres psql -U postgres -d myapp

# Run as a specific user
docker compose exec --user root api sh

# Run a migration
docker compose exec api ./migrate up
```

## Run One-Off Commands (New Container)

```bash
# Run a command in a new container and remove it
docker compose run --rm api go test ./...

# Run without service dependencies
docker compose run --no-deps --rm api sh

# Override environment variables
docker compose run --rm -e DEBUG=true api ./scripts/setup.sh
```

## Scaling and Restarting

```bash
# Restart all services
docker compose restart

# Restart a specific service
docker compose restart api

# Scale a service (multiple instances)
docker compose up -d --scale worker=3
```

## Networking

Compose creates a default network. Services can reach each other by service name:

```yaml
# docker-compose.yml example
services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://postgres:dev@postgres:5432/myapp
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: dev
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

```bash
# Inspect the compose network
docker network inspect <project>_default

# List all networks
docker network ls
```

## Environment Overrides

```bash
# Use .env file (auto-loaded from compose file location)
# .env:
# POSTGRES_PASSWORD=secret
# API_PORT=8080

# Override with a custom env file
docker compose --env-file .env.local up -d

# Use multiple compose files (override pattern)
docker compose -f docker-compose.yml -f docker-compose.override.yml up -d
```

## Volumes and Data

```bash
# List volumes for the project
docker compose volume ls 2>/dev/null || docker volume ls --filter label=com.docker.compose.project=$(basename $PWD)

# Copy files from a service container
docker compose cp postgres:/var/lib/postgresql/data/pg_hba.conf ./pg_hba.conf

# Copy files into a service container
docker compose cp ./config.yaml api:/app/config.yaml
```

## Build and Update

```bash
# Rebuild images for changed services
docker compose build

# Rebuild a specific service image
docker compose build api

# Pull latest images from registry
docker compose pull

# Force recreate containers (picks up config changes)
docker compose up -d --force-recreate
```

## Common docker-compose.yml Patterns

```yaml
# Production-ready pattern with healthchecks and resource limits
services:
  api:
    image: myapp:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=${DATABASE_URL}
    depends_on:
      postgres:
        condition: service_healthy
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
```

## Troubleshooting

```bash
# Check compose config (validates YAML and variable substitution)
docker compose config

# View events from services
docker compose events

# Show resource usage
docker compose top

# Inspect a specific service container
docker inspect $(docker compose ps -q api)
```
