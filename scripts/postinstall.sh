#!/bin/bash

if [ -z `getent group ssd` ]; then
	groupadd ssd
fi

if [ -z `getent passwd ssd` ]; then
	useradd ssd -g ssd -s /bin/sh
fi

install --mode=755 --owner=ssd --group=ssd --directory /var/log/ssd

systemctl daemon-reload

#END
