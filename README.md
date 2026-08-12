# sysmon — Linux System Monitor (Go + Docker)

A small HTTP API, written in Go, that reads live system stats directly
from the Linux `/proc` filesystem — no external monitoring libraries —
and ships as a minimal multi-stage Docker image.

## What it does

| Endpoint            | Source              | Returns                                  |
|----------------------|---------------------|-------------------------------------------|
| `GET /health`         | —                   | `{"status":"ok"}`                         |
| `GET /stats/cpu`      | `/proc/stat`        | Live CPU usage % (via two-snapshot delta) |
| `GET /stats/memory`   | `/proc/meminfo`     | Total/free/available/used memory          |
| `GET /stats/load`     | `/proc/loadavg`     | 1/5/15-minute load averages               |
| `GET /stats/processes`| `/proc/<pid>/*`     | List of running processes, state, RSS     |
| `GET /metrics`        | (derived)           | Prometheus-format plaintext metrics       |

## Why it's built this way (for your own understanding / interview prep)

- **CPU % isn't a single read.** `/proc/stat` only exposes cumulative
  jiffie counters since boot. To get a live percentage, `cpuHandler`
  takes two snapshots 200ms apart and computes the delta — this is
  the same technique `top`/`htop` use internally.
- **No third-party libraries.** Everything uses Go's standard library
  (`net/http`, `os`, `bufio`, `encoding/json`). This was a deliberate
  choice so the project actually demonstrates understanding of Linux
  internals and Go fundamentals, rather than wrapping someone else's
  package.
- **Multi-stage Docker build.** The build stage uses the full
  `golang:1.22-alpine` image to compile a statically linked binary
  (`CGO_ENABLED=0`). The final image is built `FROM scratch` — a
  completely empty base — and contains nothing but that one binary.
  This is the detail worth explaining in an interview: it shows you
  understand *why* containers are built this way (small attack
  surface, small image size, fast pulls), not just that `docker build`
  exists.

## Prerequisites

- Go 1.22+ — https://go.dev/dl/ (only needed to run outside Docker)
- Docker — https://docs.docker.com/get-docker/

## Run it locally with Go (no Docker)

```bash
cd sysmon-go
go build -o sysmon .
./sysmon
```

You should see:
```
2026/08/12 10:28:50 sysmon listening on :8080
```

In another terminal:
```bash
curl localhost:8080/health
curl localhost:8080/stats/memory
curl localhost:8080/stats/cpu
curl localhost:8080/stats/load
curl localhost:8080/stats/processes
curl localhost:8080/metrics
```

## Run it with Docker

Build the image:
```bash
cd sysmon-go
docker build -t sysmon .
```

Run the container:
```bash
docker run -p 8080:8080 sysmon
```

Test it exactly the same way as above — the API is identical:
```bash
curl localhost:8080/stats/memory
```

**Note on containerized stats:** by default, the container only sees
its *own* `/proc` (its own cgroup-limited view), not the host
machine's. That's actually expected Docker behavior, not a bug — but
if you want the container to report real host-level stats, run it
with the host's proc mounted in:
```bash
docker run -p 8080:8080 -v /proc:/host/proc:ro sysmon
```
(This would require a small code change to read from `/host/proc`
instead of `/proc` — a good "next step" to mention if asked about it.)

## Check the image size (worth showing off)

```bash
docker images sysmon
```
A `scratch`-based Go binary image is typically just a few MB, versus
several hundred MB for an unoptimized single-stage build — this
comparison is a good thing to have ready to explain in an interview.

## Project structure

```
sysmon-go/
├── main.go       # HTTP server + route handlers
├── proc.go       # All /proc-reading logic (CPU, memory, load, processes)
├── go.mod        # Go module definition (no external dependencies)
├── Dockerfile    # Multi-stage build → scratch-based final image
└── README.md
```

## What I learned (fill this in with your own words before adding to GitHub)

- How Linux exposes live system state through `/proc` as plain text
- Why CPU usage requires a delta between two snapshots, not a single read
- How to structure a small REST API in Go using only the standard library
- Why multi-stage Docker builds matter for image size and security
- (If you do the host-proc mount) How container process/filesystem
  isolation actually works under the hood

## Possible next steps (optional, for a stronger resume story)

- Add a Kubernetes/OpenShift Deployment YAML and test on Minikube
- Add goroutine-based background polling instead of on-request reads
- Add basic unit tests for the `/proc` parsing functions
- Push the image to Docker Hub or a container registry
