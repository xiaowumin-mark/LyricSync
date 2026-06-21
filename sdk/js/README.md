# LyricSync JavaScript Client

```js
import { LyricSyncClient } from '@lyricsync/client'

const client = new LyricSyncClient()
const health = await client.health()
console.log(health.status)

const ws = client.connectEvents()
ws.onmessage = (event) => {
  console.log(JSON.parse(event.data).type)
}
```

The client uses `fetch` and `WebSocket` from the runtime. In Node.js, use Node 22+ or pass compatible implementations.
