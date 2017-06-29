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

Kanzi version 1.1 C++.

(Pareto in bold)

|        Compressor           | Encoding (sec)  |    Size        |
|-----------------------------|-----------------|----------------|
|**Kanzi LZ4**	              |    **0.8**      | **101631119**  |	
|**Kanzi LZ4+Huffman**        |  	  **1.1** 	  |  **86358882**  |
|**Zstd 1.1.4 -6**	          | 	  **2.1**     |  **62969187**  | 
|Gzip 1.6	                    |       5.7    	  |    68227965    |        
|Zstd 1.1.4 -13               |	     10.5       |    58789203    |
|Kanzi BWT+RANK+ZRLT+ANS      |	     13.8	      |    52061115    |
|Bzip2 1.0.6 -9	              |      14.2       |    54506769	   |
|**Kanzi BWT+RANK+ZRLT+FPAQ** |    **16.1**     |  **49489938**  |
|Lzma 5.1.0alpha -3	          |      23.0	      |    55743540    |
|**Kanzi BWT+CM**	            |     **27**	    |  **46505288**  |
|Zstd 1.1.4 -19	              |       51	      |    54016682    |
|Lzma 5.1.0alpha -9           |       70.7	    |    48780457    |
|Tangelo 2.4	                |       79.9      |    44862127    |
|**Kanzi TPAQ**               |     **93.1**	  |  **42415732**  |
|zpaq v7.14 method 4          |      106.0	    |    42628166    |
|Tangelo 2.0	                |      300.3    	|    41267068    |
|zpaq v7.14 method 5          |	    392.8	      |    39112924    |
