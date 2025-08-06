# Kanzi

Kanzi is a modern, modular, portable, and efficient lossless data compressor written in Go.

* Modern: Kanzi implements state-of-the-art compression algorithms and is built to fully utilize multi-core CPUs via built-in multi-threading.
* Modular: Entropy codecs and data transforms can be selected and combined at runtime to best suit the specific data being compressed.
* Expandable: A clean, interface-driven design—with no external dependencies—makes Kanzi easy to integrate, extend, and customize.
* Efficient: Carefully optimized to balance compression ratio and speed for practical, high-performance usage.

Unlike most mainstream lossless compressors, Kanzi is not limited to a single compression paradigm. By combining multiple algorithms and techniques, it supports a broader range of compression ratios and adapts better to diverse data types.

Most traditional compressors underutilize modern hardware by running single-threaded—even on machines with many cores. Kanzi, in contrast, is concurrent by design, compressing multiple blocks in parallel across threads for significant performance gains. However, it is not compatible with standard compression formats.

It’s important to note that Kanzi is a data compressor, not an archiver. It includes optional checksums for verifying data integrity, but does not provide features like cross-file deduplication or data recovery mechanisms. That said, it produces a seekable bitstream—meaning one or more consecutive blocks can be decompressed independently, without needing to process the entire stream.

