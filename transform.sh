#!/bin/bash
#set -v

if [ $# -eq 0 ]; then
    echo Target directory argument required 
fi

if [ ! -f kanzi.zip ]; then
    echo java source zip 'kanzi.zip' required in current directory
fi

echo Transforming java files under $1 for Android

if [ ! -d $1 ]; then
   mkdir $1
fi

if [ ! -d $1/android ]; then
   mkdir $1/android
else
   rm -rf $1/android
fi

echo Extract source files
unzip -q kanzi.zip -d $1/android

# Delete incompatible files
echo Delete incompatible files
for f in `find $1/android -name "*.java" -exec grep -l "java.awt" {} \;`;
do
   rm -f $f
done

for f in `find $1/android -name "*.java" -exec grep -l "javax.swing" {} \;`;
do
   rm -f $f
done

for f in `find $1/android -name "*.java" -exec grep -l "concurrent.ForkJoin" {} \;`;
do
   rm -f $f
done

for f in `find $1/android -name "*.java" -exec grep -l "java.lang.management" {} \;`;
do
   rm -f $f
done

# Replace packages
echo Replace packages
find $1/android -name "*.java" -exec ./replacepackage.sh {} \;

echo Done !
