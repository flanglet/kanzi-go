kanzi
=====


Java &amp; Go code for manipulation and compression of data and images.



= Overview=

This project offers Java & Go code for manipulation and compression of data and images.
The goal is to provide clean APIs and really fast implementation.
It includes lossless compression codecs (Huffman, Range, LZ4, Snappy, PAQ), color model transforms, resampling, wavelet, DCT, Hadamard transform, bit stream manipulation, Burrows-Wheeler (BWT), Distance Coding and Move-To-Front transform, run length coding, etc ...
It also provides video filters including fast Gaussian filter, Sobel filter and constant time bilateral filter.

== Package hierarchy ==

kanzi
    * app
    * bitstream
    * entropy
    * filter
        * seam
    * function
        * wavelet
    * io
    * test
    * transform
    * util
        * color
        * sampling
        * sort
                      
                      
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

{{{
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
}}}

*Testing the compressor*

All tests performed on a desktop i7-2600 @3.40GHz, Win7, 16GB RAM with Oracle JDK7.

With block transform and Huffman codec, no checksum and block of 4000000 bytes

{{{
java -cp kanzi.jar kanzi.app.BlockCompressor -input=c:\temp\rt.jar -output=c:\temp\rt.knz -overwrite -block=4000000 -transform=block -entropy=huffman
Encoding ...

Encoding:          8270 ms
Input size:        60008624
Output size:       16176375
Ratio:             0.2695675
Throughput (KB/s): 7086
}}}

With block transform and FPAQ codec, no checksum and block of 4000000 bytes. Slower but better compression ratio

{{{
java -cp kanzi.jar kanzi.app.BlockCompressor -input=c:\temp\rt.jar -output=c:\temp\rt.knz -overwrite -block=4000000 -transform=block -entropy=fpaq
Encoding ...

Encoding:          10259 ms
Input size:        60008624
Output size:       15528498
Ratio:             0.2587711
Throughput (KB/s): 5712
}}}

With LZ4 transform and Huffman codec, no checksum and block of 1 MB. Lower compression ratio but very fast

{{{
java -cp kanzi.jar kanzi.app.BlockCompressor -input=c:\temp\rt.jar -output=c:\temp\rt.knz -overwrite -block=1M -transform=lz4 -entropy=huffman
Encoding ...

Encoding:          869 ms
Input size:        60008624
Output size:       23452848
Ratio:             0.39082462
Throughput (KB/s): 67436
}}}

With Snappy transform and Huffman codec, checksum, verbose and block of 4 MB. Using the Go version. The verbose option shows the impact of each step for each block.
{{{
go run BlockCompressor.go -input=c:\temp\rt.jar -output=c:\temp\rt.knz -overwrite -block=4M -verbose -checksum -entropy=huffman -transform=snappy
Input file name set to 'c:\temp\rt.jar'
Output file name set to 'c:\temp\rt.knz'
Block size set to 4194304 bytes
Debug set to true
Overwrite set to true
Checksum set to true
Using SNAPPY transform (stage 1)
Using HUFFMAN entropy codec (stage 2)
Using 1 job
Encoding ...
Block 0: 4194304 => 1890979 => 1600744 (38%)  [a613036f]
Block 1: 4194304 => 2050610 => 1732459 (41%)  [d43e35e6]
Block 2: 4194304 => 2003638 => 1709656 (40%)  [c178520c]
Block 3: 4194304 => 1959657 => 1665629 (39%)  [6b76374b]
Block 4: 4194304 => 1772464 => 1511513 (36%)  [4add340a]
Block 5: 4194304 => 1817654 => 1544182 (36%)  [2b22c33b]
Block 6: 4194304 => 1926893 => 1637754 (39%)  [414e8c24]
Block 7: 4194304 => 2001918 => 1700584 (40%)  [ee3876e0]
Block 8: 4194304 => 1958682 => 1655864 (39%)  [a4abcf1d]
Block 9: 4194304 => 1906796 => 1624348 (38%)  [de6ce5eb]
Block 10: 4194304 => 2111934 => 1790768 (42%)  [ae2e1ac1]
Block 11: 4194304 => 2225195 => 1891118 (45%)  [98235e9f]
Block 12: 4194304 => 2348021 => 1984262 (47%)  [8b38b0c6]
Block 13: 4194304 => 2218619 => 1880493 (44%)  [fa1cc886]
Block 14: 1288368 => 416802 => 357198 (27%)  [40559fcc]

Encoding:          1506 ms
Input size:        60008624
Output size:       24286590
Ratio:             0.404718
Throughput (KB/s): 38912
}}}

