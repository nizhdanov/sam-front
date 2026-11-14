#!/bin/sh
set -eu
case " ${RENEWED_DOMAINS:-} " in
    *" sam.v1sdev.ru "*)
        cd /opt/sam-backend
        docker compose -f compose.yaml -f compose.server.yaml exec -T nginx nginx -t
        docker compose -f compose.yaml -f compose.server.yaml exec -T nginx nginx -s reload
        ;;
esac
