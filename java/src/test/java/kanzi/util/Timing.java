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

import java.lang.management.ManagementFactory;
import java.lang.management.ThreadMXBean;

// Based on code located here:
// http://nadeausoftware.com/articles/2008/03/java_tip_how_get_cpu_and_user_time_benchmarking

public class Timing
{
   public static final Timing instance  = new Timing();


   private Timing()
   {
   }


   // Get CPU time in nanoseconds.
   public long getCpuTime()
   {
      ThreadMXBean bean = ManagementFactory.getThreadMXBean();
      return bean.isCurrentThreadCpuTimeSupported() ? bean.getCurrentThreadCpuTime() : 0L;
   }


   // Get user time in nanoseconds.
   public long getUserTime()
   {
      ThreadMXBean bean = ManagementFactory.getThreadMXBean();
      return bean.isCurrentThreadCpuTimeSupported() ? bean.getCurrentThreadUserTime() : 0L;
   }


   // Get system time in nanoseconds.
   public long getSystemTime()
   {
      ThreadMXBean bean = ManagementFactory.getThreadMXBean();

      return bean.isCurrentThreadCpuTimeSupported() ? 
         (bean.getCurrentThreadCpuTime() - bean.getCurrentThreadUserTime()) : 0L;
   }


   // Get CPU time in nanoseconds.
   public long getCpuTime(long[] ids)
   {
      ThreadMXBean bean = ManagementFactory.getThreadMXBean();

      if (bean.isThreadCpuTimeSupported() == false)
         return 0L;

      long time = 0L;

      for (int i=0; i<ids.length; i++)
      {
         long t = bean.getThreadCpuTime(ids[i]);

         if (t != -1)
            time += t;
      }

      return time;
   }


   // Get user time in nanoseconds.
   public long getUserTime(long[] ids)
   {
      ThreadMXBean bean = ManagementFactory.getThreadMXBean();

      if (bean.isThreadCpuTimeSupported() == false)
         return 0L;

      long time = 0L;

      for (int i=0; i<ids.length; i++)
      {
         long t = bean.getThreadUserTime(ids[i]);

         if (t != -1)
            time += t;
      }

      return time;
   }


   // Get system time in nanoseconds.
   public long getSystemTime(long[] ids)
   {
      ThreadMXBean bean = ManagementFactory.getThreadMXBean();

      if (bean.isThreadCpuTimeSupported() == false)
         return 0L;

      long time = 0L;

      for (int i=0; i<ids.length; i++)
      {
         long tc = bean.getThreadCpuTime(ids[i]);

         if (tc != -1)
         {
            long tu = bean.getThreadUserTime(ids[i]);

            if (tu != -1)
               time += (tc - tu);
         }
      }

      return time;
   }
}
