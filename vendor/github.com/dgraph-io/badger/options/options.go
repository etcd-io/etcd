/*
 * Copyright 2017 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package options

// FileLoadingMode specifies how data in LSM table files and value log files should
// be loaded.
type FileLoadingMode int

const (
	// FileIO indicates that files must be loaded using standard I/O
	FileIO FileLoadingMode = iota
	// LoadToRAM indicates that file must be loaded into RAM
	LoadToRAM
	// MemoryMap indicates that that the file must be memory-mapped
	MemoryMap
)
