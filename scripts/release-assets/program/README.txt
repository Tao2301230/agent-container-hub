agent-container-hub release bundle

This bundle is intended for host-process deployment on the target OS encoded in the tarball name.
For example:
- *-linux-amd64.tar.gz / *-linux-arm64.tar.gz: Linux hosts
- *-darwin-amd64.tar.gz / *-darwin-arm64.tar.gz: macOS hosts

It does not include container images or source code build tooling.

What is included:
- agent-container-hub binary
- .env.example
- configs/environments/ runtime configs
- start.sh / stop.sh
- systemd/agent-container-hub.service (Linux bundles only)

Deployment steps:
1. Extract the tar.gz bundle.
2. Change into the extracted agent-container-hub directory.
3. Copy .env.example to .env and adjust paths, bind address, auth token, and ENGINE if needed.
4. If ENGINE is left empty or set to docker/podman, make sure that engine is installed and the service user can access it.
5. If you need to run without Docker Desktop, set ENGINE=local explicitly. This keeps the API shape but runs commands on the host without container isolation or image builds.
6. Start with ./start.sh or ./start.sh --daemon.

systemd:
- Linux bundles include a template unit at systemd/agent-container-hub.service.
- Replace /opt/agent-container-hub with your real install path before enabling it.

Notes:
- configs/environments is treated as the live environment config source.
- data/rootfs and data/builds are kept outside the binary and should live on persistent storage in production.
- stop.sh only stops processes started by ./start.sh --daemon.
- The host OS must match the bundle name; a Linux host bundle will not run on macOS, and vice versa.
