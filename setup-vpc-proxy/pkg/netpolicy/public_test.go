// Copyright 2026 The Agent Substrate Authors
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

package netpolicy

import (
	"net/netip"
	"testing"
)

func TestPublicTarget(t *testing.T) {
	for _, s := range []string{"0.1.2.3", "10.1.2.3", "100.64.0.1", "127.0.0.1", "169.254.169.254", "172.18.0.1", "192.0.0.9", "192.0.2.1", "192.88.99.1", "192.168.1.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "224.1.1.1", "255.255.255.255", "::ffff:10.1.1.1", "::1", "2606:4700::1111"} {
		t.Run(s, func(t *testing.T) {
			if CheckPublicTarget(netip.MustParseAddr(s), 443) == nil {
				t.Fatal("allowed")
			}
		})
	}
	for _, p := range []uint16{0, 22, 8443, 65535} {
		if CheckPublicTarget(netip.MustParseAddr("1.1.1.1"), p) == nil {
			t.Fatal(p)
		}
	}
	for _, s := range []string{"1.1.1.1", "93.184.216.34", "::ffff:1.1.1.1"} {
		if e := CheckPublicTarget(netip.MustParseAddr(s), 443); e != nil {
			t.Fatal(e)
		}
	}
}
