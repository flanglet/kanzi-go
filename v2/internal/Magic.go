/*
Copyright 2011-2024 Frederic Langlet
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
	"encoding/binary"
)

const (
	NO_MAGIC     = 0
	JPG_MAGIC    = 0xFFD8FFE0
	GIF_MAGIC    = 0x47494638
	PDF_MAGIC    = 0x25504446
	ZIP_MAGIC    = 0x504B0304 // Works for jar & office docs
	LZMA_MAGIC   = 0x377ABCAF // Works for 7z  37 7A BC AF 27 1C
	PNG_MAGIC    = 0x89504E47
	ELF_MAGIC    = 0x7F454C46
	MAC_MAGIC32  = 0xFEEDFACE
	MAC_CIGAM32  = 0xCEFAEDFE
	MAC_MAGIC64  = 0xFEEDFACF
	MAC_CIGAM64  = 0xCFFAEDFE
	ZSTD_MAGIC   = 0x28B52FFD
	BROTLI_MAGIC = 0x81CFB2CE
	RIFF_MAGIC   = 0x52494646 // WAV, AVI, WEBP
	CAB_MAGIC    = 0x4D534346
	FLAC_MAGIC   = 0x664C6143
	XZ_MAGIC     = 0xFD377A58 // FD 37 7A 58 5A 00
	RAR_MAGIC    = 0x52617221 // 42 61 72 21 1A 07 00
	KNZ_MAGIC    = 0x4B414E5A

	BZIP2_MAGIC   = 0x425A68
	MP3_ID3_MAGIC = 0x494433

	GZIP_MAGIC = 0x1F8B
	BMP_MAGIC  = 0x424D
	WIN_MAGIC  = 0x4D5A
	PBM_MAGIC  = 0x5034 // bin only
	PGM_MAGIC  = 0x5035 // bin only
	PPM_MAGIC  = 0x5036 // bin only

)

// Magic is a utility to detect common header magic values
type Magic struct {
}

var (
	_KEYS32 = [18]uint{
		GIF_MAGIC, PDF_MAGIC, ZIP_MAGIC, LZMA_MAGIC, PNG_MAGIC,
		ELF_MAGIC, MAC_MAGIC32, MAC_CIGAM32, MAC_MAGIC64, MAC_CIGAM64,
		ZSTD_MAGIC, BROTLI_MAGIC, CAB_MAGIC, RIFF_MAGIC, FLAC_MAGIC,
		XZ_MAGIC, KNZ_MAGIC, RAR_MAGIC,
	}

	_KEYS16 = [3]uint{
		GZIP_MAGIC, BMP_MAGIC, WIN_MAGIC,
	}
)

// GetMagicType checks the first bytes of the slice against a list of common magic values
func GetMagicType(src []byte) uint {
	if len(src) < 4 {
		return NO_MAGIC
	}

	key := uint(binary.BigEndian.Uint32(src))

	if (key & ^uint(0x0F)) == JPG_MAGIC {
		return key
	}

	if ((key >> 8) == BZIP2_MAGIC) || ((key >> 8) == MP3_ID3_MAGIC) {
		return key >> 8
	}

	for _, k := range _KEYS32 {
		if key == k {
			return key
		}
	}

	key16 := key >> 16

	for _, k := range _KEYS16 {
		if key16 == k {
			return key16
		}
	}

	if (key16 == PBM_MAGIC) || (key16 == PGM_MAGIC) || (key16 == PPM_MAGIC) {
		subkey := (key >> 8) & 0xFF

		if (subkey == 0x07) || (subkey == 0x0A) || (subkey == 0x0D) || (subkey == 0x20) {
			return key16
		}
	}

	return NO_MAGIC
}

// IsDataCompressed return true if the provided magic parameter corresponds
// to a known compressed data type.
func IsDataCompressed(magic uint) bool {
	switch magic {
	case JPG_MAGIC:
		return true
	case GIF_MAGIC:
		return true
	case PNG_MAGIC:
		return true
	//case RIFF_MAGIC: may or may not
	//	return true
	case LZMA_MAGIC:
		return true
	case ZSTD_MAGIC:
		return true
	case BROTLI_MAGIC:
		return true
	case CAB_MAGIC:
		return true
	case ZIP_MAGIC:
		return true
	case GZIP_MAGIC:
		return true
	case BZIP2_MAGIC:
		return true
	case FLAC_MAGIC:
		return true
	case MP3_ID3_MAGIC:
		return true
	case XZ_MAGIC:
		return true
	case KNZ_MAGIC:
		return true
	case RAR_MAGIC:
		return true
	default:
	}

	return false
}

// IsDataMultimedia return true if the provided magic parameter corresponds
// to a known multimedia data type.
func IsDataMultimedia(magic uint) bool {
	switch magic {
	case JPG_MAGIC:
		return true
	case GIF_MAGIC:
		return true
	case PNG_MAGIC:
		return true
	case RIFF_MAGIC:
		return true
	case FLAC_MAGIC:
		return true
	case MP3_ID3_MAGIC:
		return true
	case BMP_MAGIC:
		return true
	case PBM_MAGIC:
		return true
	case PGM_MAGIC:
		return true
	case PPM_MAGIC:
		return true
	default:
	}

	return false
}

// IsDataExecutable return true if the provided magic parameter corresponds
// to a known executable data type.
func IsDataExecutable(magic uint) bool {
	switch magic {
	case ELF_MAGIC:
		return true
	case WIN_MAGIC:
		return true
	case MAC_MAGIC32:
		return true
	case MAC_CIGAM32:
		return true
	case MAC_MAGIC64:
		return true
	case MAC_CIGAM64:
		return true
	default:
	}

	return false
}
