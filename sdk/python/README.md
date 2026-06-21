# LyricSync Python Client

```python
from lyricsync_client import LyricSyncClient

client = LyricSyncClient()
health = client.health()
print(health["status"])
```

The client uses only Python's standard library for HTTP calls. WebSocket helpers return endpoint URLs; use your preferred WebSocket implementation to connect.
