# sysmon-go

A small system monitoring API written in Go. It reads live CPU, memory, load average, and process data directly from the Linux `/proc` filesystem and exposes them as JSON over HTTP. Also ships with a multi-stage Dockerfile for a minimal container image.

No external libraries — just Go's standard library and raw `/proc` parsing.

## Endpoints

- `GET /health` — basic health check
- `GET /stats/cpu` — live CPU usage %
- `GET /stats/memory` — memory usage stats
- `GET /stats/load` — 1/5/15 min load average
- `GET /stats/processes` — list of running processes
- `GET /metrics` — Prometheus-style metrics output

## Run locally

```bash
go build -o sysmon .
./sysmon
```

Then in another terminal:
```bash
curl localhost:8080/stats/memory
```

## Run with Docker

```bash
docker build -t sysmon .
docker run -p 8080:8080 sysmon
```

## Notes

- CPU usage is calculated by taking two `/proc/stat` snapshots 200ms apart, since Linux only exposes cumulative counters, not a live percentage.
- The Docker image uses a multi-stage build — compiles in `golang:alpine`, then copies just the binary into a `scratch` base — so the final image is only a few MB.

## Stack

Go · Docker · Linux (`/proc`)
