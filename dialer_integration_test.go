//go:build integration
// +build integration

// Copyright 2021 The mqtt-go authors.
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

package mqtt

import (
	"context"
	"crypto/tls"
	"testing"
)

func TestIntegration_BaseClientStoreDialer(t *testing.T) {
	d := &BaseClientStoreDialer{
		Dialer: &URLDialer{
			URL: urls["MQTTs"],
			Options: []DialOption{
				WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
			},
		},
	}

	if cli := d.BaseClient(); cli != nil {
		t.Errorf("Initial BaseClient() is expected to be nil, got %p", cli)
	}

	ctx := context.TODO()

	cli1, err := d.DialContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cli1.Close()
	if cli := d.BaseClient(); cli != cli1 {
		t.Errorf("Expected BaseClient(): %p, got: %p", cli1, cli)
	}

	cli2, err := d.DialContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cli2.Close()
	if cli := d.BaseClient(); cli != cli2 {
		t.Errorf("Expected BaseClient(): %p, got: %p", cli2, cli)
	}
}
