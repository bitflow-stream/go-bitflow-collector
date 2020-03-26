#!/bin/bash
home=`dirname $(readlink -f $0)`
root=`readlink -f "$home/.."`

go build -o "$home/_output/native/bitflow-collector" $@ "$root/bitflow-collector"
"$root/plugins/build-plugins.sh" "$home/_output/native/bitflow-collector-plugins" $@
