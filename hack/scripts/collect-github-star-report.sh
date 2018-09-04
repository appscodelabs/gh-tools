#!/bin/bash

gh-tools star-report --report-dir=/home/tamal/go/src/github.com/tamalsaha/star-report
pushd /home/tamal/go/src/github.com/tamalsaha/star-report
git add --all
git commit -a -s -m report\ $(date --iso-8601=seconds) || true
git push origin master
popd
