kanzi
=====


State-of-the-art lossless data compression in Go.
The goal is to provide clean APIs and really fast implementation.
It includes compression codecs (Run Length coding, Exp Golomb coding, Huffman, Range, LZ, ANS, Context Mixers, PAQ derivatives), bit stream manipulation, and transforms such as Burrows-Wheeler (BWT) and Move-To-Front, etc ...



For more details, check https://github.com/flanglet/kanzi/wiki.

Credits

Matt Mahoney,
Yann Collet,
Jan Ondrus,
Yuta Mori,
Ilya Muravyov,
Neal Burns,
Fabian Giesen,
Jarek Duda

Disclaimer

Use at your own risk. Always keep a backup of your files.


[![Build Status](https://travis-ci.org/flanglet/kanzi-go.svg?branch=master)](https://travis-ci.org/flanglet/kanzi-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)
[![Documentation](https://godoc.org/github.com/flanglet/kanzi-go?status.svg)](http://godoc.org/github.com/glanglet-kanzi-go)


Silesia corpus benchmark
-------------------------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 18.04

go 1.13beta1

Kanzi version 1.6 Go implementation. Block size is 100 MB. 


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   211,938,580    |	
|**Kanzi -l 1**               |  	   **1.8** 	  |     **1.2**     |  **76,600,331**  |
|**Kanzi -l 1 -j 6**          |  	   **0.9** 	  |     **0.5**     |  **76,600,331**  |
|Gzip 1.6	                    |        6.0      |       1.0       |    68,227,965    |        
|Gzip 1.6	-9                  |       14.3      |       1.0       |    67,631,990    |        
|**Kanzi -l 2**               |	     **4.7**	  |     **2.4**     |  **61,788,747**  |
|**Kanzi -l 2 -j 6**          |	     **2.1**	  |     **1.0**     |  **61,788,747**  |
|Zstd 1.3.3 -13               |	      11.9      |       0.3       |    58,789,182    |
|Brotli 1.0.5 -9              |       94.3      |       1.4       |    56,289,305    |
|Lzma 5.2.2 -3	              |       24.3	    |       2.4       |    55,743,540    |
|**Kanzi -l 3**               |	    **10.9**	  |     **8.8**     |  **55,983,177**  |
|**Kanzi -l 3 -j 6**          |	     **4.1**	  |     **2.9**     |  **55,983,177**  |
|Bzip2 1.0.6 -9	              |       14.1      |       4.8       |    54,506,769	   |
|Zstd 1.3.3 -19	              |       45.2      |       0.4       |    53,977,895    |
|**Kanzi -l 4**               |	    **12.9**	  |     **6.7**     |  **51,795,306**  |
|**Kanzi -l 4 -j 6**          |      **4.4**    |     **2.9**     |  **51,795,306**  |
|**Kanzi -l 5**	              |     **17.2**    |    **12.9**     |  **48,279,102**  |
|**Kanzi -l 5 -j 6**          |      **6.0**    |     **4.7**     |  **48,279,102**  |
|Lzma 5.2.2 -9                |       65.0	    |       2.4       |    48,780,457    |
|**Kanzi -l 6**               |     **33.0**	  |    **28.6**     |  **46,485,189**  |
|**Kanzi -l 6 -j 6**          |     **10.5**	  |     **8.5**     |  **46,485,189**  |
|Tangelo 2.4	                |       83.2      |      85.9       |    44,862,127    |
|zpaq v7.14 m4 t1             |      107.3	    |     112.2       |    42,628,166    |
|zpaq v7.14 m4 t12            |      108.1	    |     111.5       |    42,628,166    |
|**Kanzi -l 7**               |     **64.1**	  |    **66.0**     |  **41,892,099**  |
|**Kanzi -l 7 -j 6**          |     **21.4**	  |    **21.9**     |  **41,892,099**  |
|Tangelo 2.0	                |      302.0    	|     310.9       |    41,267,068    |
|**Kanzi -l 8**               |     **86.2**	  |    **88.4**     |  **40,502,391**  |
|**Kanzi -l 8 -j 6**          |     **31.6**	  |    **32.0**     |  **40,502,391**  |
|zpaq v7.14 m5 t1             |	     343.1	    |     352.0       |    39,112,924    |
|zpaq v7.14 m5 t12            |	     344.3	    |     350.4       |    39,112,924    |


enwik8
-------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 18.04

go 1.13beta1

Kanzi version 1.6 Go implementation. Block size is 100 MB. 1 thread


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   100,000,000    |	
|**Kanzi -l 1**               |  	  **1.33** 	  |    **0.78**     |  **34,135,723**  |
|**Kanzi -l 2**               |     **2.65**    |    **1.37**     |  **27,450,033**  |        
|**Kanzi -l 3**               |	    **5.50**    |    **4.32**     |  **25,695,567**  |
|**Kanzi -l 4**               |	    **5.18**	  |    **2.68**     |  **22,512,452**  |
|**Kanzi -l 5**               |	    **7.96**	  |    **5.44**     |  **21,301,346**  |
|**Kanzi -l 6**               |	   **17.47**	  |   **13.81**     |  **20,791,496**  |
|**Kanzi -l 7**               |	   **24.28**	  |   **25.08**     |  **19,597,394**  |
|**Kanzi -l 8**               |	   **32.36**	  |   **33.35**     |  **19,163,098**  |


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
