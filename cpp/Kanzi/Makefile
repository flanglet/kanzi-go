CC=g++
CFLAGS=-c -static -std=c++11 -Wall -Wextra -s -O2 -msse2 
#CFLAGS=-c -static -Wall -DNDEBUG -O3 -fomit-frame-pointer -msse2 -std=c++0x -D_FILE_OFFSET_BITS=64 
LDFLAGS=-lpthread
LIB_SOURCES=Global.cpp \
	transform/DCT16.cpp \
	transform/DCT32.cpp \
	transform/DCT4.cpp \
	transform/DCT8.cpp \
	transform/DST4.cpp \
	transform/DWT_CDF_9_7.cpp \
	transform/BWT.cpp \
	transform/BWTS.cpp \
	transform/MTFT.cpp \
	transform/SBRT.cpp \
	transform/DivSufSort.cpp \
	bitstream/DebugInputBitStream.cpp \
	bitstream/DebugOutputBitStream.cpp \
	bitstream/DefaultInputBitStream.cpp \
	bitstream/DefaultOutputBitStream.cpp \
	io/CompressedInputStream.cpp \
	io/CompressedOutputStream.cpp \
	io/BlockEvent.cpp \
	io/InfoPrinter.cpp \
	entropy/ANSRangeDecoder.cpp \
	entropy/ANSRangeEncoder.cpp \
	entropy/EntropyUtils.cpp \
	entropy/ExpGolombDecoder.cpp \
	entropy/ExpGolombEncoder.cpp \
	entropy/RiceGolombEncoder.cpp \
	entropy/RiceGolombDecoder.cpp \
	entropy/HuffmanDecoder.cpp \
	entropy/HuffmanEncoder.cpp \
	entropy/RangeDecoder.cpp \
	entropy/RangeEncoder.cpp \
	entropy/HuffmanCommon.cpp \
	entropy/AdaptiveProbMap.cpp \
	entropy/BinaryEntropyDecoder.cpp \
	entropy/BinaryEntropyEncoder.cpp \
	entropy/FPAQPredictor.cpp \
	entropy/PAQPredictor.cpp \
	entropy/TPAQPredictor.cpp \
	entropy/CMPredictor.cpp \
	function/BWTBlockCodec.cpp \
	function/LZ4Codec.cpp \
	function/SnappyCodec.cpp \
	function/RLT.cpp \
	function/ZRLT.cpp 
LIB_OBJECTS=$(LIB_SOURCES:.cpp=.o)

TEST_SOURCES=test/TestEntropyCodec.cpp \
	test/TestDefaultBitStream.cpp \
	test/TestFunctions.cpp \
	test/TestMTFT.cpp \
	test/TestBWT.cpp \
	test/TestTransforms.cpp
TEST_OBJECTS=$(TEST_SOURCES:.cpp=.o)

APP_SOURCES=app/Kanzi.cpp \
	app/BlockCompressor.cpp \
	app/BlockDecompressor.cpp
APP_OBJECTS=$(APP_SOURCES:.cpp=.o)

SOURCES=$(LIB_SOURCES) $(APP_SOURCES)
OBJECTS=$(SOURCES:.cpp=.o)
TESTS=TestBWT TestMTFT \
	TestEntropyCodec TestDefaultBitStream \
        TestFunctions TestTransforms

APP=Kanzi

all: tests app

tests: $(TESTS)

app: $(APP)

TestBWT: $(LIB_OBJECTS) test/TestBWT.o
	$(CC) $^ -o $@ $(LDFLAGS)

TestMTFT: $(LIB_OBJECTS) test/TestMTFT.o
	$(CC) $^ -o $@ $(LDFLAGS)

TestEntropyCodec: $(LIB_OBJECTS) test/TestEntropyCodec.o
	$(CC) $^ -o $@ $(LDFLAGS)

TestDefaultBitStream: $(LIB_OBJECTS) test/TestDefaultBitStream.o
	$(CC) $^ -o $@ $(LDFLAGS)

TestFunctions: $(LIB_OBJECTS) test/TestFunctions.o
	$(CC) $^ -o $@ $(LDFLAGS) 

TestTransforms: $(LIB_OBJECTS) test/TestTransforms.o
	$(CC) $^ -o $@ $(LDFLAGS)

Kanzi: $(LIB_OBJECTS) $(APP_OBJECTS) app/Kanzi.o
	$(CC) $^ -o $@ $(LDFLAGS)

clean: 
	rm -f $(TESTS) $(OBJECTS) $(TEST_OBJECTS) $(APP)

.cpp.o:
	$(CC) $(CFLAGS) $< -o $@
