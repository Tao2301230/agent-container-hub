agent-container-hub program bundle

This bundle is intended for host-process deployment on the target OS encoded in the archive name.
It includes the backend binary, runtime configs, and platform entry scripts. The management UI remains embedded in the Go binary; there is no separate frontend/dist tree in this project.

What is included:
- manifest.json
- .env.example
- README.txt
- backend/agent-container-hub(.exe)
- configs/environments/ runtime configs
- current-platform deploy/start/stop entry scripts
- scripts/program-common.{sh|ps1}

Deployment steps:
1. Extract the archive for the matching host OS.
2. Change into the extracted agent-container-hub directory.
3. Copy .env.example to .env and adjust paths, bind address, auth token, and ENGINE if needed.
4. Run ./deploy.sh on macOS/Linux or ./deploy.ps1 on Windows to validate the bundle and create runtime directories.
5. Start with ./start.sh or ./start.sh --daemon on macOS/Linux, or ./start.ps1, ./start.ps1 --daemon, or ./start.ps1 -Daemon on Windows.
6. Use ./stop.sh or ./stop.ps1 only for daemon-mode processes managed by the bundle scripts.

Layout notes:
- manifest.json is the host-facing bundle contract and declares the embedded UI entry at /app.
- configs/environments remains in the bundle because it is the runtime source of truth for environment definitions.
- data/ and run/ are created on first deploy/start and are not pre-created in the archive.
- If ENGINE=auto, or if ENGINE is empty, the service auto-detects docker first and then podman. Startup validates the selected engine with `info` and exits if the daemon is unreachable.
