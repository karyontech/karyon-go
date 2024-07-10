
# Karyon-go

karyon jsonrpc client, written in go.

## Install

```sh
    go get github.com/karyontech/karyon-go 
```

## Example 

```go

import (
	rpc "github.com/karyontech/karyon-go/jsonrpc/client"
)

config := rpc.RPCClientConfig{
	Addr: "ws://127.0.0.1:6000",
}

client, err := rpc.NewRPCClient(config)
defer client.Close()
if err != nil {
	log.Fatal(err)
}

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

## License

This project is licensed under the GPL-3.0 License. See the
[LICENSE](https://github.com/karyontech/karyon-go/blob/master/LICENSE) file for
details. 

## Contributions

Contributions are welcome! Please open an issue or submit a pull request.
