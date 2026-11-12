# result-uploader

Uploads tool results from a given directory to the Test of Things web app.

## Build

```sh
make build   # -> bin/result-uploader
```

A prebuilt binary is published as the `latest` GitHub release on every push to `main`.

## Usage

```sh
result-uploader -dir <path> -statement <statement_id> -api-url <url> -api-key <key> [-workers 4]
```

- Files are uploaded from every directory that has a `00meta.json`. Files in other directories are skipped.
- Trivy reports are uploaded from any directory and need no `00meta.json`. Name them `<software>@<host>.trivy.json`, e.g. `Ruuvi_App_SW@Ruuvi_App.trivy.json`:
  - `<software>` and `<host>` are the software and host system addresses in the security statement.
  - The suffix `.trivy.json` must be lowercase.
- Files that fail to upload (e.g. too large) are logged and skipped. The rest are still processed, and the tool exits with code 1.
