# Is my Homelab up? - Agent

This is the agent part of the project. See the [project README](https://github.com/is-my-homelab-up/.github/blob/main/profile/README.md) for more details about the project.

## Configuration

The agent can be configured using environemnt variables.

| Environment Variable | Default Value  | Description                                             |
|----------------------|----------------|---------------------------------------------------------|
| `CLOUD_ADDRESS`      | empty          | Address of the cloud service.                           |
| `CLOUD_ENDPOINT`     | `v1/announce`  | Endpoint to notify the cloud service                    |
| `CLOUD_ID`           | empty          | Agent ID provided by the cloud service on registration. |
| `CLOUD_APIKEY`       | empty          | API key provided by the cloud service on registration.  |
| `INTERVAL`           | `10`           | Interval in seconds for the agent to notify the cloud   |
| `SERVER_ADDRESS`     | `0.0.0.0:8080` | Address of the agent health server                      |
| `LOG_LEVEL`          | `INFO`         | Minimum log level (`DEBUG`, `INFO`, `WARN`, `ERROR`)    |

## License

This project uses the AGPL-3.0 license. See the [LICENSE file](./LICENSE) for the full legal text.

