# Management of Locate Service JWK

The locate service loads multiple JSON Web Keys (JWK) for signing and
verifying signatures. To safely configure and deploy a private key in
AppEngine, we rely on Google Secret Manager (GSM).

In advance, an operator should create private JWK signing and verify keys
using the `management/create_jwt_keys_and_secrets.sh` script. The script takes
two arguments, GCP project ID and "key ID". It is recommended that the key ID be
in the following date format _YYMMDD_. For example:

```sh
./create_jwt_keys_and_secrets.sh mlab-sandbox $(date +%Y%m%d)
```

This script will generate the JWT keys and then load them into GSM for a given
project. These secrets will be read by the locate service at runtime.

It is the operator's responsibility to deploy the public JWK verifier key to
all target servers.

## Key rotation

Periodically, the access token signing key should be rotated. To ensure
continuity of verification, clients (in particular the ndt-server) MUST posess
both the current and next verifier key.

To create a new key pair for both the Locate Service (LS) and monitoring, run
the script `./management/create_jwt_keys_and_secrets.sh`. If a Secret Manager
secret for the key already exists, you will be prompted to add a new version of
the secret.

### Locate Service

The LS reads the signer key directly from the Secret Manager upon initialization
at runtime. When the script creates a new version of a secret, it will disable
the new signer key version. This will prevent the LS from using the new signer
key when it starts up. Before enabling the key, you must ensure that the
associated public key has been distributed to all services that need to verify
LS keys. Currently, this list includes: ndt, ndt-canary and the access envelope.

To do this, first, upload the public key to Google Cloud storage. The public key
will be found in the directory where you ran the script, and will have a name
like "jwk_sig_EdDSA_locate_\<keyid\>.pub".  Upload it to this bucket:

```sh
gs://k8s-support-\<project\>/locate
```

The k8s-support repo Cloud Build deployment scripts will read it from that
bucket. Once the key has been uploaded, you will need to add an *additional*
`-token.verify-key` flag to all of the services that use them:

* [ndt.jsonnet](https://github.com/m-lab/k8s-support/blob/master/k8s/daemonsets/experiments/ndt.jsonnet)
* [ndt-canary.jsonnet](https://github.com/m-lab/k8s-support/blob/master/k8s/daemonsets/experiments/ndt-canary.jsonnet)
* [wehe.jsonnet](https://github.com/m-lab/k8s-support/blob/master/k8s/daemonsets/experiments/wehe.jsonnet)

**NOTE**: do not remove or overwrite the existing `-token-verify-key`
flags. The flag can be specified multiple times, and we are adding a new one. The
value of the new flag should be the filename for the public key you uploaded to
GCS.

Once the k8s-support Cloud Build has completed after a push to the repo (or a
tag, for production), verify that the Secret named `locate-verify-keys` contains
the file you uploaded to GCS:

```sh
kubectl --context <project> describe secret locate-verify-keys
```

Modifying the DaemonSet pod specs (with the additional flag) will cause rolling
updates of the DaemonSets, deploying the new public, verifier key. Once you are
100% sure that all ndt-servers have rolled out the change, and you have verified
that the ndt-server has loaded the additional public key, then, and only then,
can you enable the new signer key in the Secret Manager. You can enable it with
a command like:

```sh
gcloud secrets versions list locate-service-signer-key --project <project>
# Note the version number of the new, disabled version
gcloud secrets versions enable <version> --secret locate-service-verify-key --project <project>
```

LS instances will start using the new signer key when they are restarted.
You can use the `cbctl` command to trigger a Cloud Build of the LS, which will
redeploy the service, causing all instances to be restarted. For example:

```sh
go get github.com/m-lab/gcp-config/cmd/cbctl
cbctl -repo locate -trigger_name push-m-lab-locate-trigger -project mlab-staging
```

### Monitoring

The script-exporter pod in the prometheus-federation cluster uses the private
key for script-exporter monitoring. To deploy the new private key, update the
MONITORING_SIGNER_KEY evironment variable in the Travis-CI settings for the
prometheus-support repo:

[https://travis-ci.com/github/m-lab/prometheus-support/settings](https://travis-ci.com/github/m-lab/prometheus-support/settings)

You will need to delete the existing MONITORING_SIGNER_KEY environment variable
and recreate it using the value found in the file "jwk_sig_EdDSA_monitoring_\<keyid\>"
after the script is run.

The next time the prometheus-support Travis-CI build runs for a project, the new
key from that environment variable will start to be used.

**WARNING**: do not deploy the new private key, until you are sure that all LS
instances in AppEngine have restarted and picked up the new verify public key,
else script-exporter monitoring requests will fail.
