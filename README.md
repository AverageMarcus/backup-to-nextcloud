# backup-to-nextcloud

A simple application that will backup files to Nextcloud.

Designed to be run as a CronJob with a focus on directories containing backup archives (e.g. daily zip files).

## Features

* Define jobs via yaml config
* Optionally clean up files on Nextcloud older than a given age
* Optionally only store the latest X number of files

## Usage

### Example Config

```yaml
nextcloudURL: https://nextcloud.example.com
jobs:
  - sourceDirectory: backups/example-daily
    destinationDirectory: backups/example
    # Optional
    maxAge: 168h # 7 days
    # Optional
    maxItems: 7
```

By default, this loads from `./config.yaml` but the file path can be specified by setting the `NEXTCLOUD_CONFIG_PATH` environment variable.

### Credentials

Generate an app password for your user in Nextcloud then populate the following environment variables:

* `NEXTCLOUD_USER`
* `NEXTCLOUD_PASSWORD`

### Example Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup-to-nextcloud
  labels:
    app.kubernetes.io/name: backup-to-nextcloud
spec:
  schedule: "30 11 * * *" # Run every day at 11:30 AM
  jobTemplate:
    metadata:
      labels:
        cronjob: backup-to-nextcloud
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: rg.fr-par.scw.cloud/averagemarcus/backup-to-nextcloud:latest
            imagePullPolicy: IfNotPresent
            env:
              - name: NEXTCLOUD_CONFIG_PATH
                value: /config/config.yaml
            envFrom:
              - secretRef:
                  name: backup-to-nextcloud-credentials
            volumeMounts:
            - mountPath: /backup
              name: backup-source
            - mountPath: /config
              name: config
          restartPolicy: OnFailure
          volumes:
          - name: backup-source
            persistentVolumeClaim:
              claimName: source-to-backup
          - name: config
            configMap:
              name: backup-to-nextcloud-jobs
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: backup-to-nextcloud-jobs
data:
  config.yaml: |
    nextcloudURL: https://nextcloud.example.com
    jobs:
      - sourceDirectory: /backup/app1
        destinationDirectory: /backup/app1
        maxAge: 168h
        maxItems: 7
---
apiVersion: v1
kind: Secret
metadata:
  name: backup-to-nextcloud-credentials
stringData:
  NEXTCLOUD_USER: XXXX
  NEXTCLOUD_PASSWORD: XXXX

```

## Building from source

With Docker:

```sh
make docker-build
```

Standalone:

```sh
make build
```

## Resources

* [Nextcloud WebDav docs](https://docs.nextcloud.com/server/stable/developer_manual/client_apis/WebDAV/basic.html)

## Contributing

If you find a bug or have an idea for a new feature please [raise an issue](issues/new) to discuss it.

Pull requests are welcomed but please try and follow similar code style as the rest of the project and ensure all tests and code checkers are passing.

Thank you 💛

## License

See [LICENSE](LICENSE)

