kanzi
=====


Kanzi is a modern, modular, expandable and efficient lossless data compressor implemented in Go.

* modern: state-of-the-art algorithms are implemented and multi-core CPUs can take advantage of the built-in multi-tasking.
* modular: entropy codec and a combination of transforms can be provided at runtime to best match the kind of data to compress.
* expandable: clean design with heavy use of interfaces as contracts makes integrating and expanding the code easy. No dependencies.
* efficient: the code is optimized for efficiency (trade-off between compression ratio and speed).

Kanzi supports a wide range of compression ratios and can compress many files more than most common compressors (at the cost of decompression speed).
It is not compatible with standard compression formats.
Kanzi is a lossless data compressor, not an archiver. It uses checksums (optional but recommended) to validate data integrity but does not have a mechanism for data recovery. It also lacks data deduplication across files.


For more details, check https://github.com/flanglet/kanzi-go/wiki.

There is a C++ implementation available here: https://github.com/flanglet/kanzi-cpp

There is Java implementation available here: https://github.com/flanglet/kanzi


Credits

Matt Mahoney,
Yann Collet,
Jan Ondrus,
Yuta Mori,
Ilya Muravyov,
Neal Burns,
Fabian Giesen,
Jarek Duda,
Ilya Grebnov

Disclaimer

Use at your own risk. Always keep a backup of your files.


