#!/usr/bin/env sh
# This script uses sh instead of bash, so it runs in most basic Docker containers (such as alpine)
export home=`dirname $(readlink -f $0)`
test $# = 1 || { echo "Need 1 parameter: output folder for built plugin binaries"; exit 1; }
export plugin_output="$1"

build_dependency() {
  echo "Building $@..."
  go install "$@"/...
}

# Install dependencies to build the plugins against up-to-date dependency versions
build_dependency "github.com/bitflow-stream/go-bitflow"
build_dependency "github.com/bitflow-stream/go-bitflow-collector"

# Compile all plugins
find "$home" -mindepth 1 -maxdepth 1 -type d -not -name "_output" -print0 | xargs -0 -n1 sh -c '
  plugin_dir="$0" &&
  plugin_name=`basename "$plugin_dir"` &&
  echo "Building plugin ${plugin_name}..." &&
  cd "$plugin_dir" &&
  go build -buildmode=plugin -o "$plugin_output/$plugin_name" .'
