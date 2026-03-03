#!/bin/bash
while true
do
    while IFS= read -r line; do
        if [[ $line =~ ^#.* ]]; then
            continue
        fi
        curl -s http://localhost:8080/domain/$line | jq
    done < domains.txt

    while IFS= read -r line; do
        if [[ $line =~ ^#.* ]] || [[ -z $line ]]; then
            continue
        fi
        curl -s http://localhost:8080/ip/$line | jq
    done < ips.txt
done
