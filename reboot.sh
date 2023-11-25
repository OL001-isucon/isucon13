#!/bin/bash

# set -ux
set -eux

pdnsutil delete-zone u.isucon.dev
sudo rm -f /opt/aws-env-isucon-subdomain-address.sh.lock
sudo reboot
