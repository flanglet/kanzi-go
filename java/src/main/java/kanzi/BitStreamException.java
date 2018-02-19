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


public class BitStreamException extends RuntimeException
{
    private static final long serialVersionUID = 7279737120722476336L;
    
    public static final int UNDEFINED = 0;
    public static final int INPUT_OUTPUT   = 1;
    public static final int END_OF_STREAM  = 2;
    public static final int INVALID_STREAM = 3;
    public static final int STREAM_CLOSED  = 4;

    private final int code;
    
    
    protected BitStreamException()
    {
        this.code = UNDEFINED;
    }
    
    
    public BitStreamException(String message, int code)
    {
        super(message);
        this.code = code;
    }
    
    
    public BitStreamException(String message, Throwable cause, int code)
    {
        super(message, cause);
        this.code = code;
    }
    
    
    public BitStreamException(Throwable cause, int code)
    {
        super(cause);
        this.code = code;
    }
    
    
    public int getErrorCode()
    {
        return this.code;
    }
}
