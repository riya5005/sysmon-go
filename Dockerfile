# --- Build stage ---
# Compile a static binary using the full Go toolchain image.
FROM golang:1.22-alpine AS build

WORKDIR /app
COPY go.mod ./
COPY *.go ./

# CGO_ENABLED=0 produces a statically linked binary with no libc
# dependency, which is what lets us copy it into a "scratch" image
# with nothing else in it.
RUN CGO_ENABLED=0 GOOS=linux go build -o sysmon .

# --- Final stage ---
# scratch is a completely empty base image (0 bytes). This keeps the
# final image tiny (a few MB, vs ~300MB+ for the build image) and
# reduces attack surface since there's no shell, no package manager,
# nothing but our binary.
FROM scratch

COPY --from=build /app/sysmon /sysmon

EXPOSE 8080

ENTRYPOINT ["/sysmon"]
