# Running Command with Monitoring Tokens

The locate service issues access tokens based on the client use-case.

Clients using the `/v2/query` path receive access tokens for a specific
service. Clients using the `/v2/monitoring` path receive access tokens
specifically for monitoring.

These special purpose, monitoring access tokens allow the target server to
identify monitoring requests and optionally handle them differently. For
example, the target server could allow the request when another client would
be rejected, or monitoring metrics could be updated differently.

Because target servers treat monitoring access tokens differently than query
access tokens, additional authorization is required before issuing monitoring
access tokens. This authorization is provided using access tokens!

The locate service uses a private signing key that issues access tokens. A
privileged, end to end monitoring client will also have the ability to create
access tokens to request monitoring access tokens from the locate service.

Basic sequence diagram for a /v2/monitoring request:

```txt
Get access token: monitoring-token <------> locate/v2/monitoring
Use access token: e2e-client       -------> service
```

For our end to end monitoring, we will use the `monitoring-token` command to
get an access token from the locate service, pass a service URL to a command
through an environment variable. For example:

```sh
export LOCATE_URL=https://locate-dot-mlab-sandbox.appspot.com/v2/monitoring/
export MONITORING_SIGNER_KEY=/path/to/key.json

monitoring-token \
    -machine=${MACHINE} \
    -service=ndt/ndt5 -- \
    ndt5-client -throttle=131072 -protocol=ndt5+wss
```

## Debug

By default, `monitoring-token` does not report any extra output and the
subcommand output is discarded. To report diagnostic information from
`monitoring-token` and print the subcommand's stdout and stderr, use the
`-logx.debug=true` flag.

```sh
monitoring-token -logx.debug=true \
    ...
```
