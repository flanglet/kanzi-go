Build Kanzi
===========

cd src/kanzi/app


Build Kanzi.go (compressor/decompressor)
go build -gcflags="-l=5" Kanzi.go BlockCompressorTask.go BlockDecompressorTask.go


Build Compressor only
go build -gcflags="-l=5" BlockCompressor.go BlockCompressorTask.go 


Build Decompressor only
go build -gcflags="-l=5" BlockDecompressor.go BlockDecompressorTask.go 
