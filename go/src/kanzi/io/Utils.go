/*
Copyright 2011-2013 Frederic Langlet
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

package io

import (
	"bufio"
	"os"
)

// Simple wrapper around File to add buffered read/write and implement
// kanzi.InputStream & kanzi.OutputStream
type BufferedOutputStream struct {
	file   *os.File
	writer *bufio.Writer
}

func NewBufferedOutputStream(file *os.File) (*BufferedOutputStream, error) {
	bos := new(BufferedOutputStream)
	bos.file = file
	bos.writer = bufio.NewWriter(file)
	return bos, nil
}

func (this *BufferedOutputStream) Write(b []byte) (n int, err error) {
	return this.writer.Write(b)
}

func (this *BufferedOutputStream) Close() error {
	if err := this.writer.Flush(); err != nil {
		return err
	}

	return this.file.Close()
}

type BufferedInputStream struct {
	file   *os.File
	reader *bufio.Reader
}

func NewBufferedInputStream(file *os.File) (*BufferedInputStream, error) {
	bis := new(BufferedInputStream)
	bis.file = file
	bis.reader = bufio.NewReader(file)
	return bis, nil
}

func (this *BufferedInputStream) Read(b []byte) (n int, err error) {
	return this.reader.Read(b)
}

func (this *BufferedInputStream) Close() error {
	return this.file.Close()
}
