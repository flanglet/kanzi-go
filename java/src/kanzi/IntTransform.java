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

package kanzi;


public interface IntTransform
{
   // Indexed arrays are required rather than just arrays and indexes
   // Since the number of integers in input and output of the transform may differ
   // the arrays may not be big enough and the number of processed integers may
   // vary. The indexes in the indexed array instance can be updated to reflect
   // this fact.
   public boolean forward(IndexedIntArray src, IndexedIntArray dst);


   // Indexed arrays are required rather than just arrays and indexes
   // Since the number of integers in input and output of the transform may differ
   // the arrays may not be big enough and the number of processed integers may
   // vary. The indexes in the indexed array instance can be updated to reflect
   // this fact.
   public boolean inverse(IndexedIntArray src, IndexedIntArray dst);
}
