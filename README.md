# Stage

Web server for serving static SPAs with environment-based transformations. Build once, deploy to multiple environments (QA, staging, prod) with different configs.

## Quick Start

Use as a base image in your Dockerfile:

```dockerfile
# Build your static app
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Use stage to serve it
FROM cloudbeesdemo/stage:latest
COPY --from=builder /app/dist /app/assets
```

In your static files, use placeholders:

```javascript
// src/config.js
const sdkKey = '__FF_SDK_KEY__';
const apiEndpoint = '__API_ENDPOINT__';
```

Set environment variables when deploying:

```bash
STAGE_FF_SDK_KEY=prod-abc-123
STAGE_API_ENDPOINT=https://api.production.com
```

Stage transforms files at startup and caches them. Placeholders like `__FF_SDK_KEY__` get replaced with values from `STAGE_FF_SDK_KEY`.

## Configuration

### Server Settings

- `PORT` - Server port (default: `8080`)
- `HOST` - Server host (default: `0.0.0.0`)
- `ASSET_DIR` - Directory with static assets (default: `/app/assets`)
- `LOG_LEVEL` - `DEBUG`, `INFO`, `WARN`, `ERROR` (default: `INFO`)
- `FM_KEY` - Feature Management SDK key (optional, used for future FM visualization features and automatically replaces `__FM_KEY__` placeholders)

### Transformations

Any env var prefixed with `STAGE_` becomes a transformation:

- `STAGE_<NAME>=value` → replaces `__<NAME>__` in your files
- Case sensitive
- Only transforms text files (HTML, JS, CSS, JSON, etc.)
- **Special case**: `FM_KEY` (without `STAGE_` prefix) automatically replaces `__FM_KEY__` placeholders

### Prometheus Mock Server

Stage includes a built-in mock Prometheus server for testing continuous verification workflows. 

**Configuration:**
- `PROMETHEUS_ENABLED` - Enable mock server (default: `true`)
- `STAGE_PROMETHEUS_SCENARIO` - Initial scenario: `healthy`, `high-errors`, `latency-spike`, `gradual-degradation` (default: `healthy`)

**Endpoints:**
- `/api/v1/query` - Prometheus Query API (GET/POST)
- `/metrics` - Prometheus text exposition format
- `/prometheus/admin` - Web control panel for switching scenarios
- `/prometheus/api/scenario` - REST API for scenario control

**Scenarios:**
- **healthy** - Minimal errors (~0.1%), low latency (~100ms)
- **high-errors** - Progressive error increase: 5% → 25% over 5 minutes
- **latency-spike** - Latency degradation: 150ms → 2000ms over 3 minutes
- **gradual-degradation** - Both errors and latency worsen over 10 minutes

**Supported Queries:**
- `rate(http_requests_errors_total[5m])` - Error rate
- `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))` - P99 latency
- `up` - Service uptime

## Examples

### Docker Compose

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - STAGE_FF_SDK_KEY=dev-key-123
      - STAGE_API_ENDPOINT=http://localhost:3000
```

### Kubernetes

```yaml
env:
  - name: STAGE_FF_SDK_KEY
    value: {{ .Values.featureFlags.sdkKey }}
  - name: STAGE_API_ENDPOINT
    value: {{ .Values.api.endpoint }}
```

### Direct Docker Run

```bash
docker run -d \
  -p 8080:8080 \
  -e STAGE_FF_SDK_KEY=my-key \
  -e STAGE_API_ENDPOINT=https://api.example.com \
  your-app:latest
```

## Health Check

```bash
curl http://localhost:8080/health
```

Returns cache stats (files cached, hits, misses, memory usage).

## Troubleshooting

**Placeholders not replaced?**
- Check env vars have `STAGE_` prefix
- Verify placeholders use `__NAME__` format (double underscores)
- Set `LOG_LEVEL=DEBUG` to see what's being transformed

**404 errors?**
- Check `ASSET_DIR` is correct (default: `/app/assets`)
- Verify files were copied in Dockerfile

**SPA routing not working?**
- Should work automatically for paths without file extensions
- API routes (`/api/*`) intentionally return 404

## License

MIT
