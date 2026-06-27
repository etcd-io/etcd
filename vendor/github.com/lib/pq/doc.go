/*
Package pq is a Go PostgreSQL driver for database/sql.

Most clients will use the database/sql package instead of using this package
directly. For example:

	import (
		"database/sql"

		_ "github.com/lib/pq"
	)

	func main() {
		dsn := "user=pqgo dbname=pqgo sslmode=verify-full"
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			log.Fatal(err)
		}

		age := 21
		rows, err := db.Query("select name from users where age = $1", age)
		// …
	}

You can also connect with an URL:

	dsn := "postgres://pqgo:password@localhost/pqgo?sslmode=verify-full"
	db, err := sql.Open("postgres", dsn)

# Connection String Parameters

See [NewConfig].

# Queries

database/sql does not dictate any specific format for parameter placeholders,
and pq uses the PostgreSQL-native ordinal markers ($1, $2, etc.). The same
placeholder can be used more than once:

	rows, err := db.Query(
		`select * from users where name = $1 or age between $2 and $2 + 3`,
		"Duck", 64)

pq does not support [sql.Result.LastInsertId]. Use the RETURNING clause with a
Query or QueryRow call instead to return the identifier:

	row := db.QueryRow(`insert into users(name, age) values('Scrooge McDuck', 93) returning id`)

	var userid int
	err := row.Scan(&userid)

# Data Types

Parameters pass through [driver.DefaultParameterConverter] before they are handled
by this package. When the binary_parameters connection option is enabled, []byte
values are sent directly to the backend as data in binary format.

This package returns the following types for values from the PostgreSQL backend:

  - integer types smallint, integer, and bigint are returned as int64
  - floating-point types real and double precision are returned as float64
  - character types char, varchar, and text are returned as string
  - temporal types date, time, timetz, timestamp, and timestamptz are
    returned as time.Time
  - the boolean type is returned as bool
  - the bytea type is returned as []byte

All other types are returned directly from the backend as []byte values in text format.

# Errors

pq may return errors of type [*pq.Error] which contain error details:

	pqErr := new(pq.Error)
	if errors.As(err, &pqErr) {
	    fmt.Println("pq error:", pqErr.Code.Name())
	}

# Bulk imports

You can perform bulk imports by preparing a "COPY [..] FROM STDIN" statement in
a transaction ([sql.Tx]). The returned [sql.Stmt] handle can then be repeatedly
"executed" to copy data into the target table. After all data has been processed
you should call Exec() once with no arguments to flush all buffered data. Any
call to Exec() might return an error which should be handled appropriately, but
because of the internal buffering an error returned by Exec() might not be
related to the data passed in the call that failed.

It is not possible to COPY outside of an explicit transaction in pq.

Use nil for NULL, or explicitly add WITH NULL 'SOME STRING' (the default of \N
doesn't work).

# Notifications

PostgreSQL supports a simple publish/subscribe model using PostgreSQL's [NOTIFY] mechanism.

To start listening for notifications, you first have to open a new connection to
the database by calling [NewListener]. This connection can not be used for
anything other than LISTEN / NOTIFY. Calling Listen will open a "notification
channel"; once a notification channel is open, a notification generated on that
channel will effect a send on the Listener.Notify channel. A notification
channel will remain open until Unlisten is called, though connection loss might
result in some notifications being lost. To solve this problem, Listener sends a
nil pointer over the Notify channel any time the connection is re-established
following a connection loss. The application can get information about the state
of the underlying connection by setting an event callback in the call to
NewListener.

A single [Listener] can safely be used from concurrent goroutines, which means
that there is often no need to create more than one Listener in your
application. However, a Listener is always connected to a single database, so
you will need to create a new Listener instance for every database you want to
receive notifications in.

The channel name in both Listen and Unlisten is case sensitive, and can contain
any characters legal in an [identifier]. Note that the channel name will be
truncated to 63 bytes by the PostgreSQL server.

# Kerberos Support

If you need support for Kerberos authentication, add the following to your main
package:

	import "github.com/lib/pq/auth/kerberos"

	func init() {
		pq.RegisterGSSProvider(func() (pq.Gss, error) { return kerberos.NewGSS() })
	}

This package is in a separate module so that users who don't need Kerberos don't
have to add unnecessary dependencies.

[identifier]: http://www.postgresql.org/docs/current/static/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS
[NOTIFY]: http://www.postgresql.org/docs/current/static/sql-notify.html
*/
package pq
