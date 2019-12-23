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

	done := make(chan struct{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tlsConfig, err := newTLSConfig(
		host,
		"root-CA.crt",
		"certificate.crt",
		"private.key",
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	println("Connecting to", host)

	cli, err := mqtt.NewReconnectClient(ctx,
		&mqtt.URLDialer{
			URL: fmt.Sprintf("mqtts://%s:8883", host),
			Options: []mqtt.DialOption{
				mqtt.WithTLSConfig(tlsConfig),
			},
		},
		"sample",
		mqtt.WithKeepAlive(30),
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	println("Connected")

	cli.Handle(mqtt.HandlerFunc(func(msg *mqtt.Message) {
		fmt.Printf("%s[%d]: %s\n", msg.Topic, int(msg.QoS), []byte(msg.Payload))
		close(done)
	}))

	if err := cli.Subscribe(ctx, mqtt.Subscription{
		Topic: "test",
		QoS:   mqtt.QoS1,
	}); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	println("Publishing one message to 'test' topic")

	if err := cli.Publish(ctx, &mqtt.Message{
		Topic:   "test",
		QoS:     mqtt.QoS1,
		Payload: []byte("{\"message\": \"Hello\"}"),
	}); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	println("Waiting message on 'test' topic")
	<-done

	println("Disconnecting")

	if err := cli.Disconnect(ctx); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	println("Disconnected")
}

func newTLSConfig(host, caFile, crtFile, keyFile string) (*tls.Config, error) {
	certpool := x509.NewCertPool()
	cas, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	certpool.AppendCertsFromPEM(cas)

	cert, err := tls.LoadX509KeyPair(crtFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:            certpool,
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		InsecureSkipVerify: false,
		ServerName:         host,
		Certificates:       []tls.Certificate{cert},
	}, nil
}
