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

## Key rotation

Periodically, the access token signing key should be rotated. To ensure
continuity of verification, clients (in particular the ndt-server) MUST posess
both the current and next verifier key.

To create a new key pairs for both the Locate Service (LS) and monitoring, run the script
./management/create_jwt_keys_and_secrets.sh. If a Secret Manager secret for the
key already exists, you will be prompted to add a new version of the secret.

### Locate Service

The LS reads the signer key directly from the Secret Manager upon initializion
at runtime. When the script creates a new version of a secret, it will disable
the new signer key version. This will prevent the LS from using the new signer
key when it starts up. Before enabling the key, you must ensure that the
associated public key has been distributed to all ndt-servers for the project.

To do this, first, upload the public key to Google Cloud storage. The public key
will be found in the directory where you ran the script, and will have a name
like "jwk_sig_EdDSA_locate_\<keyid\>.pub".  Upload it to this bucket:

gs://k8s-support-\<project\>/locate

The k8s-support repo Cloud Build deployment scripts will read it from that
bucket. Once the key has been uploaded, you will need to add an *additional*
`-token.verify-key` flag to the list of flag for the ndt-server. *NOTE*: do not
remove or overwrite the existing `-token-verify-key` flag. The flag can be
specified multiple times, and we are adding a new one. The value of the new flag
should be the filename for the public key you uploaded to GCS.

Once the k8s-support Cloud Build has completed after a push to the repo (or a
tag, for production), verify that the Secret named `locate-verify-keys` contains
the file you uploaded to GCS:

```sh
kubectl --context <project> describe secret locate-verify-keys
```

Modifying the ndt DaemonSet pod spec (with the additional flag) will cause a
rolling update of the DaemonSet, deploying the new public key. Once you are 100%
sure that all ndt-servers have rolled out the change, and you have verified that
the ndt-server has loaded the additional public key, then, and only then, can
you enable the new signer key in the Secret Manager. You can enable it with a
command like:

```sh
gcloud secrets versions list locate-service-signer-key --project <project>
# Note the version number of the new, disabled version
gcloud secrets versions enable <version> --secret locate-service-verify-key --project <project>
```

LS instances will start using the new signer key when they are restarted.
AppEngine instances get restarted periodically by GCP. If you don't want to wait
a week or more for them to naturally restart, then you could manually trigger
the `push-m-lab-locate-trigger` Cloud Build trigger against the appropriate
branch or tag for the project to re-deploy the LS to AppEngine.

### Monitoring

The script-exporter pod in the prometheus-federation cluster uses the private
key for script-exporter monitoring. To deploy the new private key, update the
MONITORING_SIGNER_KEY evironment variable in the Travis-CI settings for the
prometheus-support repo:

https://travis-ci.com/github/m-lab/prometheus-support/settings

You will need to delete the existing MONITORING_SIGNER_KEY environment variable
and recreate it using the value found in the file "jwk_sig_EdDSA_monitoring_\<keyid\>"
after the script is run.

The next time the prometheus-support Travis-CI build runs for a project, the new
key from that environment variable will start to be used.

**WARNING**: do not deploy the new private key, until you are sure that all LS
instances in AppEngine have restarted and picked up the new verify public key,
else script-exporter monitoring requests will fail.

## TODO

* Describe JWK management for
  [m-lab/k8s-support](http://github.com/m-lab/k8s-support).
