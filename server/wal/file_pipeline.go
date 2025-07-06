// Copyright 2016 The etcd Authors
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

package wal

import (
	"fmt"
	"os"
	"path/filepath"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"

	"go.uber.org/zap"
)

// filePipeline pipelines allocating disk space
// 它负责预创建日志文件并为日志文件预分配空间。在 filePipeline 中会启动一个独立的后台 goroutine 来创建".tmp"结尾的临时文件
// 当进行日志文件切换时，直接将临时文件进行重命名即可使用
type filePipeline struct {
	lg *zap.Logger

	// dir to put files
	// 存放临时文件的目录。
	dir string
	// size of files to make, in bytes
	// 创建临时文件时预分配空间的大小，默认是 64MB 由 wal.SegmentSizeBytes 指定，该值也是每个日志文件的大小
	size int64
	// count number of files generated
	// 当前 filePipeline 实例创建的临时文件数。
	count int

	// 新建的 临时文件句柄会通过 filec 通道返回给 WAL 实例使用 。
	filec chan *fileutil.LockedFile

	// 当创建临时文件出现异常时，则将异常传递到 errc 通道中。
	errc chan error

	// 当 filePipeline.Close()被调用时会关闭 donec 通道，从而通知 filePipeline实例删除最后一次创建的临时文件。
	donec chan struct{}
}

func newFilePipeline(lg *zap.Logger, dir string, fileSize int64) *filePipeline {
	if lg == nil {
		lg = zap.NewNop()
	}
	fp := &filePipeline{
		lg:    lg,
		dir:   dir,
		size:  fileSize,
		filec: make(chan *fileutil.LockedFile),
		errc:  make(chan error, 1),
		donec: make(chan struct{}),
	}
	go fp.run()
	return fp
}

// Open returns a fresh file for writing. Rename the file before calling
// Open again or there will be file collisions.
func (fp *filePipeline) Open() (f *fileutil.LockedFile, err error) {
	select {
	case f = <-fp.filec: // 从 filec 通道中获取已经创建好的临时文件，并返回
	case err = <-fp.errc: // 如果创建临时文件时的异常，则通过 errc 通道 中获取，并返回
	}
	return f, err
}

func (fp *filePipeline) Close() error {
	close(fp.donec)
	return <-fp.errc
}

func (fp *filePipeline) alloc() (f *fileutil.LockedFile, err error) {
	// count % 2 so this file isn't the same as the one last published
	// 为了防止与前一个创建的临时文件重名，新建临时文件的编号是 0 或是 1
	fpath := filepath.Join(fp.dir, fmt.Sprintf("%d.tmp", fp.count%2))
	// 创建临时文件，注意文件的模式和权限
	if f, err = fileutil.LockFile(fpath, os.O_CREATE|os.O_WRONLY, fileutil.PrivateFileMode); err != nil {
		return nil, err
	}

	//  尝试预分自己空间，如果当前系统不支持预分配文件空间，则并不会报错
	if err = fileutil.Preallocate(f.File, fp.size, true); err != nil {
		fp.lg.Error("failed to preallocate space when creating a new WAL", zap.Int64("size", fp.size), zap.Error(err))
		f.Close()
		return nil, err
	}

	// 递增创建的文件数量
	fp.count++
	return f, nil
}

// 创建新的临时文件并将其句柄传递到 filec 通道中
func (fp *filePipeline) run() {
	defer close(fp.errc)
	for {
		f, err := fp.alloc()
		if err != nil {
			fp.errc <- err
			return
		}
		select {
		case fp.filec <- f:
		case <-fp.donec:
			os.Remove(f.Name())
			f.Close()
			return
		}
	}
}
