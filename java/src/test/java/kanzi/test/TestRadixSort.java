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

package kanzi.test;

import kanzi.util.sort.RadixSort;
import org.junit.Test;


public class TestRadixSort extends TestAbstractSort
{
    @Test
    public void testRadixSort()
    {
        testCorrectness("RadixSort (radix 1)", new RadixSort(1), 5);
        testCorrectness("RadixSort (radix 2)", new RadixSort(2), 5);
        testCorrectness("RadixSort (radix 4)", new RadixSort(4), 5);
        testCorrectness("RadixSort (radix 8)", new RadixSort(8), 5);
        testSpeed("RadixSort (radix 4)", new RadixSort(4), 5000);
        testSpeed("RadixSort (radix 8)", new RadixSort(8), 5000);
    }    
}

