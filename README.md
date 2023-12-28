# Kanzi


Kanzi is a modern, modular, expandable and efficient lossless data compressor implemented in Go.

* modern: state-of-the-art algorithms are implemented and multi-core CPUs can take advantage of the built-in multi-tasking.
* modular: entropy codec and a combination of transforms can be provided at runtime to best match the kind of data to compress.
* expandable: clean design with heavy use of interfaces as contracts makes integrating and expanding the code easy. No dependencies.
* efficient: the code is optimized for efficiency (trade-off between compression ratio and speed).

Unlike the most common lossless data compressors, Kanzi uses a variety of different compression algorithms and supports a wider range of compression ratios as a result. Most usual compressors do not take advantage of the many cores and threads available on modern CPUs (what a waste!). Kanzi is concurrent by design and uses several go routines by default to compress blocks concurrently. It is not compatible with standard compression formats. Kanzi is a lossless data compressor, not an archiver. It uses checksums (optional but recommended) to validate data integrity but does not have a mechanism for data recovery. It also lacks data deduplication across files.

For more details, check https://github.com/flanglet/kanzi-go/wiki.

See how to reuse the code here: https://github.com/flanglet/kanzi-go/wiki/Using-and-extending-the-code

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

Use at your own risk. Always keep a copy of your original files.


