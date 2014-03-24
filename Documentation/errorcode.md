Error Code
======

This document describes the error code in **Etcd** project.

It's categorized into four groups:

- Command Related Error
- Post Form Related Error
- Raft Related Error
- Etcd Related Error

Error code corresponding strerror
------

    const (
        EcodeKeyNotFound    = 100
        EcodeTestFailed     = 101
        EcodeNotFile        = 102
        EcodeNoMorePeer     = 103
        EcodeNotDir         = 104
        EcodeNodeExist      = 105
        EcodeKeyIsPreserved = 106
        EcodeRootROnly      = 107

        EcodeValueRequired     = 200
        EcodePrevValueRequired = 201
        EcodeTTLNaN            = 202
        EcodeIndexNaN          = 203

        EcodeRaftInternal = 300
        EcodeLeaderElect  = 301

        EcodeWatcherCleared = 400
        EcodeEventIndexCleared = 401
    )

    // command related errors
    errors[100] = "Key Not Found"
    errors[101] = "Test Failed" //test and set
    errors[102] = "Not A File"
    errors[103] = "Reached the max number of peers in the cluster"
    errors[104] = "Not A Directory"
    errors[105] = "Already exists" // create
    errors[106] = "The prefix of given key is a keyword in etcd"
    errors[107] = "Root is read only"

    // Post form related errors
    errors[200] = "Value is Required in POST form"
    errors[201] = "PrevValue is Required in POST form"
    errors[202] = "The given TTL in POST form is not a number"
    errors[203] = "The given index in POST form is not a number"

    // raft related errors
    errors[300] = "Raft Internal Error"
    errors[301] = "During Leader Election"

    // etcd related errors
    errors[400] = "watcher is cleared due to etcd recovery"
    errors[401] = "The event in requested index is outdated and cleared"
