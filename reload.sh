#!/bin/bash

# set -ux
set -eux

dir=$(cd $(dirname $0); pwd)

# truncate logs
sudo truncate -s 0 /var/log/mysql/error.log
sudo truncate -s 0 /var/log/mysql/mysql-slow.log
sudo truncate -s 0 /var/log/nginx/access.log
sudo truncate -s 0 /var/log/nginx/error.log

# go build
cd $dir/webapp/go
make build
cd $dir
