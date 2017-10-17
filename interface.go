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

// Package casengine defines common interfaces for CAS engines.
package casengine

import (
	"io"

	"github.com/opencontainers/go-digest"
	"golang.org/x/net/context"
)

// Reader represents a content-addressable storage engine reader.
type Reader interface {

	// Get returns a reader for retrieving a blob from the store.
	// Returns os.ErrNotExist if the digest is not found.
	//
	// Implementations are *not* required to verify that the returned
	// reader content matches the requested digest.  Callers that need
	// that verification are encouraged to use something like:
	//
	//   rawReader, err := engine.Get(ctx, digest)
	//   defer rawReader.Close()
	//   verifier := digest.Verifier()
	//   verifiedReader := io.TeeReader(rawReader, verifier)
	//   consume(verifiedReader)
	//   if !verifier.Verified() {
	//     dieScreaming()
	//   }
	//
	// for streaming verification.
	Get(ctx context.Context, digest digest.Digest) (reader io.ReadCloser, err error)
}

// Closer represents a content-addressable storage engine closer.
type Closer interface {

	// Close releases resources held by the engine.  Subsequent engine
	// method calls will fail.
	Close(ctx context.Context) (err error)
}

// ReadCloser is the interface that groups the basic Reader and Closer
// interfaces.
type ReadCloser interface {
	Reader
	Closer
}
