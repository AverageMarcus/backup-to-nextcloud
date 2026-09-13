# backup-to-nextcloud

A simple application that will backup files to Nextcloud.

Designed to be run as a CronJob with a focus on directories containing backup archives (e.g. daily zip files).

## Features

* Define jobs via yaml config
* Optionally clean up files on Nextcloud older than a given age
* Optionally only store the latest X number of files

## Example Config

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

## Credentials

Generate an app password for your user in Nextcloud then populate the following environment variables:

* `NEXTCLOUD_USER`
* `NEXTCLOUD_PASSWORD`

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
