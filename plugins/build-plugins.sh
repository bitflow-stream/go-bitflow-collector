#!/usr/bin/env bash
export home=`dirname $(readlink -e $0)`
export plugin_output="$home/_output"

function build_plugin() {
  plugin_dir="$@"
  plugin_name=$(basename "$plugin_dir")
  echo "Building plugin ${plugin_name}..."
  cd "$plugin_dir"
  go build -buildmode=plugin -o "$plugin_output/$plugin_name" .
}
export -f build_plugin

function build_dependency() {
  echo "Building $@..."
  go install "$@"/...
}

# Install dependencies to build the plugins against up-to-date dependency versions
build_dependency "github.com/bitflow-stream/go-bitflow"
build_dependency "github.com/bitflow-stream/go-bitflow-collector"

# Compile all plugins
find "$home" -mindepth 1 -maxdepth 1 -type d -not -name "_output" -exec bash -c 'build_plugin $0' {} \;
