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

package kanzi.util;


// Implementation based on initial code at http://www.compuphase.com/hilbert.htm
public class HilbertCurveGenerator
{
    private enum Direction { UP, LEFT, DOWN, RIGHT }

    private final int dim;


    public HilbertCurveGenerator(int dim)
    {
       if (dim < 2)
          throw new IllegalArgumentException("The dimension must be at least 2");
       
       this.dim = dim;
    }


    public int[] generate(int[] data)
    {
        if (data.length < this.dim*this.dim)
            data = new int[this.dim*this.dim];

        int level = 0;

        for (int d=this.dim; d>=2; d>>=1)
           level++;

        data[0] = 0;
        Context ctx = new Context(data, 1, 0);
        this.generate(level, Direction.LEFT, ctx);
        return data;
    }

    
    private void generate(int level, Direction direction, Context ctx)
    {
      final int[] data = ctx.data;

      if (level == 1)
      {
        int offset = ctx.offset;
        int idx = ctx.idx;

        switch (direction)
        {
            case LEFT:
              offset++;
              data[idx++] = offset;
              offset += this.dim;
              data[idx++] = offset;
              offset--;
              data[idx++] = offset;
              break;

            case RIGHT:
              offset--;
              data[idx++] = offset;
              offset -= this.dim;
              data[idx++] = offset;
              offset++;
              data[idx++] = offset;
              break;

            case UP:
              offset += this.dim;
              data[idx++] = offset;
              offset++;
              data[idx++] = offset;
              offset -= this.dim;
              data[idx++] = offset;
              break;

            case DOWN:
              offset -= this.dim;
              data[idx++] = offset;
              offset--;
              data[idx++] = offset;
              offset += this.dim;
              data[idx++] = offset;
              break;
         }

         ctx.offset = offset;
         ctx.idx = idx;
      }
      else
      {
        switch (direction)
        {
            case LEFT:
              generate(level-1, Direction.UP, ctx);
              ctx.offset++;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.LEFT, ctx);
              ctx.offset += this.dim;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.LEFT, ctx);
              ctx.offset--;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.DOWN, ctx);
              break;

            case RIGHT:
              generate(level-1, Direction.DOWN, ctx);
              ctx.offset--;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.RIGHT, ctx);
              ctx.offset -= this.dim;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.RIGHT, ctx);
              ctx.offset++;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.UP, ctx);
              break;

            case UP:
              generate(level-1, Direction.LEFT, ctx);
              ctx.offset += this.dim;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.UP, ctx);
              ctx.offset++;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.UP, ctx);
              ctx.offset -= this.dim;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.RIGHT, ctx);
              break;

            case DOWN:
              generate(level-1, Direction.RIGHT, ctx);
              ctx.offset -= this.dim;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.DOWN, ctx);
              ctx.offset--;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.DOWN, ctx);
              ctx.offset += this.dim;
              data[ctx.idx++] = ctx.offset;
              generate(level-1, Direction.LEFT, ctx);
              break;
         }
       }
    }


    private static class Context
    {
        final int[] data;
        int idx;
        int offset;

        Context(int[] data, int idx, int offset)
        {
            this.data = data;
            this.idx = idx;
            this.offset = offset;
        }
    }

}
