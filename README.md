kanzi
=====


State-of-the-art lossless data compression in Go.
The goal is to provide clean APIs and really fast implementation.
It includes compression codecs (Run Length coding, Exp Golomb coding, Huffman, Range, LZ4, Snappy, ANS, Context Mixers, PAQ derivatives), bit stream manipulation, and transforms such as Burrows-Wheeler (BWT) and Move-To-Front, etc ...



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



Silesia corpus benchmark
-------------------------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 18.04

go 1.12beta1

Kanzi version 1.5 Go implementation. Block size is 100 MB. 


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   211,938,580    |	
|**Kanzi -l 1**               |  	   **2.0** 	  |     **1.2**     |  **80,003,837**  |
|**Kanzi -l 1 -j 12**         |  	   **0.8** 	  |     **0.4**     |  **80,003,837**  |
|Gzip 1.6	                    |        6.0      |       1.0       |    68,227,965    |        
|Gzip 1.6	-9                  |       14.3      |       1.0       |    67,631,990    |        
|**Kanzi -l 2**               |	     **5.3**	  |     **2.8**     |  **63,878,466**  |
|**Kanzi -l 2 -j 12**         |	     **2.0**	  |     **1.1**     |  **63,878,466**  |
|Zstd 1.3.3 -13               |	      11.9      |       0.3       |    58,789,182    |
|Brotli 1.0.5 -9              |       94.3      |       1.4       |    56,289,305    |
|Lzma 5.2.2 -3	              |       24.3	    |       2.4       |    55,743,540    |
|**Kanzi -l 3**               |	    **13.3**	  |    **10.3**     |  **55,594,153**  |
|**Kanzi -l 3 -j 12**         |	     **5.3**	  |     **4.0**     |  **55,594,153**  |
|Bzip2 1.0.6 -9	              |       14.1      |       4.8       |    54,506,769	   |
|Zstd 1.3.3 -19	              |       45.2      |       0.4       |    53,977,895    |
|**Kanzi -l 4**               |	    **13.7**	  |    **10.0**     |  **51,795,306**  |
|**Kanzi -l 4 -j 12**         |      **5.2**    |     **3.6**     |  **51,795,306**  |
|**Kanzi -l 5**	              |     **18.1**    |    **15.8**     |  **49,455,342**  |
|**Kanzi -l 5 -j 12**         |      **6.6**    |     **5.7**     |  **49,455,342**  |
|Lzma 5.2.2 -9                |       65.0	    |       2.4       |    48,780,457    |
|**Kanzi -l 6**               |     **36.5**	  |    **34.2**     |  **46,485,165**  |
|**Kanzi -l 6 -j 12**         |     **11.3**	  |    **10.6**     |  **46,485,165**  |
|Tangelo 2.4	                |       83.2      |      85.9       |    44,862,127    |
|zpaq v7.14 m4 t1             |      107.3	    |     112.2       |    42,628,166    |
|zpaq v7.14 m4 t12            |      108.1	    |     111.5       |    42,628,166    |
|**Kanzi -l 7**               |     **68.6**	  |    **71.1**     |  **41,838,503**  |
|**Kanzi -l 7 -j 12**         |     **24.7**	  |    **25.0**     |  **41,838,503**  |
|Tangelo 2.0	                |      302.0    	|     310.9       |    41,267,068    |
|**Kanzi -l 8**               |     **91.7**	  |    **95.1**     |  **40,844,691**  |
|**Kanzi -l 8 -j 12**         |     **35.9**	  |    **35.9**     |  **40,844,691**  |
|zpaq v7.14 m5 t1             |	     343.1	    |     352.0       |    39,112,924    |
|zpaq v7.14 m5 t12            |	     344.3	    |     350.4       |    39,112,924    |


enwik8
-------

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 18.04

go 1.12beta1

Kanzi version 1.5 Go implementation. Block size is 100 MB. 1 thread


|        Compressor           | Encoding (sec)  | Decoding (sec)  |    Size          |
|-----------------------------|-----------------|-----------------|------------------|
|Original     	              |                 |                 |   100,000,000    |	
|**Kanzi -l 1**               |  	  **1.33** 	  |    **0.77**     |  **35,611,290**  |
|**Kanzi -l 2**               |     **2.82**    |    **1.55**     |  **28,468,601**  |        
|**Kanzi -l 3**               |	    **6.64**    |    **5.00**     |  **25,517,555**  |
|**Kanzi -l 4**               |	    **5.58**	  |    **3.84**     |  **22,512,813**  |
|**Kanzi -l 5**               |	    **8.22**	  |    **6.69**     |  **21,934,022**  |
|**Kanzi -l 6**               |	   **19.45**	  |   **16.93**     |  **20,791,492**  |
|**Kanzi -l 7**               |	   **28.52**	  |   **29.09**     |  **19,613,190**  |
|**Kanzi -l 8**               |	   **36.18**	  |   **37.00**     |  **19,284,434**  |


Build
-----

There are no dependencies, making the project easy to build.

**Option 1: go get** 

~~~
cd $GOPATH

go get github.com/flanglet/kanzi-go

cd src/github.com/flanglet/kanzi-go/app

go build -gcflags=-B Kanzi.go BlockCompressor.go BlockDecompressor.go InfoPrinter.go
~~~



**Option 2: git clone** 

~~~
cd $GOPATH/src

mkdir github.com; cd github.com

mkdir flanglet; cd flanglet

git clone https://github.com/flanglet/kanzi-go.git

cd kanzi-go/app

go build -gcflags=-B Kanzi.go BlockCompressor.go BlockDecompressor.go InfoPrinter.go
~~~
