// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
    "io/ioutil"
    "net/http"
    "bytes"
    "strconv"
  )

var endpoint, host, path string
var port,limit int

func HitETCD(endpoint string, id int, c chan string ) {
    //ETCD Keys endpoint
    var value string = `{"value":"New value"+strconv.Itoa(id)}`
    var jsonStr = []byte(value)
    req, err := http.NewRequest("POST", endpoint+strconv.Itoa(id), bytes.NewBuffer(jsonStr))
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    c <- string(body)
}

func main() {
    respMap := make(map[int]string)
    host = "127.0.0.1";
    path = "/v2/keys/";
    port = 2379;
    limit = 9000;
    endpoint = "http://"+host+":"+strconv.Itoa(port)+path

    for i := 0; i < limit; i++ {
        //Spawning an independent goroutine to make HTTP get calls
        resp := make(chan string)
         go HitETCD(endpoint, i, resp)
         respMap[i] = <- resp
    }
    println("# of Concurrent Request made",len(respMap))
}
