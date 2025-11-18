# Setup for local development

## Redis & Locate Servers

```sh
docker-compose build
docker-compose up
```

## Heartbeats

Use real machine names in order to reuse the public 'registration.json'. While
the names are real, the statuses are not.

You may need to run multiple heartbeat instances to simulate multiple machines.
Each heartbeat service will need a unique prometheusx.listen-address to prevent
conflicts.

```sh
./heartbeat -experiment=ndt \
  -heartbeat-url ws://localhost:8080/v2/platform/heartbeat \
  -hostname=mlab1-lga0t.mlab-sandbox.measurement-lab.org \
  -registration-url=https://siteinfo.mlab-oti.measurementlab.net/v2/sites/registration.json \
  -services 'ndt/ndt7=ws:///ndt/v7/download,ws:///ndt/v7/upload,wss:///ndt/v7/download,wss:///ndt/v7/upload' \
  -prometheusx.listen-address=:9991
```

You may also construct your own registration.json file and provide this to the heartbeat:

```txt
  -registration-url=file://$PWD/registration.json \
```

## Fake service ports

The heartbeat scans the local service port to determine if the service is
healthy. Since we're faking everything, all we need to do is listen on the ports
that the heartbeat is scanning. For example:

```sh
ncat --broker --listen -p 80 &
ncat --broker --listen -p 443 &
```

The `--broker` flag will keep the process listening indefinitely.

If the heartbeat does not detect a running service, the machine will be reported
as unhealthy, and will not be returned by Locate under normal behavior.

## Local requests

Once everything is running, you can make local requests to verify it is working
as you expect. It will be necessary to provide location information since
the location headers added by App Engine will be missing. For example:

* http://localhost:8080/v2/nearest/ndt/ndt7?region=US-NJ

## Pitfalls

Just as in production, the ready handler should return "ok". If it does not,
there is either a problem communicating with your redis instance, or there may
be corrupt or junk data in Redis that is confusing Locate. This can happen if
you run locate and redis separately outside of docker-compose.

```sh
curl --dump-header - localhost:8080/v2/ready
```

## Query Redis

```sh
$ redis-cli --scan --pattern "*"
"mlab1-lga0t.mlab-sandbox.measurement-lab.org"
$ redis-cli --raw HGETALL mlab1-lga0t.mlab-sandbox.measurement-lab.org
Health
{"Score":1}
Registration
{"City":"New York","CountryCode":"US","ContinentCode":"NA","Experiment":"ndt","Hostname":"mlab1-lga0t.mlab-sandbox.measurement-lab.org","Latitude":40.7667,"Longitude":-73.8667,"Machine":"mlab1","Metro":"lga","Project":"mlab-sandbox","Probability":1,"Site":"lga0t","Type":"physical","Uplink":"10g","Services":{"ndt/ndt7":["ws:///ndt/v7/download","ws:///ndt/v7/upload","wss:///ndt/v7/download","wss:///ndt/v7/upload"]}}
```
