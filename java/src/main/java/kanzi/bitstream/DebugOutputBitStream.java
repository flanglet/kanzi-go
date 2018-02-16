/*
Copyright 2011-2017 Frederic Langlet
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
you may obtain a copy of the License at

                http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kanzi.bitstream;

import kanzi.BitStreamException;
import java.io.PrintStream;
import kanzi.OutputBitStream;


// Very util little wrapper used to print the bits written to the delegate
// bitstream (uses the decorator design pattern)
public final class DebugOutputBitStream implements OutputBitStream
{
    private final OutputBitStream delegate;
    private final PrintStream out;
    private final int width;
    private int lineIndex;
    private boolean mark;
    private boolean hexa;
    private byte current;


    public DebugOutputBitStream(OutputBitStream bitstream)
    {
        this(bitstream, System.out, 80);
    }


    public DebugOutputBitStream(OutputBitStream bitstream, PrintStream out)
    {
        this(bitstream, out, 80);
    }


    public DebugOutputBitStream(OutputBitStream bitstream, PrintStream out, int width)
    {
        if (bitstream == null)
            throw new NullPointerException("Invalid null bitstream parameter");

        if (out == null)
            throw new NullPointerException("Invalid null print stream parameter");

        if ((width != -1) && (width < 8))
            width = 8;

        if (width != -1)
            width &= 0xFFFFFFF8;

        this.width = width;
        this.delegate = bitstream;
        this.out = out;
    }


    public synchronized void setMark(boolean mark)
    {
        this.mark = mark;
    }


    public synchronized boolean mark()
    {
        return this.mark;
    }


    public synchronized void showByte(boolean hex)
    {
        this.hexa = hex;
    }


    public synchronized boolean showByte()
    {
        return this.hexa;
    }


    protected synchronized void printByte(byte val)
    {
       if ((val >= 0) && (val < 10))
            this.out.print(" [00" + (val & 0xFF) + "] ");
        else if ((val >= 0) && (val < 100))
            this.out.print(" [0" + (val & 0xFF) + "] ");
        else
            this.out.print(" [" + (val & 0xFF) + "] ");
    }


    // Processes the least significant bit of the input integer
    @Override
    public synchronized void writeBit(int bit) throws BitStreamException
    {
         bit &= 1;
         this.out.print((bit == 1) ? "1" : "0");
         this.current <<= 1;
         this.current |= bit;
         this.lineIndex++;

         if (this.mark == true)
             this.out.print("w");

         if (this.width != -1)
         {
             if ((this.lineIndex-1) % this.width == this.width-1)
             {
                 if (this.showByte())
                     this.printByte(this.current);

                 this.out.println();
                 this.lineIndex = 0;
             }
             else if ((this.lineIndex & 7) == 0)
             {
                 if (this.showByte())
                     this.printByte(this.current);
                 else
                     this.out.print(" ");
             }
         }
         else if ((this.lineIndex & 7) == 0)
         {
             if (this.showByte())
                 this.printByte(this.current);
             else
                 this.out.print(" ");
         }

        this.delegate.writeBit(bit);
    }


    @Override
    public synchronized int writeBits(long bits, int length) throws BitStreamException
    {
        int res = this.delegate.writeBits(bits, length);

        for (int i=1; i<=res; i++)
        {
            long bit = (bits >> (res-i)) & 1;
            this.current <<= 1;
            this.current |= bit;
            this.lineIndex++;
            this.out.print((bit == 1) ? "1" : "0");

            if ((this.mark == true) && (i == res))
                this.out.print("w");

            if (this.width != -1)
            {
                if (this.lineIndex % this.width == 0)
                {
                    if (this.showByte())
                        this.printByte(this.current);

                    this.out.println();
                    this.lineIndex = 0;
                }
                else if ((this.lineIndex & 7) == 0)
                {
                    if (this.showByte())
                        this.printByte(this.current);
                    else
                        this.out.print(" ");
                }
            }
            else if ((this.lineIndex & 7) == 0)
            {
                if (this.showByte())
                    this.printByte(this.current);
                else
                    this.out.print(" ");            
            }
        }

        return res;
    }
    

    @Override
    public void close() throws BitStreamException
    {
        this.delegate.close();
    }


    @Override
    public long written()
    {
        return this.delegate.written();
    }
}
