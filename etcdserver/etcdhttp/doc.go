/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

/*

## Add Member

### Request

`curl http://${remote_client_url}/v2/admin/members/{$id} -XPUT -d 'PeerURLs=${peer_url_1}&PeerURLs=${peer_url_2}'`

Parameter `remote_client_url` is serving client url of the cluster.
Parameter `id` is the identification of new member in hexadecimal.
Parameter `peer_url_` is peer urls of the new member.

### Response

Categorized by HTTP status code.

#### HTTP 201

The member is created successfully.

#### HTTP 400

etcd cannot parse out the request.

#### HTTP 500

etcd fails to create the new member.

## Remove Member

### Request

`curl http://${remote_client_url}/v2/admin/members/{$id} -XDELETE`

Parameter `remote_client_url` is serving client url of the cluster.
Parameter `id` is the identification of member to be removed in hexadecimal.

### Response

#### HTTP 204

The member is removed successfully.

#### HTTP 400

etcd cannot parse out the request.

#### HTTP 500

etcd fails to remove the member.

*/
package etcdhttp
