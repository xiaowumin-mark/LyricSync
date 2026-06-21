from lyricsync_client import LyricSyncClient


client = LyricSyncClient()
health = client.health()

print({
    "ok": health.get("ok"),
    "status": health.get("status"),
    "version": health.get("version"),
})
