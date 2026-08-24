# podtop

`podtop` is an interactive terminal monitor for Podman containers. It shows
CPU and memory usage along with their all-time highs. Data is read from
`podman stats --format json` or `podman-compose stats --format json`.

## Requirements

- Go 1.24 or newer
- Podman
- `podman-compose` for Compose mode
- Mouse support in the terminal is optional

## Compiling

The Go module is located in the `src/podtop` subdirectory:

```sh
cd src/podtop
go build -o podtop .
```

This creates the executable `src/podtop/podtop`. Alternatively, run the
program directly:

```sh
cd src/podtop
go run .
```

## Usage

By default, `podtop` displays all running and stopped containers:

```sh
./podtop
```

### Modes

Monitor all containers:

```sh
./podtop -mode all
```

Monitor containers from a Compose file:

```sh
./podtop -mode compose -file compose.yml
```

Monitor a single container:

```sh
./podtop -mode one -container my-container
```

### Options

| Option | Default | Description |
| --- | --- | --- |
| `-mode` | `all` | Mode: `all`, `compose`, or `one` |
| `-container` | empty | Container name or ID for `one` mode |
| `-file` | `compose.yml` | Compose file for `compose` mode |
| `-history` | `60` | Number of measurements stored per container |
| `-refresh` | `2s` | Refresh interval, for example `1s` or `500ms` |
| `-interval` | `5s` | Podman report interval, for example `1s` |
| `-escape-box` | `false` | Run Podman through `host-spawn` |

`-refresh` controls UI updates, while `-interval` controls how often Podman
generates a new stats report. Example:

```sh
./podtop -mode all -history 120 -refresh 1s -interval 1s
```

## Controls

- `q` or `Ctrl+C`: quit the program
- Up/down arrow keys: select a container
- Mouse wheel: select a container

The program also exits when it receives `SIGTERM`.
