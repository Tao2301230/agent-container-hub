agent-container-hub image bundle

This bundle is intended for offline image distribution plus runtime configuration delivery.
It targets Linux hosts and includes the saved container image together with the live config tree.

What is included:
- .env.example
- configs/environments/ runtime configs
- images/*.tar.gz offline image archive
- load-image.sh helper to import the bundled image into docker

Bundle layout notes:
- configs/environments is treated as the live environment config source.
- No empty data/ tree is pre-created in the bundle; runtime data should be provisioned by the deployment environment.
- The image archive name encodes the exact version and architecture.

Recommended deployment flow:
1. Extract the tar.gz bundle.
2. Change into the extracted agent-container-hub directory.
3. Copy .env.example to .env and adjust values for the target host.
4. Run ./load-image.sh to import the bundled image into the local docker daemon.
5. Start containers using your deployment method of choice with the imported image tag.

Notes:
- This bundle does not include a host-process binary release.
- The runtime service still depends on access to a compatible container engine on the deployment host.
