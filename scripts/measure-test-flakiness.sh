#!/usr/bin/env bash

set -euo pipefail

if [[ -z ${GITHUB_TOKEN:-} ]]
then
    echo "Please set the \$GITHUB_TOKEN environment variable for the script to work"
    exit 1
fi

temp_dir=$(mktemp -d)

trap '{ rm -rf -- "${temp_dir}"; }' EXIT

json_file="${temp_dir}/commit-and-check-data.json"

# This GraphQL query retrieves information about the most recent commits to the default branch of the "etcd" repository owned by "etcd-io".
# Specifically, it retrieves the commit URL and the state of the status check rollup for each of the 100 most recent commits on the default branch.
# {
#   repository(owner: "etcd-io", name: "etcd") {
#     defaultBranchRef {
#       target {
#         ... on Commit {
#           history(first: 100) {
#             edges {
#               node {
#                 ... on Commit {
#                   commitUrl
#                   statusCheckRollup {
#                     state
#                   }
#                 }
#               }
#             }
#           }
#         }
#       }
#     }
#   }
# }

# Try this above query in https://docs.github.com/en/graphql/overview/explorer

curl --fail --show-error --silent -H "Authorization: token ${GITHUB_TOKEN}" \
    -X POST \
    -d '{
        "query": "query { repository(owner: \"etcd-io\", name: \"etcd\") { defaultBranchRef { target { ... on Commit { history(first: 100) { edges { node { ... on Commit { commitUrl statusCheckRollup { state } } } } } } } } } }"
    }' \
    https://api.github.com/graphql | jq . > "${json_file}"

failure_percentage=$(jq '.data.repository.defaultBranchRef.target.history.edges | reduce .[] as $item (0; if $item.node.statusCheckRollup.state == "FAILURE" then (. + 1) else . end)' "${json_file}")

echo "Commit status failure percentage is - ${failure_percentage} %"
