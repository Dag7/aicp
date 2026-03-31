# aicp

Python SDK for the Agentic Identity and Commerce Protocol (AICP).

## Installation

```bash
pip install aicp
```

## Quick Start

```python
import asyncio
from aicp import AICPAgent

async def main():
    agent = await AICPAgent.create(
        server_url="https://aicp.example.com",
        private_key=b"..." # 32-byte Ed25519 private key
    )
    service = await agent.discover("example.com")
    result = await agent.verify("did:aicp:abc123")
    await agent.close()

asyncio.run(main())
```
