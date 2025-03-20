# glowroot-exporter

A Prometheus exporter for Glowroot APM metrics. This exporter collects transaction and error metrics from Glowroot and exposes them in Prometheus format.

## Features

- Collects error statistics and transaction metrics from Glowroot
- Supports agent rollups and child agents
- Configurable metrics update interval
- Prometheus-compatible metrics endpoint

## Configuration

Create a `config.yaml` file with the following structure:

```yaml
server:
  glowroot_url: "http://glowroot-server:4000"
  exporter_port: 9101
  glowroot_time_interval_minutes: 5
  metrics_update_interval_seconds: 60
```

## Building

### Go Build

```bash
go build -o glowroot-exporter main.go
```

### Docker Build

```bash
docker build -t glowroot-exporter .
```

## Running

### Direct Execution

```bash
./glowroot-exporter
```

### Docker Run

```bash
docker run -p 9101:9101 -v $(pwd)/config.yaml:/app/config.yaml ufkunkaraman/glowroot-exporter:latest
```

## Metrics

The exporter exposes the following metrics at `/metrics`:

- `glowroot_agent_rollup`: Information about Glowroot agent rollups
- `glowroot_agent_rollup_id`: Information about Glowroot agent IDs
- `glowroot_agent_rollup_id_error_total_count`: Total error count
- `glowroot_agent_rollup_id_error_transaction_total_count`: Total transaction count
- `glowroot_agent_rollup_id_error`: Error count per transaction
- `glowroot_agent_rollup_id_slow_trace_transaction_total_count`: Slow trace transaction count
- `glowroot_agent_rollup_id_slow_trace_transaction`: Transaction count per slow trace

## Docker Compose Example

```yaml
version: '3'
services:
  glowroot-exporter:
    image: ufkunkaraman/glowroot-exporter:latest
    ports:
      - "9101:9101"
    volumes:
      - ./config.yaml:/app/config.yaml
    restart: unless-stopped
```

## Prerequisites

- Go 1.21 or higher
- Access to a Glowroot APM server
- Docker (optional, for containerized deployment)

## Grafana Dashboard 

- Import Glowroot Exporter Dashboard.json or import  [Grafana](https://grafana.com/grafana/dashboards/23125)

### Global Overview
![Global Overview](media/global-overview.png)

### Resource Overview
![Resource Overview](media/resource-overview.png)

### Error Overview
![Error Total](media/error-overview.png)

