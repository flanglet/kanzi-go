kanzi
=====


Java &amp; Go code for manipulation and compression of data and images.



= Overview=

This project offers Java & Go code for manipulation and compression of data and images.
The goal is to provide clean APIs and really fast implementation.
It includes lossless compression codecs (Huffman, Range, LZ4, Snappy, PAQ), color model transforms, resampling, wavelet, DCT, Hadamard transform, bit stream manipulation, Burrows-Wheeler (BWT), Distance Coding and Move-To-Front transform, run length coding, etc ...
It also provides video filters including fast Gaussian filter, Sobel filter and constant time bilateral filter.

== Package hierarchy ==

                      
  * kanzi: top level including common classes and interfaces
  * app contains applications (E.G. block compressor)
  * bitstream: utilities to manipulate a stream of data at the bit level
  * entropy: implementation of several common entropy codecs (process bits)
  * filter: pre/post processing filter on image data
  * filter/seam: a filter that allows context based image resizing
  * function: implementation of common functions (input and output sizes differ but process only bytes): RLT, ZLT, LZ4, Snappy
  * function/wavelet: utility functions for Wavelet transforms
  * io: implementation of InputStream, OutputStream with block codec
  * test: contains many classes to test the utility classes
  * transform: implementation of common functions (input and output sizes are identical) such as Wavelet, Discrete Cosine, Walsh-Hadamard, Burrows-Wheeler
  * util: misc. utility classes, MurMurHash, xxHash           
  * util/color: color space mapping utilities (RGB and several YUV implementations)
  * util/sampling: implementation of several fast image resampling classes
  * util/sort: implementation of the most common sorting algorithms: QuickSort, Radix, MergeSort, BucketSort, etc...
           
There are no static dependencies to other jar files but jna.jar can be provided in case video filters are implemented via JNI calls.

Java 7 is required (only for kanzi.test.TestSort and kanzi.util.sort.ForkJoinParallelSort). 
           


== Block compressor examples ==

How to use the block compressor/decompressor from the command line:

To compress, use kanzi.app.BlockCompressor / BlockCompressor.go

To decompress, use kanzi.app.BlockDecompressor / BlockDecompressor.go

The Block compressor cuts the input file into chunks of 100KB (or the size provided on the command line with the 'block' option). Optionally, a checksum for the chunk of data can be computed and stored in the output.

As a first step, it applies a transform (default is block codec) to turn the block into a smaller number of bytes (byte transform). As a second step, it applies an entropy coder (to turn the block into a smaller number of bits).

Each step can be bypassed based on command line options. 

The decompressor extracts all necessary information from the header of the bitstream (input file) such as entropy type, transform type, block size, checksum enabled/disabled, etc... before applying appropriate entropy decoder followed by the inverse transform for each block. Optionally, a checksum is computed and checked against the one stored in the bitstream (based on original data).

The 2 step process allows very fast compression/decompression (Snappy/LZ4+no entropy or Snappy/LZ4+Huffman) or high compression ratio (block transform + PAQ or FPAQ + block size > 1MB). 

See some examples below:

java -cp kanzi.jar kanzi.app.BlockCompressor -help
-help                : display this message
-verbose             : display the block size at each stage (in bytes, floor rounding if fractional)
-silent              : silent mode, no output (except warnings and errors)
-overwrite           : overwrite the output file if it already exists
-input=<inputName>   : mandatory name of the input file to encode
-output=<outputName> : optional name of the output file (defaults to <input.knz>)
-block=<size>        : size of the input blocks (max 16MB - 4 / min 1KB / default 1MB)
-entropy=            : entropy codec to use [None|Huffman*|Range|PAQ|FPAQ]
-transform=          : transform to use [None|Block*|Snappy|LZ4|RLT]
-checksum            : enable block checksum
-jobs=<jobs>         : number of parallel jobs


java -cp kanzi.jar kanzi.app.BlockDecompressor -help
-help                : display this message
-verbose             : display the block size at each stage (in bytes, floor rounding if fractional)
-overwrite           : overwrite the output file if it already exists
-silent              : silent mode, no output (except warnings and errors)
-input=<inputName>   : mandatory name of the input file to decode
-output=<outputName> : optional name of the output file
-jobs=<jobs>         : number of parallel jobs

*Testing the compressor*


java -cp kanzi.jar kanzi.app.BlockCompressor -input=c:\temp\rt.jar -output=c:\temp\rt.knz -overwrite -block=4000000 -transform=block -entropy=huffman
Encoding ...

Encoding:          8270 ms
Input size:        60008624
Output size:       16176375
Ratio:             0.2695675
Throughput (KB/s): 7086


With block transform and FPAQ codec, no checksum and block of 4000000 bytes. Slower but better compression ratio

java -cp kanzi.jar kanzi.app.BlockCompressor -input=c:\temp\rt.jar -output=c:\temp\rt.knz -overwrite -block=4000000 -transform=block -entropy=fpaq
Encoding ...

Encoding:          10259 ms
Input size:        60008624
Output size:       15528498
Ratio:             0.2587711
Throughput (KB/s): 5712


With LZ4 transform and Huffman codec, no checksum and block of 1 MB. Lower compression ratio but very fast

java -cp kanzi.jar kanzi.app.BlockCompressor -input=c:\temp\rt.jar -output=c:\temp\rt.knz -overwrite -block=1M -transform=lz4 -entropy=huffman
Encoding ...

Encoding:          869 ms
Input size:        60008624
Output size:       23452848
Ratio:             0.39082462
Throughput (KB/s): 67436


With Snappy transform and Huffman codec, checksum, verbose and block of 4 MB. Using the Go version.

go run BlockCompressor.go -input=c:\temp\rt.jar -output=c:\temp\rt.knz -overwrite -block=4M -checksum -entropy=huffman -transform=snappy
ntropy codec (stage 2)
Encoding ...

Encoding:          1506 ms
Input size:        60008624
Output size:       24286590
Ratio:             0.404718
Throughput (KB/s): 38912


Decode (Go then Java):

go run BlockDecompressor.go -input=c:\temp\rt.knz -output=c:\temp\rt.jar -overwrite -verbose
Decoding ...

Decoding:          1722 ms
Input size:        24286590
Output size:       60008624
Throughput (KB/s): 34031

java -cp kanzi.jar kanzi.app.BlockDecompressor -input=c:\temp\rt.knz -output=c:\temp\rt.jar -overwrite -verbose 
Decoding ...

Decoding:          1041 ms
Input size:        24286590
Output size:       60008624
Throughput (KB/s): 56294


More details are available at https://code.google.com/p/kanzi/
