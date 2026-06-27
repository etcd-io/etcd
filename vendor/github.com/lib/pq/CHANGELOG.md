unreleased
----------


v1.12.3 (2026-04-03)
--------------------
- Send datestyle startup parameter, improving compatbility with database engines
  that use a different default datestyle such as EnterpriseDB ([#1312]).

[#1312]: https://github.com/lib/pq/pull/1312

v1.12.2 (2026-04-02)
--------------------

- Treat io.ErrUnexpectedEOF as driver.ErrBadConn so database/sql discards the
  connection. Since v1.12.0 this could result in permanently broken connections,
  especially with CockroachDB which frequently sends partial messages ([#1299]).

[#1299]: https://github.com/lib/pq/pull/1299

v1.12.1 (2026-03-30)
--------------------

- Look for pgpass file in ~/.pgpass instead of ~/.postgresql/pgpass ([#1300]).

- Don't clear password if directly set on pq.Config ([#1302]).

[#1300]: https://github.com/lib/pq/pull/1300
[#1302]: https://github.com/lib/pq/pull/1302

v1.12.0 (2026-03-18)
--------------------

- The next release may change the default sslmode from `require` to `prefer`.
  See [#1271] for details.

- `CopyIn()` and `CopyInToSchema()` have been marked as deprecated. These are
  simple query builders and not needed for `COPY [..] FROM STDIN` support (which
  is *not* deprecated). ([#1279])

      // Old
      tx.Prepare(CopyIn("temp", "num", "text", "blob", "nothing"))

      // Replacement
      tx.Prepare(`copy temp (num, text, blob, nothing) from stdin`)

### Features

- Support protocol 3.2, and the `min_protocol_version` and
  `max_protocol_version` DSN parameters ([#1258]).

- Support `sslmode=prefer` and `sslmode=allow` ([#1270]).

- Support `ssl_min_protocol_version` and `ssl_max_protocol_version` ([#1277]).

- Support connection service file to load connection details ([#1285]).

- Support `sslrootcert=system` and use `~/.postgresql/root.crt` as the default
  value of sslrootcert ([#1280], [#1281]).

- Add a new `pqerror` package with PostgreSQL error codes ([#1275]).

  For example, to test if an error is a UNIQUE constraint violation:

      if pqErr, ok := errors.AsType[*pq.Error](err); ok && pqErr.Code == pqerror.UniqueViolation {
          log.Fatalf("email %q already exsts", email)
      }

  To make this a bit more convenient, it also adds a `pq.As()` function:

      pqErr := pq.As(err, pqerror.UniqueViolation)
      if pqErr != nil {
          log.Fatalf("email %q already exsts", email)
      }

### Fixes

- Fix SSL key permission check to allow modes stricter than 0600/0640#1265 ([#1265]).

- Fix Hstore to work with binary parameters ([#1278]).

- Clearer error when starting a new query while pq is still processing another
  query ([#1272]).

- Send intermediate CAs with client certificates, so they can be signed by an
  intermediate CA ([#1267]).

- Use `time.UTC` for UTC aliases such as `Etc/UTC` ([#1282]).

[#1258]: https://github.com/lib/pq/pull/1258
[#1265]: https://github.com/lib/pq/pull/1265
[#1267]: https://github.com/lib/pq/pull/1267
[#1270]: https://github.com/lib/pq/pull/1270
[#1271]: https://github.com/lib/pq/pull/1271
[#1272]: https://github.com/lib/pq/pull/1272
[#1275]: https://github.com/lib/pq/pull/1275
[#1277]: https://github.com/lib/pq/pull/1277
[#1278]: https://github.com/lib/pq/pull/1278
[#1279]: https://github.com/lib/pq/pull/1279
[#1280]: https://github.com/lib/pq/pull/1280
[#1281]: https://github.com/lib/pq/pull/1281
[#1282]: https://github.com/lib/pq/pull/1282
[#1283]: https://github.com/lib/pq/pull/1283
[#1285]: https://github.com/lib/pq/pull/1285

v1.11.2 (2026-02-10)
--------------------
This fixes two regressions:

- Don't send startup parameters if there is no value, improving compatibility
  with Supavisor ([#1260]).

- Don't send `dbname` as a startup parameter if `database=[..]` is used in the
  connection string. It's recommended to use dbname=, as database= is not a
  libpq option, and only worked by accident previously. ([#1261])

[#1260]: https://github.com/lib/pq/pull/1260
[#1261]: https://github.com/lib/pq/pull/1261

v1.11.1 (2026-01-29)
--------------------
This fixes two regressions present in the v1.11.0 release:

- Fix build on 32bit systems, Windows, and Plan 9 ([#1253]).

- Named []byte types and pointers to []byte (e.g. `*[]byte`, `json.RawMessage`)
  would be treated as an array instead of bytea ([#1252]).

[#1252]: https://github.com/lib/pq/pull/1252
[#1253]: https://github.com/lib/pq/pull/1253

v1.11.0 (2026-01-28)
--------------------
This version of pq requires Go 1.21 or newer.

pq now supports only maintained PostgreSQL releases, which is PostgreSQL 14 and
newer. Previously PostgreSQL 8.4 and newer were supported.

### Features

- The `pq.Error.Error()` text  includes the position of the error (if reported
  by PostgreSQL) and SQLSTATE code ([#1219], [#1224]):

      pq: column "columndoesntexist" does not exist at column 8 (42703)
      pq: syntax error at or near ")" at position 2:71 (42601)

- The `pq.Error.ErrorWithDetail()` method prints a more detailed multiline
  message, with the Detail, Hint, and error position (if any) ([#1219]):

      ERROR:   syntax error at or near ")" (42601)
      CONTEXT: line 12, column 1:

           10 |     name           varchar,
           11 |     version        varchar,
           12 | );
                ^

- Add `Config`, `NewConfig()`, and `NewConnectorConfig()` to supply connection
  details in a more structured way ([#1240]).

- Support `hostaddr` and `$PGHOSTADDR` ([#1243]).

- Support multiple values in `host`, `port`, and `hostaddr`, which are each
  tried in order, or randomly if `load_balance_hosts=random` is set ([#1246]).

- Support `target_session_attrs` connection parameter ([#1246]).

- Support [`sslnegotiation`] to use SSL without negotiation ([#1180]).

- Allow using a custom `tls.Config`, for example for encrypted keys ([#1228]).

- Add `PQGO_DEBUG=1` print the communication with PostgreSQL to stderr, to aid
  in debugging, testing, and bug reports ([#1223]).

- Add support for NamedValueChecker interface ([#1125], [#1238]).


### Fixes

- Match HOME directory lookup logic with libpq: prefer $HOME over /etc/passwd,
  ignore ENOTDIR errors, and use APPDATA on Windows ([#1214]).

- Fix `sslmode=verify-ca` verifying the hostname anyway when connecting to a DNS
  name (rather than IP) ([#1226]).

- Correctly detect pre-protocol errors such as the server not being able to fork
  or running out of memory ([#1248]).

- Fix build with wasm ([#1184]), appengine ([#745]), and Plan 9 ([#1133]).

- Deprecate and type alias `pq.NullTime` to `sql.NullTime` ([#1211]).

- Enforce integer limits of the Postgres wire protocol ([#1161]).

- Accept the `passfile` connection parameter to override `PGPASSFILE` ([#1129]).

- Fix connecting to socket on Windows systems ([#1179]).

- Don't perform a permission check on the .pgpass file on Windows ([#595]).

- Warn about incorrect .pgpass permissions ([#595]).

- Don't set extra_float_digits ([#1212]).

- Decode bpchar into a string ([#949]).

- Fix panic in Ping() by not requiring CommandComplete or EmptyQueryResponse in
  simpleQuery() ([#1234])

- Recognize bit/varbit ([#743]) and float types ([#1166]) in ColumnTypeScanType().

- Accept `PGGSSLIB` and `PGKRBSRVNAME` environment variables ([#1143]).

- Handle ErrorResponse in readReadyForQuery and return proper error ([#1136]).

- Detect COPY even if the query starts with whitespace or comments ([#1198]).

- CopyIn() and CopyInSchema() now work if the list of columns is empty, in which
  case it will copy all columns ([#1239]).

- Treat nil []byte in query parameters as nil/NULL rather than `""` ([#838]).

- Accept multiple authentication methods before checking AuthOk, which improves
  compatibility with PgPool-II ([#1188]).

[`sslnegotiation`]: https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNECT-SSLNEGOTIATION
[#595]: https://github.com/lib/pq/pull/595
[#745]: https://github.com/lib/pq/pull/745
[#743]: https://github.com/lib/pq/pull/743
[#838]: https://github.com/lib/pq/pull/838
[#949]: https://github.com/lib/pq/pull/949
[#1125]: https://github.com/lib/pq/pull/1125
[#1129]: https://github.com/lib/pq/pull/1129
[#1133]: https://github.com/lib/pq/pull/1133
[#1136]: https://github.com/lib/pq/pull/1136
[#1143]: https://github.com/lib/pq/pull/1143
[#1161]: https://github.com/lib/pq/pull/1161
[#1166]: https://github.com/lib/pq/pull/1166
[#1179]: https://github.com/lib/pq/pull/1179
[#1180]: https://github.com/lib/pq/pull/1180
[#1184]: https://github.com/lib/pq/pull/1184
[#1188]: https://github.com/lib/pq/pull/1188
[#1198]: https://github.com/lib/pq/pull/1198
[#1211]: https://github.com/lib/pq/pull/1211
[#1212]: https://github.com/lib/pq/pull/1212
[#1214]: https://github.com/lib/pq/pull/1214
[#1219]: https://github.com/lib/pq/pull/1219
[#1223]: https://github.com/lib/pq/pull/1223
[#1224]: https://github.com/lib/pq/pull/1224
[#1226]: https://github.com/lib/pq/pull/1226
[#1228]: https://github.com/lib/pq/pull/1228
[#1234]: https://github.com/lib/pq/pull/1234
[#1238]: https://github.com/lib/pq/pull/1238
[#1239]: https://github.com/lib/pq/pull/1239
[#1240]: https://github.com/lib/pq/pull/1240
[#1243]: https://github.com/lib/pq/pull/1243
[#1246]: https://github.com/lib/pq/pull/1246
[#1248]: https://github.com/lib/pq/pull/1248


v1.10.9 (2023-04-26)
--------------------
- Fixes backwards incompat bug with 1.13.

- Fixes pgpass issue
