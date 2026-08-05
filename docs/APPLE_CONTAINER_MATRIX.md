# Apple container 1.2.x capability matrix

Probed live on this machine. Used to drive honest UI demotion vs Docker parity.

| Capability | Status | CLI |
|---|---|---|
| container lifecycle (start/stop/rm/exec/logs/stats) | supported | top-level |
| prune stopped containers | supported | `container prune` |
| create / run | supported | `container create`, `container run` |
| image list/pull/delete/prune/inspect | supported | `container image …` |
| volume list/create/delete/prune/inspect | supported | `container volume …` |
| network list/create/delete/prune | supported | `container network …` (UI later) |
| system status / df | supported | `container system status\|df` |

---

# Apple container 1.2.x command matrix (live probe)

## container image
```
OVERVIEW: Manage images

USAGE: container image [--debug] <subcommand>

OPTIONS:
  --debug                 Enable debug output [environment: CONTAINER_DEBUG]
  --version               Show the version.
  -h, --help              Show help information.

SUBCOMMANDS:
  delete, rm              Delete one or more images
  inspect                 Display information about one or more images
  list, ls                List images
  load                    Load images from an OCI compatible tar archive
  prune                   Remove unused or all images
  pull                    Pull an image
  push                    Push an image
  save                    Save one or more images as an OCI compatible tar
                          archive
  tag                     Create a new reference for an existing image

  See 'container help image <subcommand>' for detailed help.
```

## container volume
```
OVERVIEW: Manage container volumes

USAGE: container volume [--debug] <subcommand>

OPTIONS:
  --debug                 Enable debug output [environment: CONTAINER_DEBUG]
  --version               Show the version.
  -h, --help              Show help information.

SUBCOMMANDS:
  create                  Create a new volume
  delete, rm              Delete one or more volumes
  list, ls                List volumes
  inspect                 Display information about one or more volumes
  prune                   Remove volumes with no container references

  See 'container help volume <subcommand>' for detailed help.
```

## container network
```
OVERVIEW: Manage container networks

USAGE: container network [--debug] <subcommand>

OPTIONS:
  --debug                 Enable debug output [environment: CONTAINER_DEBUG]
  --version               Show the version.
  -h, --help              Show help information.

SUBCOMMANDS:
  create                  Create a new network
  delete, rm              Delete one or more networks
  list, ls                List networks
  inspect                 Display information about one or more networks
  prune                   Remove networks with no container connections

  See 'container help network <subcommand>' for detailed help.
```

## container system
```
OVERVIEW: Manage system components

USAGE: container system [--debug] <subcommand>

OPTIONS:
  --debug                 Enable debug output [environment: CONTAINER_DEBUG]
  --version               Show the version.
  -h, --help              Show help information.

SUBCOMMANDS:
  df                      Show disk usage for images, containers, and volumes
  dns                     Manage local DNS domains
  kernel                  Manage the default kernel configuration
  logs                    Fetch system logs for `container` services
  property                Manage system property values
  start                   Start `container` services
  status                  Show the status of `container` services
  stop                    Stop all `container` services
  version                 Show version information

  See 'container help system <subcommand>' for detailed help.
```

## container prune
```
OVERVIEW: Remove all stopped containers

USAGE: container prune [--debug]

OPTIONS:
  --debug                 Enable debug output [environment: CONTAINER_DEBUG]
  --version               Show the version.
  -h, --help              Show help information.

```

## container create
```
OVERVIEW: Create a new container

USAGE: container create [<options>] <image> [<arguments> ...]

ARGUMENTS:
  <image>                 Image name
  <arguments>             Container init process arguments

PROCESS OPTIONS:
  -e, --env <env>         Set environment variables (key=value, or just key to
                          inherit from host)
  --env-file <env-file>   Read in a file of environment variables (key=value
                          format, ignores # comments and blank lines)
  --gid <gid>             Set the group ID for the process
  -i, --interactive       Keep the standard input open even if not attached
  -t, --tty               Open a TTY with the process
  -u, --user <user>       Set the user for the process (format: name|uid[:gid])
  --uid <uid>             Set the user ID for the process
  -w, --workdir, --cwd <dir>
                          Set the initial working directory inside the container
  --ulimit <limit>        Set resource limits (format: <type>=<soft>[:<hard>])

RESOURCE OPTIONS:
  -c, --cpus <cpus>       Number of CPUs to allocate to the container
  -m, --memory <memory>   Amount of memory (1MiByte granularity), with optional
                          K, M, G, T, or P suffix

MANAGEMENT OPTIONS:
  -a, --arch <arch>       Set arch if image can target multiple architectures
                          (default: arm64)
  --cap-add <cap>         Add a Linux capability (e.g. CAP_NET_RAW, or ALL)
  --cap-drop <cap>        Drop a Linux capability (e.g. CAP_NET_RAW, or ALL)
  --cidfile <cidfile>     Write the container ID to the path provided
  -d, --detach            Run the container and detach from the process
  --dns <ip>              DNS nameserver IP address
```

