# LyricSync Go Client

```go
package main

import (
	"context"
	"fmt"

	"lyricsync/sdk/go/lyricsync"
)

func main() {
	client := lyricsync.New("http://127.0.0.1:41917")
	health, err := client.Health(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println(health.Status)
}
```

The Go client reuses LyricSync's shared `pkg/model` types and supports both HTTP reads and WebSocket connections.
