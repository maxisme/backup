# backup
[![push](https://github.com/maxisme/backup/workflows/push/badge.svg)](https://github.com/maxisme/backup/actions)

## Setup
1. Add servers to backup in the `servers.json`:
    ```json
    {
      "servers": {
        "server-name": {
          "host": "root@1.2.3.4"
        }
      },
      "exclude-dirs": [
        "/dev/*",
        "/proc/*",
        "/sys/*",
        "/tmp/*",
        "/run/*",
        "/mnt/*",
        "/media/*",
        "/lost+found/*",
        "/var/log/*",
        "/var/lib/docker/*"
      ],
      "encryption-key": "CHANGE ME"
    }
    ```
2. Addd docker-compose.yml like this:
    ```yaml
    version: '3.1'
    services:
      backup:
        build: .
        environment:
          - CRON=0 */12 * * *
        volumes:
          - "./servers.json:/servers.json"
          - "/root/.ssh:/root/.ssh"
          - "./backup/:/backup/"
          - "/root/.config/rclone:/root/.config/rclone"
    ```
   
## Decrypting backup
To decrypt backup simply run inside the `cmd` directory:
```
$ go run . /path/to/file key
```

## how to profile
```bash
go test -memprofile mem.prof -cpuprofile cpu.prof
go tool pprof -png mem.prof
go tool pprof -png cpu.prof
```