# Messages with Zookeeper
This is a message broker based on Yahoo's Zookeeper distributed consensus system.

## Docker usage
To launch the Docker containers, `cd` into the root folder (the one containing `Dockerfile`), then run:
```sh
docker compose -f <docker_compose_file_name> up --build
```

To start a Bash terminal on a Docker container, run:
```sh
docker exec -it <container-id> /bin/bash
```

To attach your terminal to the process running in the Docker container, run:
```sh
docker attach <container-id>
```

To change the file being compiled, open the dockerfile and change the target file in the Go build command.
```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -o ./go-main ./dockertest/main.go
```
## Folder structure
The packages used for this project are located in the `/internal`. Following is a brief description of the packages; for more detailed information, refer to the README files in the corresponding directories.

### Connection manager
This handles TCP messaging between nodes. This should be used with Docker.

### Local connection manager
This simulates the behaviour of Connection Manager, but runs on a single machine (not used with Docker).

### Logger
This implements a global logger (stored as a package-level variable in `logger`), for easy severity-level debugging.

### Proposals
This implements the Zab proposals protocol.

### Znode
This implements the ZNode system which is managed by Zab.
