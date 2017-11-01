Build Kanzi
===========

cd src/kanzi/app


Build Kanzi.go (compressor/decompressor):

go build -gcflags=-B Kanzi.go BlockCompressor.go BlockDecompressor.go InfoPrinter.go

