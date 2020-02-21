kanzi
=====


State-of-the-art lossless data compression in Go.
The goal is to provide clean APIs and really fast implementation of common algorithms.
It includes compression codecs (Run Length coding, Exp Golomb coding, Huffman, Range, LZ, ANS, Context Mixers, PAQ derivatives), bit stream manipulation, and transforms such as Burrows-Wheeler (BWT) and Move-To-Front, etc ...

Kanzi is the most versatile lossless data compressor in Go.  However, it is not an implementation of usual compression formats (zip, Zstandard,  LZMA, ...).


For more details, check https://github.com/flanglet/kanzi/wiki.

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


[![Build Status](https://travis-ci.org/flanglet/kanzi-go.svg?branch=master)](https://travis-ci.org/flanglet/kanzi-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)
[![Documentation](https://godoc.org/github.com/flanglet/kanzi-go?status.svg)](http://godoc.org/github.com/flanglet/kanzi-go)


Silesia corpus benchmark
-------------------------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 18.04

go 1.13

Kanzi version 1.7 Go implementation. Block size is 100 MB. 


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   211,938,580    |	
|**Kanzi -l 1**               |  	   **2.2** 	  |     **1.4**     |  **74,599,655**  |
|**Kanzi -l 1 -j 6**          |  	   **1.0** 	  |     **0.5**     |  **74,599,655**  |
|Gzip 1.6	                    |        6.0      |       1.0       |    68,227,965    |        
|Gzip 1.6	-9                  |       14.3      |       1.0       |    67,631,990    |        
|**Kanzi -l 2**               |	     **5.1**	  |     **2.3**     |  **61,679,732**  |
|**Kanzi -l 2 -j 6**          |	     **3.0**	  |     **0.9**     |  **61,679,732**  |
|Zstd 1.4.5 -13               |	      15.9      |       0.3       |    58,125,865    |
|Orz 1.5.0                    |	       7.7      |       1.9       |    57,564,831    |
|Brotli 1.0.7 -9              |       91.9      |       1.4       |    56,289,305    |
|**Kanzi -l 3**               |	    **12.0**	  |     **8.0**     |  **55,952,061**  |
|**Kanzi -l 3 -j 6**          |	     **4.5**	  |     **2.8**     |  **55,952,061**  |
|Lzma 5.2.2 -3	              |       24.3	    |       2.4       |    55,743,540    |
|Bzip2 1.0.6 -9	              |       14.1      |       4.8       |    54,506,769	   |
|Zstd 1.4.5 -19	              |       45.2      |       0.3       |    53,261,006    |
|**Kanzi -l 4**               |	    **12.8**	  |     **6.8**     |  **51,754,417**  |
|**Kanzi -l 4 -j 6**          |      **4.3**    |     **2.5**     |  **51,754,417**  |
|Lzma 5.2.2 -9                |       65.0	    |       2.4       |    48,780,457    |
|**Kanzi -l 5**	              |     **18.1**    |    **13.7**     |  **48,256,346**  |
|**Kanzi -l 5 -j 6**          |      **6.3**    |     **4.7**     |  **48,256,346**  |
|**Kanzi -l 6**               |     **34.1**	  |    **26.3**     |  **46,588,792**  |
|**Kanzi -l 6 -j 6**          |     **10.9**	  |     **8.3**     |  **46,588,792**  |
|Tangelo 2.4	                |       83.2      |      85.9       |    44,862,127    |
|zpaq v7.14 m4 t1             |      107.3	    |     112.2       |    42,628,166    |
|zpaq v7.14 m4 t12            |      108.1	    |     111.5       |    42,628,166    |
|**Kanzi -l 7**               |     **64.4**	  |    **65.0**     |  **41,862,443**  |
|**Kanzi -l 7 -j 6**          |     **21.1**	  |    **21.6**     |  **41,862,443**  |
|Tangelo 2.0	                |      302.0    	|     310.9       |    41,267,068    |
|**Kanzi -l 8**               |     **85.6**	  |    **85.9**     |  **40,473,911**  |
|**Kanzi -l 8 -j 6**          |     **31.1**	  |    **30.9**     |  **40,473,911**  |
|zpaq v7.14 m5 t1             |	     343.1	    |     352.0       |    39,112,924    |
|zpaq v7.14 m5 t12            |	     344.3	    |     350.4       |    39,112,924    |


enwik8
-------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 18.04

go 1.13

Kanzi version 1.7 Go implementation. Block size is 100 MB. 1 thread


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   100,000,000    |	
|**Kanzi -l 1**               |  	  **1.48** 	  |    **0.88**     |  **33,869,944**  |
|**Kanzi -l 2**               |     **2.73**    |    **1.36**     |  **27,404,489**  |        
|**Kanzi -l 3**               |	    **5.95**    |    **4.01**     |  **25,661,699**  |
|**Kanzi -l 4**               |	    **5.18**	  |    **2.79**     |  **22,478,636**  |
|**Kanzi -l 5**               |	    **8.55**	  |    **5.50**     |  **21,275,446**  |
|**Kanzi -l 6**               |	   **13.09**	  |    **9.06**     |  **20,893,702**  |
|**Kanzi -l 7**               |	   **24.60**	  |   **24.43**     |  **19,570,938**  |
|**Kanzi -l 8**               |	   **32.40**	  |   **32.23**     |  **19,141,858**  |


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
