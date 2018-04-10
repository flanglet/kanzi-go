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


// Interface used by the binary entropy coder to predict probabilities of 0 and 1
// symbols in the input signal
public interface Predictor 
{
    // Update the probability model
    public void update(int bit);
   
    
    // Return the split value representing the probability of 1 in the [0..4095] range. 
    // E.G. 410 represents roughly a probability of 10% for 1
    public int get();
}
