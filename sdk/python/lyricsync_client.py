from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any
from urllib.parse import urlparse, urlunparse
from urllib.request import Request, urlopen


@dataclass
class LyricSyncClient:
    base_url: str = "http://127.0.0.1:41917"
    timeout: float = 10.0

    def __post_init__(self) -> None:
        self.base_url = self.base_url.rstrip("/")

    def health(self) -> dict[str, Any]:
        return self.get("/api/health")

    def state(self) -> dict[str, Any]:
        return self.get("/api/state")

    def song(self) -> dict[str, Any]:
        return self.get("/api/song")

    def lyric(self) -> dict[str, Any]:
        return self.get("/api/lyric")

    def sessions(self) -> list[dict[str, Any]]:
        return self.get("/api/session")

    def clients(self) -> list[dict[str, Any]]:
        return self.get("/api/clients")

    def logs(self) -> list[dict[str, Any]]:
        return self.get("/api/logs")

    def config(self) -> dict[str, Any]:
        return self.get("/api/config")

    def metrics(self) -> dict[str, Any]:
        return self.get("/api/metrics")

    def get(self, path: str) -> Any:
        request = Request(f"{self.base_url}{path}", method="GET")
        with urlopen(request, timeout=self.timeout) as response:
            if response.status < 200 or response.status >= 300:
                raise RuntimeError(f"LyricSync GET {path} failed: {response.status}")
            return json.loads(response.read().decode("utf-8"))

    def event_websocket_url(self) -> str:
        return self.websocket_url("/ws")

    def amll_websocket_url(self) -> str:
        return self.websocket_url("/amll/ws")

    def websocket_url(self, path: str) -> str:
        parsed = urlparse(self.base_url)
        scheme = "wss" if parsed.scheme == "https" else "ws"
        return urlunparse((scheme, parsed.netloc, path, "", "", ""))
