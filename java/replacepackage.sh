#!/bin/bash
sed -e "s/package kanzi/package android.kanzi/" $1 > /tmp/tmp.txt
mv /tmp/tmp.txt $1
sed -e "s/import kanzi/import android.kanzi/" $1 > /tmp/tmp.txt
mv /tmp/tmp.txt $1
sed -e "s/kanzi.io.IOException/android.kanzi.io.IOException/g" $1 > /tmp/tmp.txt
mv /tmp/tmp.txt $1
