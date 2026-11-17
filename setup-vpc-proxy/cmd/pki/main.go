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

package main

import (
	"flag"
	"log"

	"go.ate.dev/substrate/setup-vpc-proxy/pkg/devpki"
)

func main() {
	dir := flag.String("out", "setup-vpc-proxy/.work/pki", "")
	negative := flag.String("negative-clients", "", "Directory for temporary negative client fixtures")
	check := flag.Bool("check", false, "Verify existing development PKI")
	flag.Parse()
	if *check {
		if err := devpki.Check(*dir); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *negative != "" {
		if err := devpki.NegativeClients(*dir, *negative); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := devpki.Generate(*dir); err != nil {
		log.Fatal(err)
	}
}