![Build Status](https://github.com/flanglet/kanzi-go/actions/workflows/go.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)](https://goreportcard.com/badge/github.com/flanglet/kanzi-go)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=flanglet_kanzi-go&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=flanglet_kanzi-go)
[![Lines of Code](https://sonarcloud.io/api/project_badges/measure?project=flanglet_kanzi-go&metric=ncloc)](https://sonarcloud.io/summary/new_code?id=flanglet_kanzi-go)
[![Documentation](https://godoc.org/github.com/flanglet/kanzi-go?status.svg)](http://godoc.org/github.com/flanglet/kanzi-go/v2)



## Why Kanzi

There are many excellent, open-source lossless data compressors available already.

If gzip is starting to show its age, zstd and brotli are open-source, standardized and used
daily by millions of people. Zstd is incredibly fast and probably the best choice in many cases.
There are a few scenarios where Kanzi could be a better choice:

- gzip, lzma, brotli, zstd are all LZ based. It means that they can reach certain compression
ratios only. Kanzi also makes use of BWT and CM which can compress beyond what LZ can do.

- These LZ based compressors are well suited for software distribution (one compression / many decompressions)
due to their fast decompression (but low compression speed at high compression ratios). 
There are other scenarios where compression speed is critical: when data is generated before being compressed and consumed
(one compression / one decompression) or during backups (many compressions / one decompression).

- Kanzi has built-in customized data transforms (multimedia, utf, text, dna, ...) that can be chosen and combined 
at compression time to better compress specific kinds of data.

- Kanzi can take advantage of the multiple cores of a modern CPU to improve performance

- It is easy to implement a new transform or entropy codec to either test an idea or improve
compression ratio on specific kinds of data.



## Benchmarks

Test machine:

AWS c5a8xlarge: AMD EPYC 7R32 (32 vCPUs), 64 GB RAM

go 1.21.3

Ubuntu 22.04.3 LTS

Kanzi version 2.2 

On this machine, Kanzi can use up to 16 threads depending on compression level
(the default block size at level 9 is 32MB, severly limiting the number of threads
in use, especially with enwik8, but all tests are performed with default values).
bzip3 uses 16 threads. zstd can use 2 for compression, other compressors
are single threaded.


### silesia.tar

Download at http://sun.aei.polsl.pl/~sdeor/corpus/silesia.zip

|        Compressor               | Encoding (sec)  | Decoding (sec)  |    Size          |
|---------------------------------|-----------------|-----------------|------------------|
|Original     	                  |                 |                 |   211,957,760    |
|s2 -cpu 16   	                  |       0.494     |      0.868      |    86,650,932    |
|**Kanzi -l 1**                   |   	**0.683**   |    **0.255**    |  **80,284,705**  |
|s2 -cpu 16 -better  	            |       1.517     |      0.868      |    79,555,929    |
|Zstd 1.5.5 -2                    |	      0.761     |      0.286      |    69,590,245    |
|**Kanzi -l 2**                   |   	**0.707**   |    **0.302**    |  **68,231,498**  |
|Brotli 1.1.0 -2                  |       1.749     |      2.459      |    68,044,145    |
|Gzip 1.10 -9                     |      20.15      |      1.316      |    67,652,229    |
|**Kanzi -l 3**                   |   	**1.204**   |    **0.368**    |  **64,916,444**  |
|Zstd 1.5.5 -5                    |	      2.003     |      0.324      |    63,103,408    |
|**Kanzi -l 4**                   |   	**1.272**   |    **0.681**    |  **60,770,201**  |
|Zstd 1.5.5 -9                    |	      4.166     |      0.282      |    59,444,065    |
|Brotli 1.1.0 -6                  |      14.53      |      4.263      |    58,552,177    |
|Zstd 1.5.5 -13                   |	     19.15      |      0.276      |    58,061,115    |
|Brotli 1.1.0 -9                  |      70.07      |      7.149      |    56,408,353    |
|Bzip2 1.0.8 -9	                  |      16.94      |      6.734      |    54,572,500    |
|**Kanzi -l 5**                   |   	**2.355**   |    **1.055**    |  **54,051,139**  |
|Zstd 1.5.5 -19                   |	     92.82      |      0.302      |    52,989,654    |
|**Kanzi -l 6**                   |   	**3.414**   |    **2.235**    |  **49,517,823**  |
|Lzma 5.2.5 -9                    |      92.6       |      3.075      |    48,744,632    |
|**Kanzi -l 7**                   |   	**4.387**   |    **3.098**    |  **47,308,484**  |
|bzip3 1.3.2.r4-gb2d61e8 -j 16    |       2.682     |      3.221      |    47,237,088    |
|**Kanzi -l 8**                   |    **19.64**    |   **21.33**     |  **43,247,248**  |
|**Kanzi -l 9**                   |    **42.41**    |   **48.37**     |  **41,807,179**  |
|zpaq 7.15 -m5 -t16               |     213.8       |    213.8        |    40,050,429    |



### enwik8

Download at https://mattmahoney.net/dc/enwik8.zip

|      Compressor        | Encoding (sec)   | Decoding (sec)   |    Size          |
|------------------------|------------------|------------------|------------------|
|Original                |                  |                  |   100,000,000    |
|**Kanzi -l 1**          |     **0.465**    |    **0.171**     |  **43,747,730**  |
|**Kanzi -l 2**          |     **0.481**    |    **0.196**     |  **37,745,093**  |
|**Kanzi -l 3**          |     **0.761**    |    **0.301**     |  **33,839,184**  |
|**Kanzi -l 4**          |	   **0.764**    |    **0.472**     |  **29,598,635**  |
|**Kanzi -l 5**          |	   **0.896**    |    **0.494**     |  **26,527,955**  |
|**Kanzi -l 6**          |	   **1.433**    |    **1.104**     |  **24,076,669**  |
|**Kanzi -l 7**          |     **3.093**    |    **2.277**     |  **22,817,376**  |
|**Kanzi -l 8**          |	  **13.63**     |   **13.17**      |  **21,181,978**  |
|**Kanzi -l 9**          |	  **32.44**     |   **34.11**      |  **20,035,133**  |




# Build

Using formal releases is recommended (see https://github.com/flanglet/kanzi-go/releases).
```
go install github.com/flanglet/kanzi-go/v2/app@v2.2.0
```

Otherwise, to build manually from the latest tag, follow the instructions below:

```
git clone https://github.com/flanglet/kanzi-go.git

cd kanzi-go/v2/app

go build Kanzi.go BlockCompressor.go BlockDecompressor.go InfoPrinter.go
```

