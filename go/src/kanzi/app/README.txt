# Build Compressor
go build BlockCompressor.go BlockCompressorTask.go 
# Build Decompressor
go build BlockDecompressor.go BlockDecompressorTask.go 
# Build Codec
go build Kanzi.go BlockCompressorTask.go BlockDecompressorTask.go 