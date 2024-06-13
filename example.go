package main

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"time"

	rpc "github.com/karyontech/karyon-go/jsonrpc/client"
	log "github.com/sirupsen/logrus"
)

type Pong struct{}

func runNewClient() error {
	config := rpc.RPCClientConfig{
		Addr: "ws://127.0.0.1:6000",
	}

	client, err := rpc.NewRPCClient(config)
	if err != nil {
		return err
	}
	defer client.Close()

	subID, ch, err := client.Subscribe("Calc.log_subscribe", nil)
	if err != nil {
		return err
	}
	log.Infof("Subscribed successfully: %d\n", subID)

	go func() {
		for notification := range ch {
			log.Infof("Receive new notification: %s\n", notification)
		}
	}()

	for {
		millisecond := rand.IntN(2000-500) + 500
		time.Sleep(time.Duration(millisecond) * time.Millisecond)
		result, err := client.Call("Calc.ping", nil)
		if err != nil {
			return err
		}

		pongMsg := Pong{}
		err = json.Unmarshal(*result, &pongMsg)
		if err != nil {
			return err
		}
	}

}

func main() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		lvl = "debug"
	}
	ll, err := log.ParseLevel(lvl)
	if err != nil {
		ll = log.DebugLevel
	}
	log.SetLevel(ll)

	err = runNewClient()
	if err != nil {
		log.Fatal(err)
	}
}
