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
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/at-wat/mqtt-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("   usage: %s server-host.domain\n", os.Args[0])
		fmt.Printf("requires: certificate.crt, private.key, root-CA.crt\n")
		os.Exit(1)
	}
	host := os.Args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	tlsConfig, err := newTLSConfig(
		host,
		"root-CA.crt",
		"certificate.crt",
		"private.key",
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	println("Connecting to", host)

	cli, err := mqtt.NewReconnectClient(ctx,
		// Dialer to connect/reconnect to the server.
		mqtt.DialerFunc(func() (mqtt.ClientCloser, error) {
			cli, err := mqtt.Dial(
				fmt.Sprintf("mqtts://%s:8883", host),
				mqtt.WithTLSConfig(tlsConfig),
			)
			if err != nil {
				return nil, err
			}
			// Register ConnState callback to low level client
			cli.ConnState = func(s mqtt.ConnState, err error) {
				fmt.Printf("State changed to %s (err: %v)\n", s, err)
			}
			return cli, nil
		}),
		"sample", // Client ID
		mqtt.WithConnectOption(
			mqtt.WithKeepAlive(30),
			mqtt.WithWill(
				&mqtt.Message{
					Topic:   "test",
					QoS:     mqtt.QoS1,
					Payload: []byte("{\"message\": \"Bye\"}"),
				},
			),
		),
		mqtt.WithPingInterval(10*time.Second),
		mqtt.WithTimeout(5*time.Second),
		mqtt.WithReconnectWait(1*time.Second, 15*time.Second),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	mux := &mqtt.ServeMux{}
	cli.Handle(mux) // Register muxer as a low level handler.

	mux.Handle("#", // Handle all topics by this handler.
		mqtt.HandlerFunc(func(msg *mqtt.Message) {
			fmt.Printf("Wildcard (%s): %s (QoS: %d)\n", msg.Topic, []byte(msg.Payload), int(msg.QoS))
		}),
	)
	mux.Handle("stop", // Handle 'stop' topic by this handler.
		mqtt.HandlerFunc(func(msg *mqtt.Message) {
			fmt.Printf("Stop: %s (QoS: %d)\n", []byte(msg.Payload), int(msg.QoS))
			cancel()
		}),
	)

	// Subscribe two topics.
	if err := cli.Subscribe(ctx,
		mqtt.Subscription{
			Topic: "test",
			QoS:   mqtt.QoS1,
		},
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

// newTLSConfig creates TLS configuration with client certificates.
func newTLSConfig(host, caFile, certFile, privateKeyFile string) (*tls.Config, error) {
	certpool := x509.NewCertPool()
	cas, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	certpool.AppendCertsFromPEM(cas)

	cert, err := tls.LoadX509KeyPair(certFile, privateKeyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		ServerName:   host,
		RootCAs:      certpool,
		Certificates: []tls.Certificate{cert},
	}, nil
}
