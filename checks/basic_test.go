// Copyright 2017 by the contributors.
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

package checks_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/karolhrdina/healthcheck/checks"
)

func TestTCPDialCheck(t *testing.T) {
	assert.NoError(t, checks.TCPDialCheck("globalwebindex.com:80", 5*time.Second)())
	assert.Error(t, checks.TCPDialCheck("heptio.com:25327", 5*time.Second)())
}

func TestHTTPGetCheck(t *testing.T) {
	assert.NoError(t, checks.HTTPGetCheck("https://www.globalwebindex.com", 5*time.Second)())
	assert.Error(t, checks.HTTPGetCheck("http://globalwebindex.com", 5*time.Second)(), "redirect should fail")
	assert.Error(t, checks.HTTPGetCheck("https://heptio.com/nonexistent", 5*time.Second)(), "404 should fail")
}

func TestDNSResolveCheck(t *testing.T) {
	assert.NoError(t, checks.DNSResolveCheck("globalwebindex.com", 5*time.Second)())
	assert.Error(t, checks.DNSResolveCheck("nonexistent.heptio.com", 5*time.Second)())
}

func TestGoroutineCountCheck(t *testing.T) {
	assert.NoError(t, checks.GoroutineCountCheck(1000)())
	assert.Error(t, checks.GoroutineCountCheck(0)())
}
