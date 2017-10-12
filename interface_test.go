// Copyright 2017 casengine contributors
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

package casengine

import (
	_ "crypto/sha256"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
)

func TestStreamingValidationGood(t *testing.T) {
	bodyIn := "Hello, World!"
	rawReader := strings.NewReader(bodyIn)
	digest, err := digest.Parse("sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f")
	if err != nil {
		t.Fatal(err)
	}

	verifier := digest.Verifier()
	verifiedReader := io.TeeReader(rawReader, verifier)
	bodyOut, err := ioutil.ReadAll(verifiedReader)
	if err != nil {
		t.Fatal(err)
	}

	if !verifier.Verified() {
		t.Fatalf("failed to verify %q for %s", bodyIn, digest)
	}

	assert.Equal(t, bodyIn, string(bodyOut))
}

func TestStreamingValidationBad(t *testing.T) {
	bodyIn := "Hello, World!"
	rawReader := strings.NewReader(bodyIn)
	digest, err := digest.Parse("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	if err != nil {
		t.Fatal(err)
	}

	verifier := digest.Verifier()
	verifiedReader := io.TeeReader(rawReader, verifier)
	bodyOut, err := ioutil.ReadAll(verifiedReader)
	if err != nil {
		t.Fatal(err)
	}

	if verifier.Verified() {
		t.Fatalf("incorrectly verified %q for %s", bodyIn, digest)
	}

	assert.Equal(t, bodyIn, string(bodyOut))
}
