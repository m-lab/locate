# Locate API Usage

## Revision History

| Version  | Date       |  Major Changes  |
|----------|------------|-----------------|
| v2.0.0   | 2020-07-28 | initial version |

## Introduction

The Locate API provides consistent, expected measurement quality for M-Lab
clients. The Locate API is a GCP hosted service that "locates" the best M-Lab
server for a user request. For different use cases, "best" could mean
different things. The sections below provide an overview of the locate API
operations and describe currently supported queries in more detail.

### Locate Servers Near the Client

#### When best means "geographically close"

GCP automatically adds client latitude and longitude to incoming [HTTP
request headers][headers]. The locate API uses this location to search for
nearby M-Lab servers that satisfy the client query.

[headers]: https://cloud.google.com/load-balancing/docs/user-defined-request-headers#how_user-defined_request_headers_work

The locate API also considers:

* is the target server online? (required)
* is the client request frequency within by our [acceptable use policy][aup]?
  (required)

> PLANNED(v2): the locate API will bias results to be in the same country
as the client.

The locate API returns up to four results for the requested measurement
service. The locate API may return fewer results when too few servers are
healthy in a geographic region. The locate API may return an error when no
servers are available.

> PLANNED(v2): in all cases above, the locate API will return a recommended
[v2.QueryResult.NextRequest][nextRequest] time and signed URL for the client
to issue the next request. This will make retry logic in clients simpler and
encourages best practices for the API. See the [request priority
hierarchy][priority].

[nextRequest]: https://godoc.org/github.com/m-lab/locate/api/v2#QueryResult
[priority]: https://godoc.org/github.com/m-lab/locate/api/v2
[aup]: https://www.measurementlab.net/aup

#### Query for Geographically Close Servers

The base URL for the locate API to query for geographically close
servers:

    http://locate.measurementlab.net/v2/nearest/

Well formed requests must specify a service name. For example:

* ndt/ndt5 - NDT5 protocol for the NDT measurement service.
* ndt/ndt7 - NDT7 protocol for the NDT measurement service.

> PLANNED(v2): to discover the list of available services, the locate API
will support queries to the base URL. Currently, only the ndt services are
supported.

A complete locate request with service name (e.g. ndt/ndt7) looks like:

    http://locate.measurementlab.net/v2/nearest/ndt/ndt7

A successful response will include a list of results. Each result object
includes the machine name and a map of "urls"; the key is the measurement
service URL template and the value is the complete URL to the service running
on the target machine. The template keys are static so clients can extract
the value.

The complete URL includes the protocol scheme, e.g. `wss`, the machine name,
the resource path (e.g. `/ndt/v7/download`), and request parameters generated
by the locate API (e.g. `access_token=`). The `access_token=` request
parameter is validated by the target service (e.g. ndt-server). An invalid
access token will always be rejected.

    {
      "results": [
        {
          "machine": "mlab3-lga05.mlab-oti.measurement-lab.org",
          "urls": {
            "ws:///ndt/v7/download": "ws://ndt-mlab3-lga05.mlab-oti.measurement-lab.org/ndt/v7/download?access_token={{TOKEN}}",
            "ws:///ndt/v7/upload": "ws://ndt-mlab3-lga05.mlab-oti.measurement-lab.org/ndt/v7/upload?access_token={{TOKEN}}",
            "wss:///ndt/v7/download": "wss://ndt-mlab3-lga05.mlab-oti.measurement-lab.org/ndt/v7/download?access_token={{TOKEN}}",
            "wss:///ndt/v7/upload": "wss://ndt-mlab3-lga05.mlab-oti.measurement-lab.org/ndt/v7/upload?access_token={{TOKEN}}",
          }
        },
        ...
      ]
    }

> PLANNED(v2): to associate multiple measurements with the same session (e.g.
upload and download), the locate API will add additional request
parameters for `session=` with a random id that the target server saves with
the measurement results.

Once a client connects to a target service using a given URL, the target
server may accept or reject the connection based on local conditions (e.g.
sufficient network capacity). The goal is to preserve the expected
measurement quality for every client. Meeting this goal means that
occassionally some clients may be temporarily prevented from running a
measurement on a particular machine.

Therefore, the client should be resilient to transient rejections by continuing
with the next returned result. Clients must also gracefully handle the case
where all target servers reject the client request. This could happen when
the platform is under extreme load. Clients should report the outage to
users.

> PLANNED(v2): the Locate API `NextRequest` will provide clients with a wait
time before trying again.
