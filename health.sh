#!/bin/bash
while true
do
    #echo $(shuf -i 1-100 -n 1)
    curl -s http://localhost:8080/health | jq
    sleep 1
done
