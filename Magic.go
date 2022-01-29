/*
Copyright 2011-2021 Frederic Langlet
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

package kanzi

import (
	"encoding/binary"
)

const (
	NO_MAGIC     = 0
	JPG_MAGIC    = 0xFFD8FFE0
	GIF_MAGIC    = 0x47494638
	PDF_MAGIC    = 0x25504446
	ZIP_MAGIC    = 0x504B0304 // Works for jar & office docs
	LZMA_MAGIC   = 0x377ABCAF
	PNG_MAGIC    = 0x89504E47
	ELF_MAGIC    = 0x7F454C46
	MAC_MAGIC32  = 0xFEEDFACE
	MAC_CIGAM32  = 0xCEFAEDFE
	MAC_MAGIC64  = 0xFEEDFACF
	MAC_CIGAM64  = 0xCFFAEDFE
	ZSTD_MAGIC   = 0x28B52FFD
	BROTLI_MAGIC = 0x81CFB2CE
	RIFF_MAGIC   = 0x04524946
	CAB_MAGIC    = 0x4D534346
	GZIP_MAGIC   = 0x1F8B
	BMP_MAGIC    = 0x424D
	WIN_MAGIC    = 0x4D5A
)

// Magic is a utility to detect common header magic values
type Magic struct {
}

var (
	KEYS32 = [14]uint{
		GIF_MAGIC, PDF_MAGIC, ZIP_MAGIC, LZMA_MAGIC, PNG_MAGIC,
		ELF_MAGIC, MAC_MAGIC32, MAC_CIGAM32, MAC_MAGIC64, MAC_CIGAM64,
		ZSTD_MAGIC, BROTLI_MAGIC, CAB_MAGIC, RIFF_MAGIC,
	}

	KEYS16 = [3]uint{
		GZIP_MAGIC, BMP_MAGIC, WIN_MAGIC,
	}
)

// GetMagicType checks the first bytes of the slice against a list of common magic values
func GetMagicType(src []byte) uint {
	key := uint(binary.BigEndian.Uint32(src))

	if (key & ^uint(0x0F)) == JPG_MAGIC {
		return key
	}

	for _, k := range KEYS32 {
		if key == k {
			return key
		}
	}

	for _, k := range KEYS16 {
		if (key >> 16) == k {
			return key >> 16
		}
	}

	return NO_MAGIC
}
