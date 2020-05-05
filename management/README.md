# Management of Locate Service JWK

The locate service loads multiple JSON Web Keys (JWK) for signing and
verifying signatures. To safely configure and deploy a private key in
AppEngine, we rely on Google Key Management Service (KMS).

In advance, an operator should create and encrypt a private JWK signing key
using KMS via `create_encrypted_signer_key.sh`.

The script `create_encrypted_signer_key.sh` produces the stanza that should
be added to the app.yaml configuration. Because the KMS encrypion is
per-project, each deployment will need it's own version of the encrypted key.
And, because the key is encrypted, it can safely be added to a public
repository.

At start up, the locate service uses KMS to decrypt the signing key and then
uses the resulting key to create a new token Signer that issues
`access_tokens`. In turn, these access tokens are verified by target servers.

It is the operator's responsibility to deploy the public JWK verifier key to
all target servers.

## Key Rotation

Periodically, the signing key should be rotated. To ensure continuity of
verification, clients must posess both the current and next verifier key. A
successful key rotation would follow these steps:

1. Create a new key pair.

    `./create_encrypted_signer_key.sh ${PROJECT} $( date +%Y%m%d )`

2. Distribute the new verifier key to all clients of the new signer key.

    * Copy verifier key to k8s-support for ndt-server and access envelope.
    * Create a new production release including the new key.

3. Promote the new signer key.

    * Copy signer key to locate app.yaml configs
    * Create a new production release including the new key.

Follow similar steps for rotating the monitoring key pairs.

## TODO

* Describe JWK management for
  [m-lab/k8s-support](http://github.com/m-lab/k8s-support).
