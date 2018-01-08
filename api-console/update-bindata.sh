#!/bin/sh

set -e

if [ -z "$GOPATH" ] ; then
    echo "GOPATH not set."
    exit 1
fi

echo "{}" > package.json

# install uglifyjs
npm install uglify-js@3
node_modules/.bin/uglifyjs --screw-ie8 -o dist/scripts/app.min.js \
    dist/scripts/api-console-vendor.js  dist/scripts/api-console.js

# update or install bindata.
go get github.com/jteeuwen/go-bindata/...
go get github.com/elazarl/go-bindata-assetfs/...


go-bindata-assetfs --pkg apiconsole \
    dist/ dist/authentication/ dist/fonts/ dist/img/ dist/styles/ \
    dist/scripts/app.min.js

