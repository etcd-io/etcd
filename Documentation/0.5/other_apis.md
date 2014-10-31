## Members API

### GET /v2/members
Return an HTTP 200 OK response code and a representation of all members in the etcd cluster:
```
    Example Request: GET 
                     http://localhost:2379/v2/members
    Response formats: JSON
    Example Response:
```
```json
    {
        "members": [
                {
                    "id":"272e204152",
                    "name":"infra1",
                    "peerURLs":[
                        "http://10.0.0.10:2379"
                    ],
                    "clientURLs":[
                            "http://10.0.0.10:2380"
                    ]
                },
                {
                    "id":"2225373f43",
                    "name":"infra2",
                    "peerURLs":[
                        "http://127.0.0.11:2379"
                    ],
                    "clientURLs":[
                        "http://127.0.0.11:2380"
                    ]
            },
        ]
    }
```

### POST /v2/members
Add a member to the cluster.
Returns an HTTP 201 response code and the representation of added member with a newly generated a memberID when successful. Returns a string describing the failure condition when unsuccessful. 

If the POST body is malformed an HTTP 400 will be returned. If the member exists in the cluster or existed in the cluster at some point in the past an HTTP 500(TODO: fix this) will be returned. If the cluster fails to process the request within timeout an HTTP 500 will be returned, though the request may be processed later.
```
    Example Request: POST
                     http://localhost:2379/v2/members
                     Body:
                     {"peerURLs":["http://10.0.0.10:2379"]}
    Respose formats: JSON
    Example Response:
```
```json
    {
        "id":"3777296169",
        "peerURLs":[
            "http://10.0.0.10:2379"
        ],
    }
```

### DELETE /v2/members/:id
Remove a member from the cluster.
Returns empty when successful. Returns a string describing the failure condition when unsuccessful. 

If the member does not exist in the cluster an HTTP 500(TODO: fix this) will be returned. If the cluster fails to process the request within timeout an HTTP 500 will be returned, though the request may be processed later.
```
    Response formats: JSON
    Example Request: DELETE
                     http://localhost:2379/v2/members/272e204152
    Example Response: Empty
```
