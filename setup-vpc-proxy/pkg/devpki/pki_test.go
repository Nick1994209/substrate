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

package devpki

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndCheck(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pki")
	if e := Generate(dir); e != nil {
		t.Fatal(e)
	}
	if e := Check(dir); e != nil {
		t.Fatal(e)
	}
	first, e := os.ReadFile(filepath.Join(dir, "1", "ca.crt"))
	if e != nil {
		t.Fatal(e)
	}
	second, e := os.ReadFile(filepath.Join(dir, "2", "ca.crt"))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Equal(first, second) {
		t.Fatal("bindings share CA")
	}
	if e = Generate(dir); e == nil {
		t.Fatal("regenerated existing PKI")
	}
	again, e := os.ReadFile(filepath.Join(dir, "1", "ca.crt"))
	if e != nil || !bytes.Equal(first, again) {
		t.Fatal("existing CA changed", e)
	}
	wrong, e := os.ReadFile(filepath.Join(dir, "2", "client.key"))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "1", "client.key"), wrong, 0600); e != nil {
		t.Fatal(e)
	}
	if e = Check(dir); e == nil {
		t.Fatal("mismatched private key accepted")
	}
}