![Build Status](https://github.com/flanglet/kanzi-go/actions/workflows/go.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)
[![Total alerts](https://img.shields.io/lgtm/alerts/g/flanglet/kanzi-go.svg?logo=lgtm&logoWidth=18)](https://lgtm.com/projects/g/flanglet/kanzi-go/alerts/)
[![Documentation](https://godoc.org/github.com/flanglet/kanzi-go?status.svg)](http://godoc.org/github.com/flanglet/kanzi-go)


Silesia corpus benchmark
-------------------------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 22.04

go1.19.beta1

Kanzi version 2.1 Go implementation. Block size is 100 MB. 


|        Compressor               | Encoding (sec)  | Decoding (sec)  |    Size          |
|---------------------------------|-----------------|-----------------|------------------|
|Original     	                  |                 |                 |   211,938,580    |
|**Kanzi -l 1 -j 1**              |    	 **2.0**    |     **1.0**     |  **69,399,477**  |
|**Kanzi -l 1 -j 6**              |      **0.7**    |     **0.4**     |  **69,399,477**  |
|Pigz 2.6 -5 -p6                  |        1.0      |       0.7       |    69,170,603    |
|Gzip 1.10 -5                     |        4.8      |       1.0       |    69,143,980    |
|Zstd 1.5.3 -2 --long=30          |	       0.9      |       0.5       |    68,694,316    |
|Zstd 1.5.3 -2 -T6 --long=30      |	       0.4      |       0.3       |    68,694,316    |
|Brotli 1.0.9 -2 --large_window=30|        1.5      |       0.8       |    68,033,377    |
|Pigz 2.6 -9 -p6                  |        3.0      |       0.6       |    67,656,836    |
|Gzip 1.10 -9                     |       15.5      |       1.0       |    67,631,990    |
|**Kanzi -l 2 -j 1**              |      **3.2**    |     **1.2**     |  **63,808,747**  |
|**Kanzi -l 2 -j 6**              |      **1.2**    |     **0.5**     |  **63,808,747**  |
|Brotli 1.0.9 -4 --large_window=30|        4.1      |       0.7       |    64,267,169    |
|**Kanzi -l 3 -j 1**              |      **5.4**    |     **2.2**     |  **59,199,795**  |
|**Kanzi -l 3 -j 6**              |      **1.8**    |     **0.8**     |  **59,199,795**  |
|Zstd 1.5.3 -9 --long=30          |	       3.7      |       0.3       |    59,272,590    |
|Zstd 1.5.3 -9 -T6 --long=30      |	       2.3      |       0.3       |    59,272,590    |
|Orz 1.5.0                        |	       7.7      |       2.0       |    57,564,831    |
|Brotli 1.0.9 -9 --large_window=30|       36.7      |       0.7       |    56,232,817    |
|Lzma 5.2.2 -3	                  |       24.1	    |       2.6       |    55,743,540    |
|**Kanzi -l 4 -j 1**              |      **9.7**    |     **5.7**     |  **54,998,198**  |
|**Kanzi -l 4 -j 6**              |      **3.7**    |     **2.2**     |  **54,998,198**  |
|Bzip2 1.0.6 -9	                  |       14.9      |       5.2       |    54,506,769	   |
|Zstd 1.5.3 -19 --long=30	        |       62.0      |       0.3       |    52,828,057    |
|Zstd 1.5.3 -19	-T6 --long=30     |       62.0      |       0.4       |    52,828,057    |
|**Kanzi -l 5 -j 1**              |     **12.3**    |     **5.4**     |  **51,760,244**  |
|**Kanzi -l 5 -j 6**              |      **4.2**    |     **2.0**     |  **51,760,244**  |
|Brotli 1.0.9 --large_window=30   |      356.2	    |       0.9       |    49,383,136    |
|Lzma 5.2.2 -9                    |       65.6	    |       2.5       |    48,780,457    |
|**Kanzi -l 6 -j 1**              |     **14.3**    |     **8.3**     |  **48,068,000**  |
|**Kanzi -l 6 -j 6**              |      **4.8**    |     **2.8**     |  **48,068,000**  |
|bsc 3.2.3 -b100 -T -t            |        8.8      |       6.0       |    46,932,394    |
|bsc 3.2.3 -b100                  |        5.4      |       4.9       |    46,932,394    |
|BCM 1.65 -b100                   |       15.5      |      21.1       |    46,506,716    |
|**Kanzi -l 7 -j 1**              |     **19.5**    |    **13.3**     |  **46,447,003**  |
|**Kanzi -l 7 -j 6**              |      **6.8**    |     **5.1**     |  **46,447,003**  |
|Tangelo 2.4	                    |       83.2      |      85.9       |    44,862,127    |
|zpaq v7.14 m4 t1                 |      107.3	    |     112.2       |    42,628,166    |
|zpaq v7.14 m4 t12                |      108.1	    |     111.5       |    42,628,166    |
|**Kanzi -l 8 -j 1**              |     **60.7**    |    **60.8**     |  **41,821,127**  |
|**Kanzi -l 8 -j 6**              |     **24.5**    |    **20.9**     |  **41,821,155**  |
|Tangelo 2.0	                    |      302.0      |     310.9       |    41,267,068    |
|**Kanzi -l 9 -j 1**              |     **87.7**    |    **90.0**     |  **40,361,391**  |
|**Kanzi -l 9 -j 6**              |     **39.2**    |    **34.5**     |  **40,361,411**  |
|zpaq v7.14 m5 t1                 |      343.1	    |     352.0       |    39,112,924    |
|zpaq v7.14 m5 t12                |	     344.3	    |     350.4       |    39,112,924    |



enwik8
-------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 22.04

go1.19.beta1

Kanzi version 2.1 Go implementation. Block size is 100 MB. 1 thread


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   100,000,000    |
|**Kanzi -l 1 -j 1**          |     **1.10**    |    **0.42**     |  **37,969,539**  |
|**Kanzi -l 2 -j 1**          |     **1.90**    |    **0.63**     |  **30,953,719**  |
|**Kanzi -l 3 -j 1**          |     **2.68**    |    **1.05**     |  **27,362,969**  |
|**Kanzi -l 4 -j 1**          |	    **4.73**    |    **2.82**     |  **25,670,924**  |
|**Kanzi -l 5 -j 1**          |	    **4.85**    |    **1.94**     |  **22,490,875**  |
|**Kanzi -l 6 -j 1**          |	    **6.57**    |    **3.32**     |  **21,232,300**  |
|**Kanzi -l 7 -j 1**          |	    **9.44**    |    **5.98**     |  **20,935,519**  |
|**Kanzi -l 8 -j 1**          |	   **23.70**    |   **23.50**     |  **19,671,786**  |
|**Kanzi -l 9 -j 1**          |	   **34.09**    |   **35.61**     |  **19,097,946**  |


Build
-----

There are no dependencies, making the project easy to build.

**Option 1: go get** 

~~~
cd $GOPATH

go get github.com/flanglet/kanzi-go

cd src/github.com/flanglet/kanzi-go/app

go build Kanzi.go BlockCompressor.go BlockDecompressor.go InfoPrinter.go
~~~



**Option 2: git clone** 

~~~
cd $GOPATH/src

mkdir github.com; cd github.com

mkdir flanglet; cd flanglet

git clone https://github.com/flanglet/kanzi-go.git

cd kanzi-go/app

go build Kanzi.go BlockCompressor.go BlockDecompressor.go InfoPrinter.go
~~~
