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

// Package counter defines a byte-counting writer.  One use case is measuring the size of content being streamed into CAS.
package counter

import (
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	bodyIn := "Hello, World!"
	counter := &Counter{}
	reader := strings.NewReader(bodyIn)
	countedReader := io.TeeReader(reader, counter)
	bodyOut, err := ioutil.ReadAll(countedReader)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, bodyIn, string(bodyOut))
	assert.Equal(t, uint64(len(bodyIn)), counter.Count())
}
