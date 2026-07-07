# grok-cli2api

`grok-cli2api` converts grok-cli traffic to the OpenAI Responses format with added header metadata.

### Currently, the following is achieved:
- Supports CLI Login
- Account pool management, automatically refreshing grok-cli keys via refresh tokens
- Supports usage logs
- Compatible with /v1/models

### TODO:
- Load balancing, circuit breaking, and rollback of the pool of accounts
- RESTful API, including but not limited to usage queries

## CLI

| Command                           | Description               |
|-----------------------------------|---------------------------|
| `grok-cli2api`                    | Start grok-cli2api        |
| `grok-cli2api serve [-p port]`    | Custom port               |
| `grok-cli2api login [--device]`   | Login your grok-cli       |
| `grok-cli2api list`               | list all accounts         |
| `grok-cli2api whoami [-a email]`  | Lookup account info       |
| `grok-cli2api refresh [-a email]` | Manual refresh token      |
| `grok-cli2api logout [-a email]`  | Delete account from auths |
| `grok-cli2api help`               | Help                      |

## Auths

- You can copy `~/.grok/auth.json` to `./data/auths/*.json`.

## Environment variables

| Variable                      | Default                              | Description       |
|-------------------------------|--------------------------------------|-------------------|
| `GROK_AUTHS_DIR`              | `./data/auths`                       | Auths dir         |
| `GROK_CLI_CHAT_PROXY_BASE_URL` | `https://cli-chat-proxy.grok.com/v1` | Upstream base url |
| `GROK_SERVE_PORT`             | `8317`                               | Listen port       |

# License
This project is licensed under the [**Me0wo NC Public License v1.3**](LICENSE).
If you are unfamiliar with the Me0wo NC Public License v1.3, please refer to [Me0wo-LICENSE](https://github.com/Sn0wo2/Me0wo-LICENSE).
