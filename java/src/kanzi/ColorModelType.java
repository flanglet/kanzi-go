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


public enum ColorModelType
{
    RGB(1), YUV444(2), YUV422(3), YUV420(4), YUV411(5);

    private final byte value;


    private ColorModelType(int value)
    {
        this.value = (byte) value;
    }

    @Override
    public String toString()
    {
        if (this.value == 2)
            return "YUV_444";

        if (this.value == 3)
            return "YUV_422";

        if (this.value == 4)
            return "YUV_420";

        if (this.value == 5)
            return "YUV_411";

        return "RGB";
    }
};
