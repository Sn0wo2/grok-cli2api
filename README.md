# grok-cli2api

> Converts grok-cli authentication to the OpenAI Responses API format

### Currently, the following is achieved:

- Supports CLI Login
- Account pool management, automatically refreshing grok-cli keys via refresh tokens
- Supports usage logs
- Compatible with /v1/models

### TODO:

- Load balancing, circuit breaking, and rollback of the pool of accounts
- RESTful API, including but not limited to usage queries
- GoReleaser

## CLI

| Command                           | Description               |
|-----------------------------------|---------------------------|
| `grok-cli2api`                    | Start grok-cli2api        |
| `grok-cli2api serve [-p port]`    | Specify a custom port     |
| `grok-cli2api login [--device]`   | Login to your grok-cli    |
| `grok-cli2api list`               | List all accounts         |
| `grok-cli2api whoami [-a email]`  | Lookup account info       |
| `grok-cli2api refresh [-a email]` | Manually refresh token    |
| `grok-cli2api logout [-a email]`  | Delete account from auths |
| `grok-cli2api help`               | Help                      |

## Auths

- You can copy `~/.grok/auth.json` to `./data/auths/*.json`.

## Environment variables

| Variable                       | Default                              | Description       |
|--------------------------------|--------------------------------------|-------------------|
| `GROK_AUTHS_DIR`               | `./data/auths`                       | Auths dir         |
| `GROK_CLI_CHAT_PROXY_BASE_URL` | `https://cli-chat-proxy.grok.com/v1` | Upstream base URL |
| `GROK_SERVE_PORT`              | `8317`                               | Listening port       |

# License

This project is licensed under the [**Me0wo NC Public License v1.3**](LICENSE).
If you are unfamiliar with the Me0wo NC Public License v1.3, please refer
to [Me0wo-LICENSE](https://github.com/Sn0wo2/Me0wo-LICENSE).

# Thanks

- [LINUX DO](https://linux.do)