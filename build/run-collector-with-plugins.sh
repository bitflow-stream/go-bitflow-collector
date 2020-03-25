#!/bin/sh
# Assume the collector binary at /bitflow-collector and plugins at /bitflow-collector-plugins/*

commandline="/bitflow-collector"
for plugin in /bitflow-collector-plugins/*; do
  commandline="$commandline -p $plugin"
done
$commandline
