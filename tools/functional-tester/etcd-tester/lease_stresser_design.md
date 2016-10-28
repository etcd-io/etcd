# Lease Stresser overview
Lease Stresser is used to test lease functionalities in a multi-nodes etcd system under stresses and failures. 

# Why?
We want to make sure all leases functionalities works well under a failure heavy and a stressed environment. 

# How?
We will use raw RPCs from Lease and KV client to send requests to etcd server to test functionalities related to leases.

There are 4 lease RPCs we will use, `LeaseGrant()`, `LeaseRevoke()`, and `LeaseKeepAlive()`, `LeaseTimeToLive()`. In addition, we want to test `KV.Put()` to attach keys to lease.

We will call those RPCs repeatedly make sure that the those calls do what they supposed to under failures and stresses.

After calling those RPCs, we want to ensure that the following checks always hold no matter what happens to the etcd cluster.

Invariant checks:
* Leases that are kept alive don’t expire
* Keys that are attached to live leases are present
* Leases that are explicitly revoked are deleted 
* Keys that are attached to expired leases are deleted.
* Leases that have short TTL are deleted after TTL has passed

In order to test above checks for leases,
I created a new lease stresser and incorporate it to the tester.
The job of lease stresser is to repeatedly call all raw lease RPCs under failures and stresses.
We will then check to see if lease invariant are still hold.

A tester does the following repeatedly.

1. start stressers 
2. inject failure
3. recover failure
4. wait cluster become healthy
5. cancel stressers, 
6. start checkers,
7. repeat with next test case

The new lease stresser is added stressers in the stresser. When lease stresser starts, it does the following:

There are 3 types of leases we are keeping track of.
`aliveLeases` keeps all the leases that are alive kept by separate go routine 
`revokedLeases` keeps all the leases that are explicitly revoked from `aliveLeases`
`shortLivedLeases` keeps track all the leases with short TTL which are expected to expire before each test case ends.

lease stresser algo:

1. start keep lease alive go routine on `aliveLeases`
2. create leases along with keys attached to them and keeps them alive with separate keep lease alive go routine. put them in `aliveLeases`.
3. create leases with short TTL with keys attaching them. put them in `shortLivedLeases`.
4. randomly revoke leases from `aliveLeases` and put them in `revokedLeases`.  
5. keep doing 1 to 4 until the stresser is cancelled.
6. stop leases creation, revoking, and keep alive go routines if context is cancelled.
7. wait until leases in `shortLivedLeases` expires.
8. exit stresser 

Keep lease alive go routine

1. send keep alive request using `LeaseKeepAlive()`
2. receive keep alive response
3. exit and remove lease from alive map if lease is expired
4. update timestamp of lease if renew succeed
5. if context is cancelled
    1. remove lease from leaseAlive if it is about to expire (this check is crucial since lease renewing could fail multiple time because of injected failure. We don’t want a lease that hasn’t been renewed for a while just happens to expire during invariant checking phase)
    2. exit
6. repeat step 1 to 5 every 500 ms to keep lease alive for every unless it is explicitly revoked

each of the steps in lease stresser algo tests a specific functionalty of lease

step 1 tests to see if `LeaseKeepAlive()` can renew a lease in Keep lease alive go routine.

step 2 tests to see if `LeaseGrant()` can create a lease and if KV.Put() can attach keys to leases. Also it tests to see if `LeaseKeepAlive()` can keep lease alive.

step 3 tests to see if `LeaseGrant()` can create a short TTL lease and if KV.Put() can attach keys to leases. 

step 4 tests to see if `LeaseRevoke()` can revoke a lease.

step 5 stresses etcd server by creating and revoking a lot of leases.

step 7 waits for etcd to automatically delete expired leases.

leases stresser simply performs action from client side. We don't really know if any of the actions actually worked or not without verifying them.
That's where the lease checker jumps in.

The lease checker checks following fields filled by lease stresser:

1. check leases from `aliveLeases` to make sure that they are not expired and keys associate with leases are still present.
2. check leases from `revokedLeases` to make sure that they are deleted and keys associate with leases are not present.
3. check lease from shortLivedLease make sure they are deleted and keys associate with leases are not present.

check 1 checks whether `LeaseGrant()` and KV.Put() works or not. If those operation succeed during stressing phase, we expect leases and keys created by them are present when we access them again.
Also it implicitly also checks `LeaseKeepAlive()` works or not. By probability, some lease in the alive lease will not be revoked for many rounds, and keep alive go routine will always keep them alive beyond its original TTL timeframe. 

check 2 checks whether `LeaseRevoke()` works properly or not. Leases revoked successfully during stressing phase are expected to be deleted from etcd sever. Also it checks whether the attached keys are being deleted too.

check 3 checks to see if etcd server properly remove expired leases that are in the expiredLeases.

If all 3 checks pass, then we also pass the invariant check. this means that lease RPCs are functioning as expected.

Erros Handling.

The lease stresser and checker will always work if they don't experience any errors.

However, we know that errors happen all the time ina distributed system. We need to take them into consideration when lease stresser runs.

There are three type of errors that can happen at each step in the lease stresser, errors induced by fault injection, transient errors, and context cancelled error by tester.

Let’s see how each of those steps take handles errors.

Step 1 simply starts keep alive go routine on each lease in keepAliveLeases. It is the keep alive go routine handle errors. 

* case: errors induced by failure injection
    * since we know that failure will be recovered eventually, we will keep retrying on renewing lease.
* case: context cancelled
    1. remove lease from leaseAlive if it is about to expire (this check is crucial since lease renewing could fail multiple time because of injected failure. We don’t want a lease that hasn’t been renewed for a while just happens to expire during invariant checking phase)
    2. exit
* case: transient error
    * retry lease renewal since transient error is temporary.

Step 2: create leases along with keys attached to them and keeps them alive with separate keep lease alive go routine. put them in `aliveLeases`.

* case: errors induced by failure injection
    * if lease creation fails, we continue to step 3. No need to retry because step 5 will retry again.
    * if lease creation succeed but attaching keys to lease failed, we continue to step 3. No need to retry because step 5 will retry again. 
* case: context cancelled
    * context is cancelled explicitly by tester. So we just exit.
* case: transient error
    * continue to step 3. No need to retry because step 5 will retry again.

Step 3: create leases with short TTL with keys attaching them. put them in `shortLivedLeases`.

* case: errors induced by failure injection
    * if lease creation fails, we continue to step 4. No need to retry because step 5 will retry again.
    * if lease creation succeed but attaching keys to lease failed, we continue to step 4. No need to retry because step 5 will retry again. 
* case: context cancelled
    * context is cancelled explicitly by tester. So we just exit
* case: transient error
    * continue to step 4. No need to retry because step 5 will retry again.

Step 4: randomly revoke leases from `aliveLeases` and put them in `revokedLeases`.  

* case: errors induced by failure injection
    * If a lease is mark to be revoked, we must revoke it even if `LeaseRevoke()` fails go through. So we retry `LeaseRevoke()` until we know that lease is revoked.	
* case: context cancelled
    * we are unsure if revoking lease request has been processed through or not when context is called. This lease is in an ambigious state. We will simply remove it from `aliveLeases` just in case the lease is revoked.
    * In that way, every lesae's state is known before the checking phase.
* case: transient error
    * We just keep retying until that lease is revoked for sure.

Step 5 - 7 do not experience errors incured by distributed components. 

With all the errors handle correctly. We will have confidence lease stresser creating and revoking lease expectedly under failure.