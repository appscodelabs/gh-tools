#!/bin/bash

# Copyright AppsCode Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

pushd /home/tamal/go/src/github.com/tamalsaha/star-report

/home/tamal/go/bin/gh-tools star-report --report-dir=/home/tamal/go/src/github.com/tamalsaha/star-report
git add --all
git commit -a -s -m report\ $(date --iso-8601=seconds) || true
git push origin master

popd
