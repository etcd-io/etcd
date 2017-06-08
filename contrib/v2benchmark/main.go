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
    "github.com/coreos/etcd/Godeps/_workspace/src/github.com/cheggaaa/pb"
    "fmt"
  )

var endpoint, host, path string
var port,limit int

func do(endpoint string, id int, method string, value []byte, c chan string ) {
    req, err := http.NewRequest(method, endpoint+strconv.Itoa(id), bytes.NewBuffer(value))
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    c <- string(body)
}

func init()  {
  host = "127.0.0.1";
  path = "/v2/keys/";
  port = 2379;
  limit = 20000;
  endpoint = "http://"+host+":"+strconv.Itoa(port)+path
}


func perform(httpMethod string)  {
  respMap := make(map[int]string)
  bar := pb.New(limit)
  var payload string
  barMsg := "Start of #"+strconv.Itoa(limit)+" Concurrent "+httpMethod+" Request on "+path
  bar.Start()

  if httpMethod == "POST" {
	   payload = `{"value":"New value"+strconv.Itoa(id),"ttl":5}`
  }else if httpMethod == "PUT" {
	   payload = `{"value":"Updated value : \t"+strconv.Itoa(id)}`
  }else if httpMethod == "DELETE" {
    endpoint += "?recursive=true"
  }
  fmt.Println(barMsg)
  for i := 0; i < limit; i++ {
      //Spawning an independent goroutine to make HTTP requests
      resp := make(chan string)
      go do(endpoint, i, httpMethod, ([]byte)(payload), resp)
       respMap[i] = <- resp
       bar.Increment()

  }
  bar.FinishPrint("Done!\n")
}

func main() {
  perform("POST")
  perform("GET")
  perform("PUT")
  perform("DELETE")
}
