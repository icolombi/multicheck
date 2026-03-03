#!/bin/bash
while true
do
    while IFS= read -r line; do
        if [[ $line =~ ^#.* ]]; then
            continue
        fi
        curl -s https://lab.icolombi.net/multicheck/domain/$line | jq
    done < domains.txt

    while IFS= read -r line; do
        if [[ $line =~ ^#.* ]] || [[ -z $line ]]; then
            continue
        fi
        curl -s https://lab.icolombi.net/multicheck/ip/$line | jq
    done < ips.txt
done

