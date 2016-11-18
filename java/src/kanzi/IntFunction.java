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


// An integer function is an operation that transforms the input int array and writes
// the result in the output int array. The result may have a different size.
// The function may fail if input and output array are the same array.
// The index of input and output arrays are updated appropriately.
public interface IntFunction extends IntTransform
{
   // Return the max size required for the output buffer
   // If the max size of the output buffer is not known, return -1
   public int getMaxEncodedLength(int srcLength);
}
