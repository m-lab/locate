# Heartbeat Service

The Heartbeat Service is a sidecar service that performs health checks
for experiment instances running in the same Kubernetes pod and reports
them to the Locate API.

On start, it establishes a WebSocket connection with the Locate Service
and sends an initial registration message. Subsequently, it performs
periodic instance health checks and sends the results to the Locate API.

## Local Development

To run the service locally, build the package and pass a test URL
in the `-heartbeat-url` flag.

```sh
$ go build
$ ./heartbeat \
    -heartbeat-url=ws://locate-dot-mlab-sandbox.appspot.com/v2/platform/heartbeat?key=${API_KEY}
```