Decode with verbose mode (Go then Java):
{{{
go run BlockDecompressor.go -input=c:\temp\rt.knz -output=c:\temp\rt.jar -overwrite -verbose
Input file name set to 'c:\temp\rt.knz'
Output file name set to 'c:\temp\rt.jar'
Debug set to true
Overwrite set to true
Using 1 job
Decoding ...
Checksum set to true
Block size set to 4194304 bytes
Using SNAPPY transform (stage 1)
Using HUFFMAN entropy codec (stage 2)
Block 0: 1600744 => 1890979 => 4194304  [a613036f]
Block 1: 1732459 => 2050610 => 4194304  [d43e35e6]
Block 2: 1709656 => 2003638 => 4194304  [c178520c]
Block 3: 1665629 => 1959657 => 4194304  [6b76374b]
Block 4: 1511513 => 1772464 => 4194304  [4add340a]
Block 5: 1544182 => 1817654 => 4194304  [2b22c33b]
Block 6: 1637754 => 1926893 => 4194304  [414e8c24]
Block 7: 1700584 => 2001918 => 4194304  [ee3876e0]
Block 8: 1655864 => 1958682 => 4194304  [a4abcf1d]
Block 9: 1624348 => 1906796 => 4194304  [de6ce5eb]
Block 10: 1790768 => 2111934 => 4194304  [ae2e1ac1]
Block 11: 1891118 => 2225195 => 4194304  [98235e9f]
Block 12: 1984262 => 2348021 => 4194304  [8b38b0c6]
Block 13: 1880493 => 2218619 => 4194304  [fa1cc886]
Block 14: 357198 => 416802 => 1288368  [40559fcc]

Decoding:          1722 ms
Input size:        24286590
Output size:       60008624
Throughput (KB/s): 34031

java -cp kanzi.jar kanzi.app.BlockDecompressor -input=c:\temp\rt.knz -output=c:\temp\rt.jar -overwrite -verbose 
Input file name set to 'c:\temp\rt.knz'
Output file name set to 'c:\temp\rt.jar'
Verbose set to true
Overwrite set to true
Using 1 job
Decoding ...
Checksum set to true
Block size set to 4194304 bytes
Using SNAPPY transform (stage 1)
Using HUFFMAN entropy codec (stage 2)
Block 1: 1600744 => 1890979 => 4194304  [a613036f]
Block 2: 1732459 => 2050610 => 4194304  [d43e35e6]
Block 3: 1709656 => 2003638 => 4194304  [c178520c]
Block 4: 1665629 => 1959657 => 4194304  [6b76374b]
Block 5: 1511513 => 1772464 => 4194304  [4add340a]
Block 6: 1544182 => 1817654 => 4194304  [2b22c33b]
Block 7: 1637754 => 1926893 => 4194304  [414e8c24]
Block 8: 1700584 => 2001918 => 4194304  [ee3876e0]
Block 9: 1655864 => 1958682 => 4194304  [a4abcf1d]
Block 10: 1624348 => 1906796 => 4194304  [de6ce5eb]
Block 11: 1790768 => 2111934 => 4194304  [ae2e1ac1]
Block 12: 1891118 => 2225195 => 4194304  [98235e9f]
Block 13: 1984262 => 2348021 => 4194304  [8b38b0c6]
Block 14: 1880493 => 2218619 => 4194304  [fa1cc886]
Block 15: 357198 => 416802 => 1288368  [40559fcc]

Decoding:          1041 ms
Input size:        24286590
Output size:       60008624
Throughput (KB/s): 56294

}}}

*Silesia corpus compression tests*

https://kanzi.googlecode.com/files/silesia.png

Compression results for the Silesia Corpus (http://sun.aei.polsl.pl/~sdeor/index.php?page=silesia)

The tests were performed on a desktop i7-2600 @3.40GHz, Win7, 16GB RAM with Oracle JDK7 (1.7.25) with Kanzi 09/13.

Average of median 3 (of 5) tests used.

The block transform was used for all tests.

Java optimized flags: -Xms1024M -XX:+UseTLAB -XX:+AggressiveOpts -XX:+UseFastAccessorMethods 

The block size was set arbitrarily to 4000000 bytes. Bigger sizes yield better compression ratios and smaller sizes yield better speed.

The compression ratio is the size of the compressed file divided by the size of the original size.

For comparison purposes, here are the compression ratios obtained for each file in the corpus with several modern compressors:

https://kanzi.googlecode.com/files/silesia2.png

Kanzi using block compression, PAQ entropy and 16.000.000 bytes per block
7Z version 9.20, Bzip version 1.05 (option -9), jbzip2 (java implementation), ZPAQ 4.04


*See more details about the block transform*

https://code.google.com/p/kanzi/wiki/BlockCodec

*Performance of the Snappy and LZ4 codecs in Kanzi*

https://code.google.com/p/kanzi/wiki/SnappyCodec

== Other examples ==

More details are available at https://code.google.com/p/kanzi/
