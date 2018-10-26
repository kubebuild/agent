#!/usr/bin/env bash

ME=`basename "$0"`

usage()
{
  echo "Usage: $ME [version]" >&2
  exit 1
}

VERSION=$1
if [ -z "$VERSION" ]; then usage; fi

IMAGE=kubebuild/agent

docker build -t $IMAGE:$VERSION .
docker tag $IMAGE:$VERSION $IMAGE:latest

docker push $IMAGE:$VERSION
docker push $IMAGE:latest