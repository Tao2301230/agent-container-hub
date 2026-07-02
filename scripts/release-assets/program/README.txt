agent-container-hub program bundle

This bundle is intended for host-process deployment on the target OS encoded in the archive name.
It includes the backend binary, runtime configs, and platform entry scripts. The management UI remains embedded in the Go binary; there is no separate frontend/dist tree in this project.

What is included:
- manifest.json
- .env.example
- README.txt
- backend/agent-container-hub(.exe)
- configs/hub.example.yml service config template
- configs/environments/ runtime configs
- current-platform deploy/start/stop entry scripts
- scripts/program-common.{sh|ps1}

Deployment steps:
1. Extract the archive for the matching host OS.
2. Change into the extracted agent-container-hub directory.
3. Run ./deploy.sh on macOS/Linux or ./deploy.ps1 on Windows to validate the bundle and create .env and configs/hub.yml. Use --output-dir to choose the config output directory.
4. Adjust .env for SERVER_HOST, SERVER_PORT, ENGINE, and AUTH_TOKEN. Adjust configs/hub.yml for paths if needed.
5. Start with ./start.sh or ./start.sh --daemon on macOS/Linux, or ./start.ps1, ./start.ps1 --daemon, or ./start.ps1 -Daemon on Windows.
6. Use ./stop.sh or ./stop.ps1 only for daemon-mode processes managed by the bundle scripts.

Layout notes:
- manifest.json is the host-facing bundle contract and declares the embedded UI entry at /app.
- configs/hub.yml is copied from configs/hub.example.yml on first deploy and is not overwritten later.
- configs/environments remains in the bundle because it is the runtime source of truth for environment definitions.
- data/ and run/ are created on first start and are not pre-created in the archive.
- If ENGINE is auto or empty, the service auto-detects docker first and then podman. Startup validates the selected engine with `info` and exits if the daemon is unreachable.
