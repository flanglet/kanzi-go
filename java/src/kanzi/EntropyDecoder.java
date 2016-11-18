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

package kanzi;


public interface EntropyDecoder
{
    // Decode the next chunk of data from the bitstream and return in the
    // provided buffer.
    public int decode(byte[] buffer, int blkptr, int len);

    // Must be called before getting rid of the entropy coder
    public void dispose();

    // Return the underlying bitstream
    public InputBitStream getBitStream();
}

