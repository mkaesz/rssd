#!/bin/sh

curl -s -X 'GET' "https://api.put.io/v2/rss/list?oauth_token=${PUT_IO_TOKEN}" -H 'accept: application/json'