## container run
```
OVERVIEW: Run a container

USAGE: container run [<options>] <image> [<arguments> ...]

ARGUMENTS:
  <image>                 Image name
  <arguments>             Container init process arguments

PROCESS OPTIONS:
  -e, --env <env>         Set environment variables (key=value, or just key to
                          inherit from host)
  --env-file <env-file>   Read in a file of environment variables (key=value
                          format, ignores # comments and blank lines)
  --gid <gid>             Set the group ID for the process
  -i, --interactive       Keep the standard input open even if not attached
  -t, --tty               Open a TTY with the process
  -u, --user <user>       Set the user for the process (format: name|uid[:gid])
  --uid <uid>             Set the user ID for the process
  -w, --workdir, --cwd <dir>
                          Set the initial working directory inside the container
  --ulimit <limit>        Set resource limits (format: <type>=<soft>[:<hard>])

RESOURCE OPTIONS:
  -c, --cpus <cpus>       Number of CPUs to allocate to the container
  -m, --memory <memory>   Amount of memory (1MiByte granularity), with optional
                          K, M, G, T, or P suffix

MANAGEMENT OPTIONS:
  -a, --arch <arch>       Set arch if image can target multiple architectures
                          (default: arm64)
  --cap-add <cap>         Add a Linux capability (e.g. CAP_NET_RAW, or ALL)
  --cap-drop <cap>        Drop a Linux capability (e.g. CAP_NET_RAW, or ALL)
  --cidfile <cidfile>     Write the container ID to the path provided
  -d, --detach            Run the container and detach from the process
  --dns <ip>              DNS nameserver IP address
```

## container image prune
```
Error: Plugins are unavailable. Start the container system services and retry:

    container system start

Check to see that the plugin exists under:
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container-plugins/image prune
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container/plugins/image prune

Usage: container [--debug] <subcommand>
  See 'container --help' for more information.
```

## container volume create
```
Error: Plugins are unavailable. Start the container system services and retry:

    container system start

Check to see that the plugin exists under:
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container-plugins/volume create
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container/plugins/volume create

Usage: container [--debug] <subcommand>
  See 'container --help' for more information.
```

## container volume prune
```
Error: Plugins are unavailable. Start the container system services and retry:

    container system start

Check to see that the plugin exists under:
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container-plugins/volume prune
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container/plugins/volume prune

Usage: container [--debug] <subcommand>
  See 'container --help' for more information.
```

## container network list
```
Error: Plugins are unavailable. Start the container system services and retry:

    container system start

Check to see that the plugin exists under:
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container-plugins/network list
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container/plugins/network list

Usage: container [--debug] <subcommand>
  See 'container --help' for more information.
```

## container system status
```
Error: Plugins are unavailable. Start the container system services and retry:

    container system start

Check to see that the plugin exists under:
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container-plugins/system status
  - /opt/homebrew/Cellar/container/1.2.0/libexec/container/plugins/system status

Usage: container [--debug] <subcommand>
  See 'container --help' for more information.
```

OVERVIEW: Manage images

USAGE: container image [--debug] <subcommand>

OPTIONS:
  --debug                 Enable debug output [environment: CONTAINER_DEBUG]
  --version               Show the version.
  -h, --help              Show help information.

SUBCOMMANDS:
  delete, rm              Delete one or more images
  inspect                 Display information about one or more images
  list, ls                List images
  load                    Load images from an OCI compatible tar archive
  prune                   Remove unused or all images
  pull                    Pull an image
  push                    Push an image
  save                    Save one or more images as an OCI compatible tar
                          archive
  tag                     Create a new reference for an existing image

  See 'container help image <subcommand>' for detailed help.
