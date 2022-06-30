# locate

[![Version](https://img.shields.io/github/tag/m-lab/locate.svg)](https://github.com/m-lab/locate/releases) [![Build Status](https://travis-ci.com/m-lab/locate.svg?branch=master)](https://travis-ci.com/m-lab/locate) [![Coverage Status](https://coveralls.io/repos/m-lab/locate/badge.svg?branch=master)](https://coveralls.io/github/m-lab/locate?branch=master) [![GoDoc](https://godoc.org/github.com/m-lab/locate?status.svg)](https://godoc.org/github.com/m-lab/locate) [![Go Report Card](https://goreportcard.com/badge/github.com/m-lab/locate)](https://goreportcard.com/report/github.com/m-lab/locate)

M-Lab Locate Service, a load balancer providing consistent “expected
measurement quality” using access control.

## Local Development

### Secret Manager
Typically the locate service will run within a GCP environment, either AppEngine
or GKE. In these cases, the locate service reads signer and verifier keys from
GCP's secret manager. This dependency is not needed for local development.

Create JSON Web Keys for local development:

```sh
jwk-keygen --use=sig --alg=EdDSA --kid=localdev_20220415
```

You may reuse the same key for signer and verifier, or create multiple keys.

```sh
./locate \
    -key-source=local \
    -signer-secret-name ./jwk_sig_EdDSA_localdev_20220415 \
    -verify-secret-name ./jwk_sig_EdDSA_localdev_20220415.pub
```

Now you may visit localhost:8080 in your browser to see a response generating
`access_token`s using these keys. Of course, the URLs returns will not be valid
for the public platform.

### Redis
A `docker-compose` configuration file is provided to run a local instance of the
locate service along with Redis.

In the root directory of the "locate" project, start a local build using default
arguments and precomputed JSON Web Keys.
```sh
docker-compose up
```

To connect with the local redis instance, run the `cmd/heartbeat` command or use
the `redis-cli` command from the terminal.
