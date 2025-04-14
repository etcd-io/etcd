#!/usr/bin/env -S python3 -u

# This file serves as the client's entrypoint. It: 
# 1. Confirms that all nodes in the cluster are available
# 2. Signals "setupComplete" using the Antithesis SDK

import etcd3, time

from antithesis.lifecycle import (
    setup_complete,
)

SLEEP = 10

def check_health():

    node_options = ["etcd0", "etcd1", "etcd2"]

    for i in range(0, len(node_options)):
        try:
            c = etcd3.client(host=node_options[i], port=2379)
            c.get('setting-up')
            print(f"Client [entrypoint]: connection successful with {node_options[i]}")
        except Exception as e:
            print(f"Client [entrypoint]: connection failed with {node_options[i]}")
            print(f"Client [entrypoint]: error: {e}")
            return False
    return True
    
print("Client [entrypoint]: starting...")

while True:
    print("Client [entrypoint]: checking cluster health...")
    if check_health():
        print("Client [entrypoint]: cluster is healthy!")
        break
    else:
        print(f"Client [entrypoint]: cluster is not healthy. retrying in {SLEEP} seconds...")
        time.sleep(SLEEP)


# Here is the python format for setup_complete. At this point, our system is fully initialized and ready to test.
setup_complete({"Message":"ETCD cluster is healthy"})

# sleep infinity
time.sleep(31536000)
