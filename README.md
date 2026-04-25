
# karyon-jsonrpc-go

A Go client for the [karyon](https://github.com/karyontech/karyon) JSON-RPC 2.0
server (Rust).

Currently the `tcp://` and `tls://` transports are supported. Other
transports (`ws`, `wss`, `unix`, `quic`, `http`) supported by the Rust
implementation will be added later.

For `tls://`, set `RPCClientConfig.TLSConfig` to a `*tls.Config`
(passing `nil` uses the system roots and verifies the hostname).

## Install

```sh
    go get github.com/karyontech/karyon-jsonrpc-go
```

## Example 

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	rpc "github.com/karyontech/karyon-jsonrpc-go/jsonrpc/client"
)

func main() {

	config := rpc.RPCClientConfig{
		Addr: "tcp://localhost:7000/",
	}

	client, err := rpc.NewRPCClient(config)
	if err != nil {
		slog.Error("Create a new rpc client", "error", err)
		os.Exit(1)
	}

	defer client.Close()

	sub, err := client.Subscribe("Calc.log_subscribe", nil)
	if err != nil {
		slog.Error("Subscribe to log_subscribe", "error", err)
		os.Exit(1)
	}
	fmt.Printf("Subscribed successfully: %d\n", sub.ID)

	go func() {
		for notification := range sub.Recv() {
			fmt.Printf("Receive a new notification: %s\n", notification)
		}
	}()

	time.Sleep(time.Second * 10)

	msg, err := client.Call("Calc.ping", nil)
	if err != nil {
		slog.Error("Call ping method", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Receive a pong msg: %s", msg)

}
```

## License

This project is licensed under the MIT License. See the
[LICENSE](https://github.com/karyontech/karyon-jsonrpc-go/blob/master/LICENSE) file for
details.

## Contributions

Contributions are welcome! Please open an issue or submit a pull request.
