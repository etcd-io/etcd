#!/bin/bash

# Copyright 2018 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

usage () {
    echo 'Username required: ./user_add.sh $username'
    exit
}

if [ "$1" == "" ]; then
    usage
fi

newuser=$1
read -r -s -p "Enter password for $newuser" newpass

user=root
pass=toor
host=127.0.0.1
port=2379
api=v3

cacert="path/to/ca.pem"
key="path/to/client-key.pem"
cert="path/to/client.pem"

tokengen() {
    json=$(printf '{"name": "%s", "password": "%s"}' \
        "$(escape "$1")" \
        "$(escape "$2")"
    )
    curl -s --cacert $cacert \
        --key $key \
        --cert $cert \
        -X POST \
        -d "$json" \
        https://${host}:${port}/${api}/auth/authenticate \
       | jq -r '.token'
}

add_user() {
    json=$(printf '{"name": "%s", "password": "%s"}' \
        "$(escape "$1")" \
        "$(escape "$2")"
    )
    curl -s --cacert $cacert \
        --key $key \
        --cert $cert \
        -H "Authorization: $3" \
        -X POST \
        -d "$json" \
        https://${host}:${port}/${api}/auth/user/add
}

escape() {
    echo "${1//\"/\\\"}"
}

token=$(tokengen $user $pass)
response=$(add_user $newuser $newpass $token)

echo -e "\\n$response"

