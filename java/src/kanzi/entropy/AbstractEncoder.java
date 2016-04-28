/*
Copyright 2011-2013 Frederic Langlet
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

package kanzi.entropy;

import kanzi.EntropyEncoder;
import kanzi.OutputBitStream;


public abstract class AbstractEncoder implements EntropyEncoder
{
    protected abstract void encodeByte(byte val);

    
    @Override
    public abstract OutputBitStream getBitStream();

    
    // Default implementation: fallback to encodeByte
    // Some implementations should be able to use an optimized algorithm
    @Override
    public int encode(byte[] array, int blkptr, int len)
    {
        if ((array == null) || (blkptr + len > array.length) || (blkptr < 0) || (len < 0))
           return -1;

        final int end = blkptr + len;

        for (int i=blkptr; i<end; i++)
           this.encodeByte(array[i]);

        return len;
    }


    @Override
    public void dispose()
    {
    }
}
