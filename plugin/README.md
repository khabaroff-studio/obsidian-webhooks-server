# Webhooks V2

Receive webhooks from external services and create notes in your vault via a self-hosted server with real-time delivery.

## Features

- **Real-time delivery** via Server-Sent Events (SSE) with polling fallback
- **Exactly-once delivery** with acknowledgment (ACK) system
- **Event deduplication** prevents duplicate writes
- **Append or overwrite** mode for file content
- **Auto-create folders** when target path doesn't exist
- **Connection status** in the status bar
- **Self-hosted** — your data stays on your server

## Requirements

- A running instance of [Webhooks Server](https://github.com/khabaroff-studio/obsidian-webhooks-v2)
- A client key from the server dashboard

## Network usage

This plugin connects to your self-hosted webhook server to receive events via SSE and HTTP polling. All network traffic goes exclusively to the server URL you configure in settings. No data is sent to any third-party service.

## Installation

### From community plugins

1. Open **Settings > Community plugins > Browse**
2. Search for **Webhooks V2**
3. Click **Install**, then **Enable**

### Manual

1. Download `main.js`, `manifest.json`, and `styles.css` from the [latest release](https://github.com/khabaroff-studio/obsidian-webhooks-v2/releases)
2. Create a folder `webhooks-v2` in your vault's `.obsidian/plugins/` directory
3. Copy the downloaded files into that folder
4. Enable the plugin in **Settings > Community plugins**

## Usage

1. Register at your server's dashboard to get a **client key**
2. Open plugin settings and paste your client key
3. Click **Test** to verify the connection
4. Send a webhook to your server:

```bash
curl -X POST "https://your-server.com/webhook/YOUR_WEBHOOK_KEY?path=inbox/note.md" \
  -H "Content-Type: text/plain" \
  -d "Hello from webhook!"
```

The content will appear in your vault at the specified path.

## Credits

Based on the original [obsidian-webhooks](https://github.com/trashhalo/obsidian-webhooks) plugin by [@trashhalo](https://github.com/trashhalo).

## License

[MIT](LICENSE)
