CC=g++
CFLAGS=-c -std=c++11 -Wall -Wextra -O3 -fomit-frame-pointer -msse2
#CFLAGS=-c  -Wall -DNDEBUG -O3 -fomit-frame-pointer -msse2 -std=c++0x -D_FILE_OFFSET_BITS=64 
LDFLAGS=-lpthread
LIB_SOURCES=Global.cpp \
	Event.cpp \
	transform/BWT.cpp \
	transform/BWTS.cpp \
	transform/DCT16.cpp \
	transform/DCT32.cpp \
	transform/DCT4.cpp \
	transform/DCT8.cpp \
	transform/DST4.cpp \
	transform/DivSufSort.cpp \
	transform/DWT_CDF_9_7.cpp \
	transform/MTFT.cpp \
	transform/SBRT.cpp \
	bitstream/DebugInputBitStream.cpp \
	bitstream/DebugOutputBitStream.cpp \
	bitstream/DefaultInputBitStream.cpp \
	bitstream/DefaultOutputBitStream.cpp \
	io/CompressedInputStream.cpp \
	io/CompressedOutputStream.cpp \
	entropy/ANSRangeDecoder.cpp \
	entropy/ANSRangeEncoder.cpp \
	entropy/BinaryEntropyDecoder.cpp \
	entropy/BinaryEntropyEncoder.cpp \
	entropy/EntropyUtils.cpp \
	entropy/ExpGolombDecoder.cpp \
	entropy/ExpGolombEncoder.cpp \
	entropy/HuffmanCommon.cpp \
	entropy/HuffmanDecoder.cpp \
	entropy/HuffmanEncoder.cpp \
	entropy/RangeDecoder.cpp \
	entropy/RangeEncoder.cpp \
	entropy/RiceGolombEncoder.cpp \
	entropy/RiceGolombDecoder.cpp \
	entropy/CMPredictor.cpp \
	entropy/FPAQPredictor.cpp \
	entropy/PAQPredictor.cpp \
	entropy/TPAQPredictor.cpp \
	function/BWTBlockCodec.cpp \
	function/LZ4Codec.cpp \
	function/ROLZCodec.cpp \
	function/RLT.cpp \
	function/SnappyCodec.cpp \
	function/TextCodec.cpp \
	function/X86Codec.cpp \
	function/ZRLT.cpp 
LIB_OBJECTS=$(LIB_SOURCES:.cpp=.o)

TEST_SOURCES=test/TestEntropyCodec.cpp \
	test/TestBWT.cpp \
	test/TestDefaultBitStream.cpp \
	test/TestFunctions.cpp \
	test/TestMTFT.cpp \
	test/TestTransforms.cpp
TEST_OBJECTS=$(TEST_SOURCES:.cpp=.o)

APP_SOURCES=app/Kanzi.cpp \
	app/InfoPrinter.cpp \
	app/BlockCompressor.cpp \
	app/BlockDecompressor.cpp
APP_OBJECTS=$(APP_SOURCES:.cpp=.o)

SOURCES=$(LIB_SOURCES) $(APP_SOURCES)
OBJECTS=$(SOURCES:.cpp=.o)
TESTS=testBWT testMTFT \
	testEntropyCodec testDefaultBitStream \
        testFunctions testTransforms

APP=kanzi
	
STATIC_LIB_SUFFIX := .a
SHARED_LIB_SUFFIX := .so
PROG_SUFFIX       := 
SHARED_LIB_CFLAGS := -fPIC

# Compiling for Windows with MinGW?
ifeq ($(OS),Windows_NT)
    STATIC_LIB_SUFFIX := .lib
    SHARED_LIB_SUFFIX := .dll
    PROG_SUFFIX       := .exe
    SHARED_LIB_CFLAGS := 
endif

STATIC_LIB := lib$(APP)$(STATIC_LIB_SUFFIX)
SHARED_LIB := lib$(APP)$(SHARED_LIB_SUFFIX)

all: $(TESTS) $(APP) $(STATIC_LIB)

# Create static library
$(STATIC_LIB):$(LIB_OBJECTS)
	$(AR) cr ../lib/$@ $+

# Create shared library
$(SHARED_LIB):$(LIB_OBJECTS)
	$(CC) -o ../lib/$@ $(LDFLAGS) $(SHARED_LIB_CFLAGS) -shared $+


testBWT: $(LIB_OBJECTS) test/TestBWT.o
	$(CC) $^ -o ../bin/$@ $(LDFLAGS)

testMTFT: $(LIB_OBJECTS) test/TestMTFT.o
	$(CC) $^ -o ../bin/$@ $(LDFLAGS)

testEntropyCodec: $(LIB_OBJECTS) test/TestEntropyCodec.o
	$(CC) $^ -o ../bin/$@ $(LDFLAGS)

testDefaultBitStream: $(LIB_OBJECTS) test/TestDefaultBitStream.o
	$(CC) $^ -o ../bin/$@ $(LDFLAGS)

testFunctions: $(LIB_OBJECTS) test/TestFunctions.o
	$(CC) $^ -o ../bin/$@ $(LDFLAGS) 

testTransforms: $(LIB_OBJECTS) test/TestTransforms.o
	$(CC) $^ -o ../bin/$@ $(LDFLAGS)

kanzi: $(OBJECTS) app/Kanzi.o
	$(CC) $^ -o ../bin/$@ $(LDFLAGS)

clean: 
	rm -f ../bin/test*$(PROG_SUFFIX) $(OBJECTS) $(TEST_OBJECTS) ../bin/$(APP)$(PROG_SUFFIX)
.cpp.o:
	$(CC) $(CFLAGS) $< -o $@
