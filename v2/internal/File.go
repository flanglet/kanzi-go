/*
Copyright 2011-2025 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	pathSeparator = string([]byte{os.PathSeparator})
)

// FileData a basic structure encapsulating a file path and size
type FileData struct {
	FullPath string
	Path     string
	Name     string
	Size     int64
}

// NewFileData creates an instance of FileData from a file path and size
func NewFileData(fullPath string, size int64) *FileData {
	this := &FileData{}
	this.FullPath = fullPath
	this.Size = size
	this.Path, this.Name = filepath.Split(fullPath)
	return this
}

// FileCompare a structure used to sort files by path and size
type FileCompare struct {
	data       []FileData
	sortBySize bool
}

func NewFileCompare(data []FileData, sortBySize bool) *FileCompare {
	this := &FileCompare{}
	this.data = data
	this.sortBySize = sortBySize
	return this
}

// Len returns the size of the internal file data buffer
func (this FileCompare) Len() int {
	return len(this.data)
}

// Swap swaps two file data in the internal buffer
func (this FileCompare) Swap(i, j int) {
	this.data[i], this.data[j] = this.data[j], this.data[i]
}

// Less returns true if the path at index i in the internal
// file data buffer is less than file data buffer at index j.
// The order is defined by lexical order of the parent directory
// path then file size.
func (this FileCompare) Less(i, j int) bool {
	if this.sortBySize == false {
		return strings.Compare(this.data[i].FullPath, this.data[j].FullPath) < 0
	}

	// First compare parent directory paths
	res := strings.Compare(this.data[i].Path, this.data[j].Path)

	if res != 0 {
		return res < 0
	}

	// Then, compare file sizes (decreasing order)
	return this.data[i].Size > this.data[j].Size
}

func CreateFileList(target string, fileList []FileData, isRecursive, ignoreLinks, ignoreDotFiles bool) ([]FileData, error) {
	fi, err := os.Stat(target)

	if err != nil {
		return fileList, err
	}

	if ignoreDotFiles == true {
		shortName := target

		if len(shortName) > 1 {
			if idx := strings.LastIndex(shortName, pathSeparator); idx > 0 {
				shortName = shortName[idx+1:]
			}

			if shortName[0] == '.' {
				return fileList, nil
			}
		}
	}

	if fi.Mode().IsRegular() || ((ignoreLinks == false) && (fi.Mode()&fs.ModeSymlink != 0)) {
		fileList = append(fileList, *NewFileData(target, fi.Size()))
		return fileList, nil
	}

	if isRecursive {
		if target[len(target)-1] != os.PathSeparator {
			target = target + pathSeparator
		}

		err = filepath.Walk(target, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if ignoreDotFiles == true {
				shortName := path

				if idx := strings.LastIndex(shortName, pathSeparator); idx > 0 {
					shortName = shortName[idx+1:]
				}

				if len(shortName) > 0 && shortName[0] == '.' {
					return nil
				}
			}

			if fi.Mode().IsRegular() || ((ignoreLinks == false) && (fi.Mode()&fs.ModeSymlink != 0)) {
				fileList = append(fileList, *NewFileData(path, fi.Size()))
			}

			return err
		})
	} else {
		var files []fs.DirEntry
		files, err = os.ReadDir(target)

		if err == nil {
			for _, de := range files {
				if de.Type().IsRegular() {
					var fi fs.FileInfo

					if fi, err = de.Info(); err != nil {
						break
					}

					if ignoreDotFiles == true {
						shortName := de.Name()

						if idx := strings.LastIndex(shortName, pathSeparator); idx > 0 {
							shortName = shortName[idx+1:]
						}

						if len(shortName) > 0 && shortName[0] == '.' {
							continue
						}
					}

					if fi.Mode().IsRegular() || ((ignoreLinks == false) && (fi.Mode()&fs.ModeSymlink != 0)) {
						fileList = append(fileList, *NewFileData(target+de.Name(), fi.Size()))
					}
				}
			}
		}
	}

	return fileList, err
}

func IsReservedName(fileName string) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	// Sorted list
	var reserved = []string{"AUX", "COM0", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6",
		"COM7", "COM8", "COM9", "COM¹", "COM²", "COM³", "CON", "LPT0", "LPT1", "LPT2",
		"LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9", "NUL", "PRN"}

	for _, r := range reserved {
		res := strings.Compare(fileName, r)

		if res == 0 {
			return true
		}

		if res < 0 {
			break
		}
	}

	return false
}
