#!/bin/bash

set -u

if [ ! "$#" -eq 1 ] || [ -z "$1" ]; then 
    echo "pass lambda function name as input"
    exit 1
fi

#
# Lambda
#
function delete_function() {
    set -u
    aws --profile bcap lambda list-event-source-mappings --function-name $1 | 
        jq -r '.EventSourceMappings[].UUID' | 
        parallel 'aws --profile bcap lambda delete-event-source-mapping --uuid {}'

    aws --profile bcap lambda delete-function --function-name $1
}

export -f delete_function

aws --profile bcap lambda list-functions | 
    jq -r '.Functions[].FunctionName' | 
    grep $1 |
    parallel delete_function {}

#
# SQS
#
aws --profile bcap sqs list-queues | 
    jq '.QueueUrls[]' -r | 
    grep $1 | 
    parallel 'aws --profile bcap sqs delete-queue --queue-url {}'

#
# IAM
#

function delete_role() {
    set -u
    aws --profile bcap iam detach-role-policy --role-name $1 --policy-arn arn:aws:iam::467357729182:policy/$1
    aws --profile bcap iam delete-role-policy --role-name $1 --policy-name $1
    # aws --profile bcap iam delete-policy --policy-arn arn:aws:iam::467357729182:policy/$1
    aws --profile bcap iam delete-role --role-name $1
}

export -f delete_role

aws --profile bcap iam list-roles | 
    jq -r '.Roles[].RoleName' |
    grep $1 |
    parallel delete_role {}

