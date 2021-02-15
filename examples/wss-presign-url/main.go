// Copyright 2019 The mqtt-go authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/at-wat/mqtt-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %s server-host.domain\n", os.Args[0])
		os.Exit(1)
	}
	host := os.Args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	println("Connecting to", host)

	cli, err := mqtt.NewReconnectClient(
		// Dialer to connect/reconnect to the server.
		mqtt.DialerFunc(func() (*mqtt.BaseClient, error) {
			// Presign URL here.
			url := fmt.Sprintf("wss://%s:9443?token=%x",
				host, time.Now().UnixNano(),
			)
			println("New URL:", url)
			return mqtt.Dial(url,
				mqtt.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
				mqtt.WithConnStateHandler(func(s mqtt.ConnState, err error) {
					fmt.Printf("State changed to %s (err: %v)\n", s, err)
				}),
			)
		}),
		mqtt.WithPingInterval(10*time.Second),
		mqtt.WithTimeout(5*time.Second),
		mqtt.WithReconnectWait(1*time.Second, 15*time.Second),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	_, err = cli.Connect(ctx,
		"sample", // Client ID
		mqtt.WithKeepAlive(30),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	mux := &mqtt.ServeMux{} // Multiplex message handlers by topic name.
	cli.Handle(mux)         // Register mux as a low-level handler.

	mux.Handle("#", // Handle all topics by this handler.
		mqtt.HandlerFunc(func(msg *mqtt.Message) {
			fmt.Printf("Received (%s): %s (QoS: %d)\n",
				msg.Topic, []byte(msg.Payload), int(msg.QoS),
			)
			cancel()
		}),
	)

	if _, err := cli.Subscribe(ctx,
		mqtt.Subscription{
			Topic: "stop",
			QoS:   mqtt.QoS1,
		},
	); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	println("Publishing one message to 'test' topic")

	if err := cli.Publish(ctx, &mqtt.Message{
		Topic:   "test",
		QoS:     mqtt.QoS1,
		Payload: []byte("{\"message\": \"Hello\"}"),
	}); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	println("Waiting message on 'stop' topic")
	<-ctx.Done()

	println("Disconnecting")

	if err := cli.Disconnect(ctx); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
