# Logging Conventions

etcd uses the [zap][zap] library for logging application output categorized into *levels*. A log message's level is determined according to these conventions:

* Debug: Everything is still fine, but even common operations may be logged, and less helpful but more quantity of notices. Usually not used in production.
  * Examples:
    * Send a normal message to a remote peer
    * Write a log entry to disk

* Info: Normal, working log information, everything is fine, but helpful notices for auditing or common operations. Should rather not be logged more frequently than once per a few seconds in a normal server's operation.
  * Examples:
    * Startup configuration
    * Start to do a snapshot

* Warning: (Hopefully) Temporary conditions that may cause errors, but may work fine. A replica disappearing (that may reconnect) is a warning.
  * Examples:
    * Failure to send a raft message to a remote peer
    * Failure to receive heartbeat message within the configured election timeout

* Error: Data has been lost, a request has failed for a bad reason, or a required resource has been lost.
  * Examples:
    * Failure to allocate disk space for WAL

* Panic: Unrecoverable or unexpected error situation that requires stopping execution.
  * Examples:
    * Failure to create the database

* Fatal: Unrecoverable or unexpected error situation that requires immediate exit. Mostly used in the test.
  * Examples:
    * Failure to find the data directory
    * Failure to run a test function

[zap]: https://github.com/uber-go/zap
