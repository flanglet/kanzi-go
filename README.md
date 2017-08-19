kanzi
=====


This project offers Java, C++ and Go code for manipulation and compression of data and images.
The goal is to provide clean APIs and really fast implementation.
It includes lossless compression codecs (Huffman, Range, LZ4, Snappy, PAQ), color model transforms, resampling, wavelet, DCT, Hadamard transform, bit stream manipulation, Burrows-Wheeler (BWT) and Move-To-Front transform, Run Length coding, etc ...
It also provides video filters such as fast Gaussian filter, Sobel filter and constant time bilateral filter.


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

Kanzi version 1.2 C++ implementation. Block size is 100 MB. 
All corpus files compressed one by one.


|        Compressor           | Encoding (sec)  |    Size          |
|-----------------------------|-----------------|------------------|
|Original     	              |                 |   211,938,580    |	
|**Kanzi -l 1**               |  	   **1.9** 	  |  **81,468,951**  |
|Zstd 1.1.4 -6*               | 	     2.1      |    62,969,187    | 
|Gzip 1.6	                    |        5.7      |    68,227,965    |        
|Zstd 1.1.4 -13               |	      10.5      |    58,789,203    |
|**Kanzi -l 2**               |	    **13.8**	  |  **51,781,225**  |
|Bzip2 1.0.6 -9	              |       14.2      |    54,506,769	   |
|**Kanzi -l 3**               |     **16.0**    |  **49,483,430**  |
|Lzma 5.1.0alpha -3	          |       23.0	    |    55,743,540    |
|**Kanzi -l 4**	              |     **27.0**    |  **46,485,004**  |
|Zstd 1.1.4 -19	              |       51.0      |    54,016,682    |
|Lzma 5.1.0alpha -9           |       70.7	    |    48,780,457    |
|Tangelo 2.4	                |       79.9      |    44,862,127    |
|**Kanzi -l 5**               |     **79.6**	  |  **42,356,371**  |
|zpaq v7.14 method 4          |      106.0	    |    42,628,166    |
|Tangelo 2.0	                |      300.3    	|    41,267,068    |
|zpaq v7.14 method 5          |	     392.8	    |    39,112,924    |
