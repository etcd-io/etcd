// Copyright 2022 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_readRaw(t *testing.T) {
	path := t.TempDir()
	mustCreateWALLog(t, path)
	var out bytes.Buffer
	readRaw(nil, walDir(path), &out)
	assert.Equal(t,
		`CRC: 0
Metadata: 
Snapshot: 
Entry: Term:1 Index:1 Type:EntryConfChange Data:"\010\001\020\000\030\002\"\000" 
Entry: Term:2 Index:2 Type:EntryConfChange Data:"\010\002\020\001\030\002\"\000" 
Entry: Term:2 Index:3 Type:EntryConfChange Data:"\010\003\020\002\030\002\"\000" 
Entry: Term:2 Index:4 Type:EntryConfChange Data:"\010\004\020\003\030\003\"\000" 
Entry: Term:3 Index:5 Data:"\010\000\022\000\032\006/path0\"\030{\"hey\":\"ho\",\"hi\":[\"yo\"]}(\0012\0008\000@\000H\tP\000X\001`+"`"+`\000h\000p\000x\001\200\001\000\210\001\000" 
Entry: Term:3 Index:6 Data:"\010\001\022\004QGET\032\006/path1\"\023{\"0\":\"1\",\"2\":[\"3\"]}(\0002\0008\000@\000H\tP\000X\001`+"`"+`\000h\000p\000x\001\200\001\000\210\001\000" 
Entry: Term:3 Index:7 Data:"\010\002\022\004SYNC\032\006/path2\"\023{\"0\":\"1\",\"2\":[\"3\"]}(\0002\0008\000@\000H\002P\000X\001`+"`"+`\000h\000p\000x\001\200\001\000\210\001\000" 
Entry: Term:3 Index:8 Data:"\010\003\022\006DELETE\032\006/path3\"\030{\"hey\":\"ho\",\"hi\":[\"yo\"]}(\0002\0008\000@\001H\002P\000X\001`+"`"+`\000h\000p\000x\001\200\001\000\210\001\000" 
Entry: Term:3 Index:9 Data:"\010\004\022\006RANDOM\032\246\001/path4/superlong/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path/path\"\030{\"hey\":\"ho\",\"hi\":[\"yo\"]}(\0002\0008\000@\000H\002P\000X\001`+"`"+`\000h\000p\000x\001\200\001\000\210\001\000" 
Entry: Term:4 Index:10 Data:"\010\005\032\025\n\0011\022\002hi\030\006 \001(\001X\240\234\001h\240\234\001" 
Entry: Term:5 Index:11 Data:"\010\006\"\020\n\004foo1\022\004bar1\030\0010\001" 
Entry: Term:6 Index:12 Data:"\010\007*\010\n\0010\022\0019\030\001" 
Entry: Term:7 Index:13 Data:"\010\0102\024\022\010\032\006\n\001a\022\001b\032\010\032\006\n\001a\022\001b" 
Entry: Term:8 Index:14 Data:"\010\t:\002\020\001" 
Entry: Term:9 Index:15 Data:"\010\nB\004\010\001\020\001" 
Entry: Term:10 Index:16 Data:"\010\013J\002\010\002" 
Entry: Term:11 Index:17 Data:"\010\014R\006\010\003\020\004\030\005" 
Entry: Term:12 Index:18 Data:"\010\r\302>\000" 
Entry: Term:13 Index:19 Data:"\010\016\232?\000" 
Entry: Term:14 Index:20 Data:"\010\017\242?\031\n\006myname\022\010password\032\005token" 
Entry: Term:15 Index:21 Data:"\010\020\342D\020\n\005name1\022\005pass1\032\000" 
Entry: Term:16 Index:22 Data:"\010\021\352D\007\n\005name1" 
Entry: Term:17 Index:23 Data:"\010\022\362D\007\n\005name1" 
Entry: Term:18 Index:24 Data:"\010\023\372D\016\n\005name1\022\005pass2" 
Entry: Term:19 Index:25 Data:"\010\024\202E\016\n\005user1\022\005role1" 
Entry: Term:20 Index:26 Data:"\010\025\212E\016\n\005user2\022\005role2" 
Entry: Term:21 Index:27 Data:"\010\026\222E\000" 
Entry: Term:22 Index:28 Data:"\010\027\232E\000" 
Entry: Term:23 Index:29 Data:"\010\030\202K\007\n\005role2" 
Entry: Term:24 Index:30 Data:"\010\031\212K\007\n\005role1" 
Entry: Term:25 Index:31 Data:"\010\032\222K\007\n\005role3" 
Entry: Term:26 Index:32 Data:"\010\033\232K\033\n\005role3\022\022\010\001\022\004Keys\032\010RangeEnd" 
Entry: Term:27 Index:33 Data:"\010\034\242K\026\n\005role3\022\003key\032\010rangeend" 
Entry: Term:27 Index:34 Data:"?" 
EOF: All entries were processed.
`, out.String())
}
