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

package kanzi.filter.seam;


// 'Container' class encapsulating a path of pixels in an image.
// Used by ContextResizer.
public class Geodesic implements Comparable<Geodesic>
{
   public int cost;
   public final int[] positions;
   public final int direction;


   public Geodesic(int direction, int length)
   {
      this.direction = direction;
      this.positions = new int[length];
   }


   @Override
   public int compareTo(Geodesic geo)
   {
      return this.cost - geo.cost;
   }


   @Override
   public String toString()
   {
      StringBuilder builder = new StringBuilder(200);

      if (this.direction == 1)
          builder.append("[dir=HORIZONTAL");
       else
          builder.append("[dir=VERTICAL");

      builder.append(", cost=");
      builder.append(this.cost);
      builder.append(", start=");
      builder.append(this.positions[0]);
      builder.append("]");
      return builder.toString();
   }
  
}