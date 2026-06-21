import { LyricSyncClient } from '../index.js'

const client = new LyricSyncClient(process.env.LYRICSYNC_URL || 'http://127.0.0.1:41917')
const health = await client.health()

console.log(JSON.stringify({
  ok: health.ok,
  status: health.status,
  version: health.version
}, null, 2))
