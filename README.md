
# Karyon-go

karyon jsonrpc client, written in go.

## Example 

```go

import (
	rpc "github.com/karyontech/karyon-go/jsonrpc/client"
)

config := rpc.RPCClientConfig{
	Addr: "ws://127.0.0.1:6000",
}

client, err := rpc.NewRPCClient(config)
if err != nil {
	log.Fatal(err)
}
defer client.Close()

sub, err := client.Subscribe("RPCService.log_subscribe", nil)
if err != nil {
	log.Fatal(err)
}
log.Infof("Subscribed successfully: %d\n", sub.ID)

go func() {
	for notification := range sub.Recv() {
		log.Infof("Receive new notification: %s\n", notification)
	}
}()

_, err := client.Call("RPCService.ping", nil)
if err != nil {
	log.Fatal(err)
}
```


