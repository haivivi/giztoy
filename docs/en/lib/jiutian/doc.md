# Jiutian API Documentation

九天大模型 (Jiutian) - China Mobile's AI Service Platform.

> **Official API Documentation**: [api/README.md](./api/README.md)

## Overview

Jiutian is China Mobile's terminal intelligent agent service management platform for AI/LLM cloud integration.

**Note:** This is API documentation only. No Go/Rust SDK implementation exists in this repository.

## API Features

- **Chat Completions**: OpenAI-compatible chat API
- **Device Integration**: Device registration and heartbeat protocols
- **Assistant Management**: Configure AI assistants

## Integration Notes

For integration with Jiutian API:

1. Use OpenAI-compatible SDK with custom base URL
2. Follow device authentication protocols
3. See [api/tutorial.md](./api/tutorial.md) for quick start

## Authentication

Requires:
- AI Token (obtained via email application)
- IP whitelist registration
- Product ID from management platform

## Environment

| Environment | URL |
|-------------|-----|
| Test | `https://z5f3vhk2.cxzfdm.com:30101` |
| Production | `https://ivs.chinamobiledevice.com:30100` |

## SDK Implementation Status

| Language | Status |
|----------|--------|
| Go | ❌ Not implemented |
| Rust | ❌ Not implemented |

For basic integration, use OpenAI-compatible SDK:

**Go:**
```go
config := openai.DefaultConfig(jiutianToken)
config.BaseURL = "https://ivs.chinamobiledevice.com:30100/v1"
client := openai.NewClientWithConfig(config)
```

## Related Documentation

- [Quick Start Tutorial](./api/tutorial.md)
- [Concepts & Terms](./api/concepts.md)
- [Authentication](./api/auth.md)
- [Chat API](./api/chat.md)
- [Device Protocol](./api/device.md)
- [FAQ](./api/faq.md)
