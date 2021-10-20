# Management of Locate Service JWKs

The Locate Service (LS) loads multiple JSON Web Keys (JWK) for signing and
verifying request tokens. To safely configure and deploy private signer and
public verifier keys in AppEngine, we rely on Google Secret Manager (SM).

To bootstrap keys and secrets in a project, where none currently exist, an
operator should use the `management/create_jwt_keys_and_secrets.sh` script. The
script takes two arguments, GCP project ID and "keyID". It is recommended that
the keyID be in the following date format _YYMMDD_. For example:

```sh
cd management
./create_jwt_keys_and_secrets.sh mlab-sandbox $(date +%Y%m%d)
```

This script will generate two JWT key pairs, one for the LS and the other for
platform monitoring. The keys will be located in the directory where you ran the
script and will look like:

```sh
ls -1 jwk*
jwk_sig_EdDSA_locate_<keyID>
jwk_sig_EdDSA_locate_<keyID>.pub
jwk_sig_EdDSA_monitoring_<keyID>
jwk_sig_EdDSA_monitoring_<keyID>.pub
```

The script will automatically load the LS private signer key and monitoring
public verify key into SM. These keys will be read from SM by the LS at
runtime. However, it is the operator's responsibility to deploy the monitoring
private signer key and LS public verifier key. Instructions on how to deploy
these keys can be found in sections later in this document.

## Key rotation

Periodically, the private signer keys should be rotated. To ensure continuity of
verification, LS and clients (in particular the ndt-server) MUST possess both
the current _and_ next verifier keys.

To create new key pairs for both the LS and monitoring, run the same script
(`./management/create_jwt_keys_and_secrets.sh`) as before. If a SM secret
for the LS private signer key already exists, you will be prompted to add a new
version of the secret. The same goes for the monitoring verify key. The new
LS private signer key version will _not_ be active or used by the LS without
further action, described below. However, the new monitoring public verify key
will be active right away. There is no harm in activating new verify keys right
away, as long as the older ones remains enabled too.

### Locate Service

The LS reads the private signer key directly from the SM at runtime. When the
script creates a new version of a secret, it will disable the new private signer
key version. This will reduce the chance that the LS could unintentionally load
the new private signer key when it starts up. Before enabling the key, you
*must* ensure that the associated LS public verifier key has been distributed to
all services that need to verify LS keys. Currently, this list includes:

* ndt
* ndt-canary
* access-envelope

To deploy the LS public verifier key, upload the public key to Google Cloud
Storage (GCS). The public verifier key will be located in the directory where
you ran the script, and will have a name like
"jwk_sig_EdDSA_locate_\<keyid\>.pub". Upload it to the following bucket,
replacing "\<project\>" with the GCP project in which you are working. For
example:

```sh
gsutil cp jwk_sig_EdDSA_locate_20211020.pub gs://k8s-support-mlab-sandbox/locate/
```

The Google Cloud Build (GCB) deployment scripts in the k8s-support repo will
read the LS public verifier key from that bucket. Once the key has been
uploaded, you will need to add an *additional* `-token.verify-key` flag to all
of the services that use it:

* [ndt.jsonnet](https://github.com/m-lab/k8s-support/blob/master/k8s/daemonsets/experiments/ndt.jsonnet)
* [ndt-canary.jsonnet](https://github.com/m-lab/k8s-support/blob/master/k8s/daemonsets/experiments/ndt-canary.jsonnet)
* [wehe.jsonnet](https://github.com/m-lab/k8s-support/blob/master/k8s/daemonsets/experiments/wehe.jsonnet)

**NOTE**: do not remove or overwrite the existing `-token-verify-key` flags. The
flag can be specified multiple times, and we are adding a new one.  The value of
the new flag should be the filename for the LS public verifier key you uploaded
to GCS.

Once the k8s-support GCB build has completed after a push to the repo (or a
tag, for production), verify that the Kubernetes Secret named
`locate-verify-keys` in the platform cluster contains the file you uploaded to
GCS:

```sh
kubectl --context <project> describe secret locate-verify-keys
```

Modifying the DaemonSet pod specs (with the additional -token-verify-key flag)
will cause rolling updates of the DaemonSets, deploying the new LS public
verifier key. Once you are 100% sure that all services using the key have rolled
out the change, and you have verified that all services have loaded the
additional public key, then, and only then, can you enable the new signer key in
the Secret Manager. You can enable it with a command like:

```sh
gcloud secrets versions list locate-service-signer-key --project <project>
# Note the version number of the new, disabled version
gcloud secrets versions enable <version> --secret locate-service-verify-key --project <project>
```

You are still not done. The LS is configured to use the _oldest_ enabled version
of a secret. To promote your new private signer key, you will also need to
disable all previous versions of the secret, such that your new private signer
key version is the _oldest_ enabled version. In practice, this will generally
mean disabling only the previous enabled version. Using commands similar to the
ones you used above to enable your new private signer key version, disable all
older versions of the secret older than yours.

LS instances will start using the new private signer key when they are
restarted. You can use the `cbctl` command to trigger a Cloud Build of the LS,
which will redeploy the service, causing all instances to be restarted. For
example:

```sh
go get github.com/m-lab/gcp-config/cmd/cbctl
cbctl -repo locate -trigger_name push-m-lab-locate-trigger -project mlab-staging
```

### Monitoring

The script-exporter pod in the prometheus-federation cluster uses the monitoring
private signer key for monitoring purposes. It signs a token using the private
key, and the LS verifies it using the corresponding public key. To deploy the
new private key, update the MONITORING_SIGNER_KEY evironment variable in the
Travis-CI settings for the prometheus-support repo:

[https://travis-ci.com/github/m-lab/prometheus-support/settings](https://travis-ci.com/github/m-lab/prometheus-support/settings)

You will need to delete the existing MONITORING_SIGNER_KEY environment variable
and recreate it using the value found in the file "jwk_sig_EdDSA_monitoring_\<keyid\>"
after the script is run.

The next time the prometheus-support Travis-CI build runs for a project, the new
key from that environment variable will start to be used.

**WARNING**: The monitoring public verifier key should have uploaded
automatically to the SM when you ran the
`./management/create_jwt_keys_and_secrets.sh` script. However, LS instances will
not load the new verifier key until they are restarted. DO NOT deploy the new
monitoring private signer key, until you are sure that all LS instances in
AppEngine have restarted and picked up the new verify public key which
corresponds to the private key, else script-exporter monitoring requests will
fail. You can trigger a redeployment of the LS using `cbctl` as described
earlier in this document.
