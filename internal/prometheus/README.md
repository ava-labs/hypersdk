# Prometheus & Grafana Dashboard for HyperSDK

This setup collects and visualizes metrics from HyperSDK benchmarks.

## Prerequisites
- Docker
- MorpheusVM running locally (default address: localhost:9650)

## Quick Start
1. Run the dashboard using script `internal/prometheus/dashboard.sh`
2. Open Grafana at [localhost:3000](http://localhost:3000)
3. Navigate to: Dashboards > Services > "HyperSDK benchmark"

## Adding New Panels
1. Login to Grafana with:
   - Username: admin
   - Password: admin
   - (Skip the password change prompt)
2. Edit an existing panel
3. Click Save - this will show JSON
4. Copy the JSON to: `internal/prometheus/dashboards/hypersdk-bench.json`

5. Restart Grafana to see changes:

```bash
docker rm -f grafana
internal/prometheus/dashboard.sh
```
