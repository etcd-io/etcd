package main

import (
    "io/ioutil"
    "net/http"
    "bytes"
    "strconv"
  )

var endpoint, host, path string
var port,limit int

func hitETCD( id int, c chan string ) {
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

    host = "127.0.0.1";
    path = "/v2/keys/";
    port = 2379;
    limit = 9000;

    endpoint = "http://"+host+":"+strconv.Itoa(port)+path

    respMap := make(map[int]string)

    for i := 0; i < limit; i++ {
        //Spawning an independent goroutine to make HTTP get calls
        resp := make(chan string)
         go hitETCD(i, resp)
         respMap[i] = <- resp
    }
    println("# of Concurrent Request made",len(respMap))
}
