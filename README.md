# dell_md_exporter

`dell_md_exporter` is a Prometheus exporter written in Go for a local Dell MD storage array. It executes the local `SMcli` binary periodically, caches the most recent successful snapshot in memory, and serves metrics from that snapshot so Prometheus scrapes do not trigger collection work.

## Features

- polls `SMcli localhost` on a configurable interval
- parses the Dell CSV performance export while filtering `Expansion Enclosure` rows
- exposes per-device storage metrics and exporter health metrics
- supports optional TLS/basic auth through `--web.config.file` and `exporter-toolkit`
- includes a build Dockerfile and a `systemd` unit

## Build

Local Go toolchain:

```bash
go build -buildvcs=false -o dell-md-exporter ./cmd/dell_md_exporter
```

Static build for older hosts such as CentOS 7:

```bash
./scripts/build-static.sh
```

This produces a fully static Linux binary with `CGO_ENABLED=0`, so it does not depend on the target host glibc version.

If you prefer the raw Docker command:

```bash
docker run --rm \
  -v "$PWD:/workspace" \
  -w /workspace \
  --entrypoint /bin/sh \
  golang:1.25 \
  -c 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -buildvcs=false -o /workspace/dell-md-exporter ./cmd/dell_md_exporter'
```

Container image build:

```bash
docker build -t dell-md-exporter-build .
```

Note: a plain `go build` can produce a dynamically linked binary depending on how Go is installed on the build machine. On older distributions, prefer the static build path above.

## GitHub Actions

Pushes and pull requests run `go test ./...` automatically in GitHub Actions.

To publish a release binary:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Pushing a `v*` tag creates or updates a GitHub Release and uploads the static asset `dell-md-exporter-linux-amd64`.

## Run

```bash
./dell-md-exporter \
  --web.listen-address=:9904 \
  --collector.interval=60s \
  --collector.timeout=30s \
  --smcli.path=/opt/dell/mdstoragesoftware/mdstoragemanager/client/SMcli \
  --smcli.output-file=/tmp/dell_md_exporter_stats.csv
```

Important flags:

- `--web.listen-address`: HTTP listen address, default `:9904`
- `--web.telemetry-path`: metrics path, default `/metrics`
- `--web.config.file`: optional exporter-toolkit web config for TLS/basic auth
- `--smcli.path`: path to Dell `SMcli`
- `--smcli.output-file`: CSV file written by `SMcli`
- `--collector.interval`: periodic refresh interval
- `--collector.timeout`: timeout for each `SMcli` run, must be lower than the interval

## Metrics

Per-device metrics use the `device` label:

- `dell_md_current_ios_per_second`
- `dell_md_current_io_latency_seconds`
- `dell_md_current_throughput_megabytes_per_second`
- `dell_md_primary_write_cache_hit_ratio`
- `dell_md_read_ratio`
- `dell_md_primary_read_cache_hit_ratio`
- `dell_md_total_ios`

Exporter health metrics:

- `dell_md_exporter_last_refresh_success`
- `dell_md_exporter_last_refresh_timestamp_seconds`
- `dell_md_exporter_last_refresh_duration_seconds`
- `dell_md_exporter_last_refresh_error`
- `dell_md_exporter_snapshot_age_seconds`
- `dell_md_exporter_devices`

## exporter-toolkit web config

If you want TLS and/or basic auth, provide a YAML file through `--web.config.file`.

Example:

```yaml
tls_server_config:
  cert_file: /etc/dell-md-exporter/tls.crt
  key_file: /etc/dell-md-exporter/tls.key
basic_auth_users:
  prometheus: $2y$10$replace_with_bcrypt_hash
```

`exporter-toolkit` reads that file on each request, so certificate and password updates are picked up without restarting the exporter.

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: dell_md
    static_configs:
      - targets:
          - storage-host.example.com:9904
```

## systemd

Install the binary and unit:

```bash
install -m 0755 dell-md-exporter /usr/local/bin/dell-md-exporter
install -D -m 0644 packaging/systemd/dell-md-exporter.service /etc/systemd/system/dell-md-exporter.service
systemctl daemon-reload
systemctl enable --now dell-md-exporter.service
```

Optional `/etc/default/dell-md-exporter` example:

```bash
DELL_MD_EXPORTER_ARGS="--web.listen-address=:9904 --collector.interval=60s --collector.timeout=30s"
```
