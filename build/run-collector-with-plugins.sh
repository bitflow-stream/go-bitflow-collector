#!/bin/sh
home=`dirname $(readlink -f $0)`

if [ $# -ge 2 -a "$1" = "-root" ]; then
  root="$2"
  shift 2
else
  # By default, execute the native build
  root="$home/_output/native"
fi

commandline="$root/bitflow-collector"
for plugin in "$root/bitflow-collector-plugins"/*; do
  echo "Adding plugin $plugin"
  commandline="$commandline -p $plugin"
done
$commandline $@

