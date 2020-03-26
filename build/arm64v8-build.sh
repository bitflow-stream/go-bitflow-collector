#!/bin/bash
home=`dirname $(readlink -f $0)`
test $# -ge 1 || { echo "Parameters: <Go-mod-cache directory> <Build args (optional)>"; exit 1; }
"$home/build.sh" arm64v8 $@ -tags nolibvirt
