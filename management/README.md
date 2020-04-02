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

## TODO

* Describe JWK management for
  [m-lab/k8s-support](http://github.com/m-lab/k8s-support).
