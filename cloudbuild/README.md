# Why?

App Engine deployments for "custom" flex configs _require_ that only one file
of type Dockerfile and cloudbuild.yaml be present in the top level directory.

As well, `gcloud app deploy app.yaml` uses the directory with the app.yaml
file as the "source" directory, which requires that it be at the top level
directory, along with the Dockerfile.
