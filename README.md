kanzi
=====


Kanzi is a modern, modular, expendable and efficient lossless data compressor implemented in Go.

* modern: state-of-the-art algorithms are implemented and multi-core CPUs can take advantage of the built-in multi-tasking.
* modular: entropy codec and a combination of transforms can be provided at runtime to best match the kind of data to compress.
* expandable: clean design with heavy use of interfaces as contracts makes integrating and expanding the code easy. No dependencies.
* efficient: the code is optimized for efficiency (trade-off between compression ratio and speed).

Kanzi supports a wide range of compression ratios and can compress many files more than most common compressors (at the cost of decompression speed).
It is not compatible with standard compression formats.


For more details, check https://github.com/flanglet/kanzi-go/wiki.

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

Use at your own risk. Always keep a backup of your files. The bitstream format is not yet finalized.


[![Build Status](https://travis-ci.org/flanglet/kanzi-go.svg?branch=master)](https://travis-ci.org/flanglet/kanzi-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)
[![Total alerts](https://img.shields.io/lgtm/alerts/g/flanglet/kanzi-go.svg?logo=lgtm&logoWidth=18)](https://lgtm.com/projects/g/flanglet/kanzi-go/alerts/)
[![Documentation](https://godoc.org/github.com/flanglet/kanzi-go?status.svg)](http://godoc.org/github.com/flanglet/kanzi-go)


Silesia corpus benchmark
-------------------------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 18.04.05

go1.15rc1

Kanzi version 1.8 Go implementation. Block size is 100 MB. 


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   211,938,580    |	
|Gzip 1.6	-4                  |        3.4      |       1.1       |    71,045,115    |        
|**Kanzi -l 1**               |  	   **2.4** 	  |     **1.3**     |  **69,840,720**  |
|**Kanzi -l 1 -j 6**          |  	   **0.9** 	  |     **0.4**     |  **69,840,720**  |
|Zstd 1.4.5 -2                |	       0.7      |       0.3       |    69,636,234    |
|Gzip 1.6	-5                  |        4.4      |       1.1       |    69,143,980    |        
|Brotli 1.0.7 -2              |        4.4      |       2.0       |    68,033,377    |
|Gzip 1.6	-9                  |       14.3      |       1.0       |    67,631,990    |        
|**Kanzi -l 2**               |	     **5.0**	  |     **2.3**     |  **60,147,109**  |
|**Kanzi -l 2 -j 6**          |	     **1.8**	  |     **0.8**     |  **60,147,109**  |
|Zstd 1.4.5 -13               |	      16.0      |       0.3       |    58,125,865    |
|Orz 1.5.0                    |	       7.6      |       2.0       |    57,564,831    |
|Brotli 1.0.7 -9              |       92.2      |       1.7       |    56,289,305    |
|Lzma 5.2.2 -3	              |       24.3	    |       2.4       |    55,743,540    |
|**Kanzi -l 3**               |	    **10.8**	  |     **6.8**     |  **54,996,910**  |
|**Kanzi -l 3 -j 6**          |	     **3.9**	  |     **2.3**     |  **54,996,910**  |
|Bzip2 1.0.6 -9	              |       14.9      |       5.2       |    54,506,769	   |
|Zstd 1.4.5 -19	              |       61.8      |       0.3       |    53,261,006    |
|Zstd 1.4.5 -19	-T6           |       53.4      |       0.3       |    53,261,006    |
|**Kanzi -l 4**               |	    **12.3**	  |     **6.9**     |  **51,739,977**  |
|**Kanzi -l 4 -j 6**          |      **4.2**    |     **2.1**     |  **51,739,977**  |
|Lzma 5.2.2 -9                |       65.0	    |       2.4       |    48,780,457    |
|**Kanzi -l 5**	              |     **15.5**    |    **11.2**     |  **48,067,650**  |
|**Kanzi -l 5 -j 6**          |      **5.3**    |     **3.7**     |  **48,067,650**  |
|**Kanzi -l 6**               |     **26.3**	  |    **22.2**     |  **46,543,124**  |
|**Kanzi -l 6 -j 6**          |      **8.5**	  |     **7.0**     |  **46,543,124**  |
|Tangelo 2.4	                |       83.2      |      85.9       |    44,862,127    |
|zpaq v7.14 m4 t1             |      107.3	    |     112.2       |    42,628,166    |
|zpaq v7.14 m4 t12            |      108.1	    |     111.5       |    42,628,166    |
|**Kanzi -l 7**               |     **63.2**	  |    **64.7**     |  **41,804,239**  |
|**Kanzi -l 7 -j 6**          |     **23.3**	  |    **21.9**     |  **41,804,239**  |
|Tangelo 2.0	                |      302.0    	|     310.9       |    41,267,068    |
|**Kanzi -l 8**               |     **85.1**	  |    **86.0**     |  **40,423,483**  |
|**Kanzi -l 8 -j 6**          |     **32.7**	  |    **33.5**     |  **40,423,483**  |
|zpaq v7.14 m5 t1             |	     343.1	    |     352.0       |    39,112,924    |
|zpaq v7.14 m5 t12            |	     344.3	    |     350.4       |    39,112,924    |


enwik8
-------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 18.04.05

go1.15rc1

Kanzi version 1.8 Go implementation. Block size is 100 MB. 1 thread


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   100,000,000    |	
|**Kanzi -l 1**               |  	  **1.40** 	  |    **0.75**     |  **32,654,135**  |
|**Kanzi -l 2**               |     **2.48**    |    **1.22**     |  **27,410,862**  |        
|**Kanzi -l 3**               |	    **5.22**    |    **3.36**     |  **25,670,935**  |
|**Kanzi -l 4**               |	    **5.02**	  |    **2.64**     |  **22,481,393**  |
|**Kanzi -l 5**               |	    **7.17**	  |    **4.50**     |  **21,232,214**  |
|**Kanzi -l 6**               |	   **10.99**	  |    **8.34**     |  **20,951,898**  |
|**Kanzi -l 7**               |	   **24.10**	  |   **24.39**     |  **19,515,358**  |
|**Kanzi -l 8**               |	   **31.84**	  |   **32.79**     |  **19,099,778**  |


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
