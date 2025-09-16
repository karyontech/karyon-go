package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	rpc "github.com/karyontech/karyon-go/jsonrpc/client"
)

func main() {

	config := rpc.RPCClientConfig{
		// Addr: "ws://localhost:7000/",
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
