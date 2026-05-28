#!/bin/bash
cd /root/msg
go build -o lavender-server . >> /var/log/lavender-build.log 2>&1
exec ./lavender-server >> /var/log/lavender-server.log 2>&1
