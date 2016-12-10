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

package kanzi.app;

import java.util.Arrays;



public class Kanzi
{
   public static void main(String[] args)
   {
      if (args.length > 0) 
      {
         String firstArg = args[0].trim().toUpperCase();
         
         switch(firstArg) 
         {
            case "-COMPRESS" :
               args = Arrays.copyOfRange(args, 1, args.length);
               BlockCompressor.main(args);
               return;
               
            case "-DECOMPRESS" :
               args = Arrays.copyOfRange(args, 1, args.length);
               BlockDecompressor.main(args);
               return;
               
            case "-HELP" :
               System.out.println("java -cp kanzi.jar kanzi.app.Kanzi -compress | -decompress | -help");
               return;
               
            default:
               // Fallback
         }
      }
      
      System.out.println("Missing arguments: try '-help'");
   }
}
