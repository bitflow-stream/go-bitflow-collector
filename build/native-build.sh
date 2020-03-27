#!/bin/bash
home=`dirname $(readlink -f $0)`
root=`readlink -f "$home/.."`
cd "$home"

target="$home/_output/native"
mkdir -p "$target"
cp "$home/run-collector-with-plugins.sh" "$target"
go build -o "$target/bitflow-collector" $@ "$root/bitflow-collector"
"$root/plugins/build-plugins.sh" "$target/bitflow-collector-plugins" $@
