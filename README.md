# Vamp Health Probe

This repository contains helper structs to run Kubernetes health checks in parallel.

## Running unit tests

```bash
make test
```

## Example usage

#### NATS health check
```go
import "github.com/magneticio/vamp-health-probe/pkg/probe" 

func StartHealthCheck() {
	natsConnection := ... // create NATS connection

	natschecker := func() error {
		if !natsConnection.IsConnected() {
			return fmt.Errorf("not connected")
		}
		return nil
	}

	healthStatusProvider := probe.NewHealthStatusProvider(map[string]probe.HealthStatusChecker{
		"NATS": natschecker,
	})
	healthStatusProvider.Start(time.Minute)
	http.HandleFunc("/healthz", healthStatusProvider.Handler)

	go log.Fatal(http.ListenAndServe("0.0.0.0:7770", nil))
}
```

#### Kubernetes livenessProbe
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 7770
  initialDelaySeconds: 10
  timeoutSeconds: 5
  periodSeconds: 60
```