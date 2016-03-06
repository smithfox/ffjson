/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package jsonrt

import (
	"testing"
)

func TestWriteJsonNumber(t *testing.T) {
	buf := NewBuffer(nil)
	buf.AppendInt(-6, 10)

	if string(buf.Bytes()) != `-6` {
		t.Fatalf("Expected: %v\nGot: %v", `-6`, string(buf.Bytes()))
	}

	buf.Reset()
	buf.AppendUint(8, 10)
	if string(buf.Bytes()) != `8` {
		t.Fatalf("Expected: %v\nGot: %v", `8`, string(buf.Bytes()))
	}

	buf.Reset()
	buf.AppendFloat(-8.07, 'g', -1, 64)
	if string(buf.Bytes()) != `-8.07` {
		t.Fatalf("Expected: %v\nGot: %v", `-8.07`, string(buf.Bytes()))
	}
}
