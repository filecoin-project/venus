#!/bin/sh -e

sed -i -e "s|\$SLACK_API_URL|$SLACK_API_URL|g" /etc/alertmanager/alertmanager.yml

if [[ -z $1 ]] || [[ ${1:0:1} == '-' ]] ; then
  exec /bin/alertmanager  "$@"
else
  exec "$@"
fi
