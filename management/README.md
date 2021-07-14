# Management of Locate Service JWK

The locate service loads multiple JSON Web Keys (JWK) for signing and
verifying signatures. To safely configure and deploy a private key in
AppEngine, we rely on Google Secret Manager (GSM).

In advance, an operator should create private JWK signing and verify keys
using the `management/create_jwt_keys_and_secrets.sh` script.

This script will generate the JWT keys and then load them into GSM for a given
project. These secrets will be read by the locate service at runtime.

It is the operator's responsibility to deploy the public JWK verifier key to
all target servers.

## TODO

* Implement and document key rotation. Periodically, the signing key should be
  rotated. To ensure continuity of verification, clients must posess both the
  current and next verifier key.

* Describe JWK management for
  [m-lab/k8s-support](http://github.com/m-lab/k8s-support).
