#!/bin/bash
home=`dirname $(readlink -f $0)`
"$home/build.sh" alpine $@
