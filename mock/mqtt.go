// Copyright 2020 The mqtt-go authors.
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

// Package mockmqtt provides simple standalone mock of mqtt.Client.
package mockmqtt

import (
	"context"

	"github.com/at-wat/mqtt-go"
)

// Client is a simple mock of mqtt.Client.
type Client struct {
	ConnectFn     func(ctx context.Context, clientID string, opts ...mqtt.ConnectOption) (sessionPresent bool, err error)
	DisconnectFn  func(ctx context.Context) error
	PublishFn     func(ctx context.Context, message *mqtt.Message) error
	SubscribeFn   func(ctx context.Context, subs ...mqtt.Subscription) error
	UnsubscribeFn func(ctx context.Context, subs ...string) error
	PingFn        func(ctx context.Context) error
	HandleFn      func(handler mqtt.Handler)
}

// Connect implements mqtt.Client.
func (c *Client) Connect(ctx context.Context, clientID string, opts ...mqtt.ConnectOption) (sessionPresent bool, err error) {
	return c.ConnectFn(ctx, clientID, opts...)
}

// Disconnect implements mqtt.Client.
func (c *Client) Disconnect(ctx context.Context) error {
	return c.DisconnectFn(ctx)
}

// Publish implements mqtt.Client.
func (c *Client) Publish(ctx context.Context, message *mqtt.Message) error {
	return c.PublishFn(ctx, message)
}

// Subscribe implements mqtt.Client.
func (c *Client) Subscribe(ctx context.Context, subs ...mqtt.Subscription) error {
	return c.SubscribeFn(ctx, subs...)
}

// Unsubscribe implements mqtt.Client.
func (c *Client) Unsubscribe(ctx context.Context, subs ...string) error {
	return c.UnsubscribeFn(ctx, subs...)
}

// Ping implements mqtt.Client.
func (c *Client) Ping(ctx context.Context) error {
	return c.PingFn(ctx)
}

// Handle implements mqtt.Client.
func (c *Client) Handle(handler mqtt.Handler) {
	c.HandleFn(handler)
}
