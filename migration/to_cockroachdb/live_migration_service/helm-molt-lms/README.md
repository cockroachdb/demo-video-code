# Deploy k8s with local changes
1. In the root folder of crdb-proxy (i.e. `<root path>/crdb-proxy/`), build the docker image with tag:
   ```sh
    docker build -t cockroachdb/molt-lms:<your-tag-name> .
   ```
2. In `./helm-molt-lms/values.yaml`, add the below
   ```yaml
   lms:
      image:
         pullPolicy: Never
         # Overrides the image tag whose default is the chart appVersion.
         tag: <your-tag-name>
      lms: ...
   ```
   with pull policy never, it will look for the local image rather than pulling from the docker hub.
