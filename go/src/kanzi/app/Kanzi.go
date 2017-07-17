/*
Copyright 2011-2017 Frederic Langlet
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

package main

import (
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		firstArg := strings.ToUpper(os.Args[1])

		if firstArg == "--COMPRESS"  || firstArg == "-C" {
			os.Args = append(os.Args[:1], os.Args[2:]...)
			BlockCompressor_main()
			return
		} else if firstArg == "--DECOMPRESS" || firstArg == "-D"{
			os.Args = append(os.Args[:1], os.Args[2:]...)
			BlockDecompressor_main()
			return
		} else if firstArg == "--HELP" || firstArg == "-H" {
			print(os.Args[0])
			println(" --compress | --decompress | --help")
			return
		}
	}

	println("Missing arguments: try '--help'")

}
