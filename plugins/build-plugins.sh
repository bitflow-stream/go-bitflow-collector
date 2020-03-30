#!/usr/bin/env sh
# This script uses sh instead of bash, so it runs in most basic Docker containers (such as alpine)
home=`dirname $(readlink -f $0)`
test $# -ge 1 || { echo "Need 1 parameter: output folder for built plugin binaries"; exit 1; }
export plugin_output=$(readlink -f "$1")
shift

build_command="
    echo \"Building plugin \`basename \"\$0\"\`...\" &&
    cd \"\$0\" &&
    go build -buildmode=plugin -o \"$plugin_output/\`basename \"\$0\"\`\" $@ ."

# Compile all plugins
find "$home" -mindepth 1 -maxdepth 1 -type d -print0 | xargs -0 -n1 sh -c "$build_command"
