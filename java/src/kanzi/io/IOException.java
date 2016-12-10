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

package kanzi.io;


public class IOException extends java.io.IOException 
{
   private static final long serialVersionUID = -9153775235137373283L;

   private final int code;
   
   public IOException(String msg, int code)
   {
      super(msg);
      this.code = code;
   }
   
   
   public int getErrorCode()
   {
      return this.code;
   }
}
