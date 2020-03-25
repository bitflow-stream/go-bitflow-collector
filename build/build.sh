#!/bin/bash
home=`dirname $(readlink -f $0)`
root=`readlink -f "$home/.."`

test $# -ge 1 || { echo "Need parameter(s): container to build for (arm32v7/arm64v8/alpine), and optionally the base directory to use for the Go-mod-cache"; exit 1; }
BUILD_TARGET="$1"
shift
BUILD_IMAGE="teambitflow/bitflow-collector:build-$BUILD_TARGET"

MOUNT_GO_MOD_CACHE=""
if [ $# -ge 1 ]; then
  mod_cache_dir=$(readlink -f "$1/$BUILD_TARGET")
  echo "Using Go-mod-cache directory: $mod_cache_dir"
  MOUNT_GO_MOD_CACHE="-v $mod_cache_dir:/go"
else
  echo "====================================================================================================================================="
  echo "WARNING: Building without Go-mod-cache. To use the cache, provide extra parameter: the base-directory to store the Go-mod-cache into."
  echo "====================================================================================================================================="
fi

# Build inside the container, but mount relevant directories to get access to the build results.
docker run -ti $MOUNT_GO_MOD_CACHE -v "$root:/build/src" "$BUILD_IMAGE" \
  sh -c "
    # Copy entire source-tree in order to make changes to go.mod/go.sum
    cp -r src build
    cd build

    # Prepare go.mod/go.sum files
    sed -i \$(find -name go.mod) -e '\_//.*gitignore\$_d' -e '\_#.*gitignore\$_d'
    find -name go.sum -delete

    # Build the collector and plugins, put the outputs in the mounted source folder
    go build -tags nolibvirt -o ../src/build/_output/$BUILD_TARGET/bitflow-collector ./bitflow-collector
    ./plugins/build-plugins.sh ../src/plugins/_output/$BUILD_TARGET
  "
