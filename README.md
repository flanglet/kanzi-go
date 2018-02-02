kanzi
=====


This project offers Java, C++ and Go code for data compression.
The goal is to provide clean APIs and really fast implementation.
It includes lossless compression codecs (Huffman, Range, LZ4, Snappy, ANS, Context Mixers, PAQ derivatives), bit stream manipulation, Burrows-Wheeler (BWT) and Move-To-Front transform, Run Length coding, Exp Golomb coding, etc ...
The Java code also provides image manipulation utilities (color model transforms, resampling, Wavelet, DCT, Hadamard transform) and  filters such as fast Gaussian filter, Sobel filter, fast median filter and constant time bilateral filter.


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

i7-7700K @4.20GHz, 32GB RAM, Ubuntu 16.10

Kanzi version 1.3 C++ implementation. Block size is 100 MB. 
All corpus files compressed one by one sequentially (1 job).


|        Compressor           | Encoding (sec)  |    Size          |
|-----------------------------|-----------------|------------------|
|Original     	              |                 |   211,938,580    |	
|**Kanzi -l 1**               |  	   **1.8** 	  |  **81,471,095**  |
|Zstd 1.1.4 -6*               | 	     2.1      |    62,969,187    | 
|Gzip 1.6	                    |        5.7      |    68,227,965    |        
|Zstd 1.1.4 -13               |	      10.5      |    58,789,203    |
|**Kanzi -l 2**               |	    **13.3**	  |  **51,781,225**  |
|Bzip2 1.0.6 -9	              |       14.2      |    54,506,769	   |
|**Kanzi -l 3**               |     **15.7**    |  **49,482,502**  |
|Lzma 5.1.0alpha -3	          |       23.0	    |    55,743,540    |
|**Kanzi -l 4**	              |     **26.6**    |  **46,485,004**  |
|Zstd 1.1.4 -19	              |       51.0      |    54,016,682    |
|Lzma 5.1.0alpha -9           |       70.7	    |    48,780,457    |
|Tangelo 2.4	                |       79.9      |    44,862,127    |
|**Kanzi -l 5**               |     **76.5**	  |  **41,522,031**  |
|zpaq v7.14 method 4          |      106.0	    |    42,628,166    |
|Tangelo 2.0	                |      300.3    	|    41,267,068    |
|zpaq v7.14 method 5          |	     392.8	    |    39,112,924    |