For more details, see [Wiki](https://github.com/flanglet/kanzi-go/wiki), [Q&A](https://github.com/flanglet/kanzi-go/wiki/q&a) and [DeepWiki](https://deepwiki.com/flanglet/kanzi-go/1-overview)

See how to reuse the code here: https://github.com/flanglet/kanzi-go/wiki/Using-and-extending-the-code

There is a C++ implementation available here: https://github.com/flanglet/kanzi-cpp

There is Java implementation available here: https://github.com/flanglet/kanzi



![Build Status](https://github.com/flanglet/kanzi-go/actions/workflows/go.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/flanglet/kanzi-go/v2)](https://goreportcard.com/report/github.com/flanglet/kanzi-go/v2)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=flanglet_kanzi-go&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=flanglet_kanzi-go)
[![Lines of Code](https://sonarcloud.io/api/project_badges/measure?project=flanglet_kanzi-go&metric=ncloc)](https://sonarcloud.io/summary/new_code?id=flanglet_kanzi-go)
[![Documentation](https://godoc.org/github.com/flanglet/kanzi-go?status.svg)](http://godoc.org/github.com/flanglet/kanzi-go/v2)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/flanglet/kanzi-go)

## Why Kanzi

There are already many excellent, open-source lossless data compressors available.

If gzip is beginning to show its age, modern alternatives like **zstd** and **brotli** offer compelling replacements. Both are open-source, standardized, and used daily by millions. **Zstd** is especially notable for its exceptional speed and is often the best choice in general-purpose compression.

However, there are scenarios where **Kanzi** may offer superior performance:

While gzip, LZMA, brotli, and zstd are all based on LZ (Lempel-Ziv) compression, they are inherently limited in the compression ratios they can achieve. **Kanzi** goes further by incorporating **BWT (Burrows-Wheeler Transform)** and **CM (Context Modeling)**, which can outperform traditional LZ-based methods in certain cases.

LZ-based compressors are ideal for software distribution, where data is compressed once and decompressed many times, thanks to their fast decompression speeds—though they tend to be slower when compressing at higher ratios. But in other scenarios—such as real-time data generation, one-off data transfers, or backups—**compression speed becomes critical**. Here, Kanzi can shine.

**Kanzi** also features a suite of built-in, customizable data transforms tailored for specific data types (e.g., multimedia, UTF, text, DNA, etc.), which can be selectively applied during compression for better efficiency.

Furthermore, Kanzi is designed to **leverage modern multi-core CPUs** to boost performance.

Finally, **extensibility** is a key strength: implementing new transforms or entropy codecs—whether for experimentation or to improve performance on niche data types—is straightforward and developer-friendly.



## Benchmarks

Test machine:

Apple M3 24 GB Sonoma 14.6.1

Kanzi version 2.4.0 Go implementation

On this machine, Kanzi uses 4 threads (half of CPUs by default).

bzip3 runs with 4 threads. 

zstd and lz4 use 4 threads for compression and 1 for decompression, other compressors are single threaded.

The default block size at level 9 is 32MB, severely limiting the number of threads
in use, especially with enwik8, but all tests are performed with default values.


### silesia.tar

Download at http://sun.aei.polsl.pl/~sdeor/corpus/silesia.zip

|        Compressor               |  Encoding (ms)  |  Decoding (ms)  |    Size          |
|---------------------------------|-----------------|-----------------|------------------|
|Original                         |                 |                 |   211,957,760    |
|s2 -cpu 4                        |       179       |        294      |    86,892,891    |
|**Kanzi -l 1**                   |     **453**     |      **127**    |    80,245,856    |
|lz4 1.1.10 -T4 -4                |       527       |        121      |    79,919,901    |
|zstd 1.5.8 -T4 -2                |       147       |        150      |    69,410,383    |
|**Kanzi -l 2**                   |     **348**     |      **161**    |    68,860,099    |
|brotli 1.1.0 -2                  |       907       |        402      |    68,039,159    |
|Apple gzip 430.140.2 -9          |     10406       |        273      |    67,648,481    |
|**Kanzi -l 3**                   |     **649**     |      **235**    |    64,266,936    |
|zstd 1.5.8 -T4 -5                |       300       |        154      |    62,851,716    |
|**Kanzi -l 4**                   |    **1009**     |      **343**    |    61,131,554    |
|zstd 1.5.8 -T4 -9                |       752       |        137      |    59,190,090    |
|brotli 1.1.0 -6                  |      3596       |        340      |    58,557,128    |
|zstd 1.5.8 -T4 -13               |      4537       |        138      |    57,814,719    |
|brotli 1.1.0 -9                  |     19809       |        329      |    56,414,012    |
|bzip2 1.0.8 -9                   |      9673       |       3140      |    54,602,583    |
|**Kanzi -l 5**                   |    **2375**     |     **1229**    |    54,025,588    |
|zstd 1.5.8 -T4 -19               |     20482       |        151      |    52,858,610    |
|**kanzi -l 6**                   |    **3771**     |     **2555**    |    49,521,392    |
|xz 5.8.1 -9                      |     48516       |       1594      |    48,774,000    |
|bzip3 1.5.1.r3-g428f422 -j 4     |      8559       |       3948      |    47,256,794    |
|**Kanzi -l 7**                   |    **4858**     |     **4470**    |    47,312,772    |
|**Kanzi -l 8**                   |   **19529**     |    **18660**    |    43,260,254    |
|**Kanzi -l 9**                   |   **26439**     |    **26406**    |    41,858,030    |



### enwik8

Download at https://mattmahoney.net/dc/enwik8.zip

|   Compressor    | Encoding (ms)  | Decoding (ms)  |  Size        |
|-----------------|----------------|----------------|--------------|
|Original         |                |                |  100,000,000 |
|Kanzi -l 1       |       250      |         76     |   43,644,013 |
|Kanzi -l 2       |       196      |         92     |   37,570,404 |
|Kanzi -l 3       |       443      |        151     |   32,466,232 |
|Kanzi -l 4       |       448      |        203     |   29,536,517 |
|Kanzi -l 5       |       823      |        447     |   26,523,940 |
|Kanzi -l 6       |      1274      |       1247     |   24,076,765 |
|Kanzi -l 7       |      2188      |       2223     |   22,817,360 |
|Kanzi -l 8       |      7993      |       8110     |   21,181,992 |
|Kanzi -l 9       |     10790      |      11078     |   20,035,144 |




# Build

Using formal releases is recommended (see https://github.com/flanglet/kanzi-go/releases).
```
go install github.com/flanglet/kanzi-go/v2/app@v2.4.0
```

Otherwise, to build manually from the latest tag, follow the instructions below:

```
git clone https://github.com/flanglet/kanzi-go.git

cd kanzi-go/v2/app

go build Kanzi.go BlockCompressor.go BlockDecompressor.go InfoPrinter.go
```

The bistream is backward compatible, however, the guarantee only applies to releases. Users can expect incompatibilities or breakage due to bitstream changes in between releases.

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
