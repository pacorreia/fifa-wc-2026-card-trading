package db

import (
"context"
"database/sql"
"database/sql/driver"
"regexp"
"strings"
"time"

_ "modernc.org/sqlite"
)

const sqliteDriverName = "sqlite-pg"

var pgArgRE = regexp.MustCompile(`\$\d+`)

// translatePGToSQLite converts PostgreSQL-style SQL to SQLite-compatible SQL.
// It replaces $N positional placeholders with ? and NOW() with the SQLite UTC equivalent.
func translatePGToSQLite(query string) string {
query = pgArgRE.ReplaceAllString(query, "?")
query = strings.ReplaceAll(query, "NOW()", "strftime('%Y-%m-%dT%H:%M:%SZ', 'now')")
return query
}

func init() {
// Retrieve the driver registered by modernc.org/sqlite and wrap it.
// sql.Open is lazy and does not open real connections.
tmp, err := sql.Open("sqlite", ":memory:")
if err != nil {
panic("sqlite-pg: failed to retrieve sqlite driver: " + err.Error())
}
inner := tmp.Driver()
tmp.Close()
sql.Register(sqliteDriverName, &translatingDriver{inner: inner})
}

// translatingDriver wraps the SQLite driver with PostgreSQL->SQLite query translation.
type translatingDriver struct {
inner driver.Driver
}

func (d *translatingDriver) Open(name string) (driver.Conn, error) {
conn, err := d.inner.Open(name)
if err != nil {
return nil, err
}
return &translatingConn{inner: conn}, nil
}

// OpenConnector implements driver.DriverContext for connection pooling.
func (d *translatingDriver) OpenConnector(name string) (driver.Connector, error) {
if dc, ok := d.inner.(driver.DriverContext); ok {
connector, err := dc.OpenConnector(name)
if err != nil {
return nil, err
}
return &translatingConnector{connector: connector, driver: d}, nil
}
return &simpleConnector{dsn: name, driver: d}, nil
}

// translatingConnector implements driver.Connector.
type translatingConnector struct {
connector driver.Connector
driver    driver.Driver
}

func (c *translatingConnector) Connect(ctx context.Context) (driver.Conn, error) {
conn, err := c.connector.Connect(ctx)
if err != nil {
return nil, err
}
return &translatingConn{inner: conn}, nil
}

func (c *translatingConnector) Driver() driver.Driver { return c.driver }

// simpleConnector is a fallback connector for drivers not implementing DriverContext.
type simpleConnector struct {
dsn    string
driver driver.Driver
}

func (c *simpleConnector) Connect(_ context.Context) (driver.Conn, error) {
return c.driver.Open(c.dsn)
}

func (c *simpleConnector) Driver() driver.Driver { return c.driver }

// translatingConn wraps a driver.Conn and translates PostgreSQL SQL to SQLite syntax.
type translatingConn struct {
inner driver.Conn
}

// Prepare implements driver.Conn.
func (c *translatingConn) Prepare(query string) (driver.Stmt, error) {
stmt, err := c.inner.Prepare(translatePGToSQLite(query))
if err != nil {
return nil, err
}
return &translatingStmt{inner: stmt}, nil
}

// Close implements driver.Conn.
func (c *translatingConn) Close() error { return c.inner.Close() }

// Begin implements driver.Conn.
func (c *translatingConn) Begin() (driver.Tx, error) { return c.inner.Begin() }

// BeginTx implements driver.ConnBeginTx.
func (c *translatingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
if bt, ok := c.inner.(driver.ConnBeginTx); ok {
return bt.BeginTx(ctx, opts)
}
return c.inner.Begin()
}

// PrepareContext implements driver.ConnPrepareContext.
func (c *translatingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
var (
stmt driver.Stmt
err  error
)
if pc, ok := c.inner.(driver.ConnPrepareContext); ok {
stmt, err = pc.PrepareContext(ctx, translatePGToSQLite(query))
} else {
stmt, err = c.inner.Prepare(translatePGToSQLite(query))
}
if err != nil {
return nil, err
}
return &translatingStmt{inner: stmt}, nil
}

// ExecContext implements driver.ExecerContext.
func (c *translatingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
if ec, ok := c.inner.(driver.ExecerContext); ok {
return ec.ExecContext(ctx, translatePGToSQLite(query), args)
}
stmt, err := c.inner.Prepare(translatePGToSQLite(query))
if err != nil {
return nil, err
}
defer stmt.Close()
return stmt.Exec(namedToValues(args))
}

// QueryContext implements driver.QueryerContext.
func (c *translatingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
var (
rows driver.Rows
err  error
)
if qc, ok := c.inner.(driver.QueryerContext); ok {
rows, err = qc.QueryContext(ctx, translatePGToSQLite(query), args)
} else {
var stmt driver.Stmt
stmt, err = c.inner.Prepare(translatePGToSQLite(query))
if err != nil {
return nil, err
}
defer stmt.Close()
rows, err = stmt.Query(namedToValues(args))
}
if err != nil {
return nil, err
}
return &translatingRows{inner: rows}, nil
}

// ResetSession implements driver.SessionResetter.
func (c *translatingConn) ResetSession(ctx context.Context) error {
if sr, ok := c.inner.(driver.SessionResetter); ok {
return sr.ResetSession(ctx)
}
return nil
}

// IsValid implements driver.Validator.
func (c *translatingConn) IsValid() bool {
if v, ok := c.inner.(driver.Validator); ok {
return v.IsValid()
}
return true
}

// translatingStmt wraps driver.Stmt to ensure rows are time-converting.
type translatingStmt struct {
inner driver.Stmt
}

func (s *translatingStmt) Close() error                                    { return s.inner.Close() }
func (s *translatingStmt) NumInput() int                                    { return s.inner.NumInput() }
func (s *translatingStmt) Exec(args []driver.Value) (driver.Result, error) { return s.inner.Exec(args) }

func (s *translatingStmt) Query(args []driver.Value) (driver.Rows, error) {
rows, err := s.inner.Query(args)
if err != nil {
return nil, err
}
return &translatingRows{inner: rows}, nil
}

// QueryContext implements driver.StmtQueryContext.
func (s *translatingStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
var (
rows driver.Rows
err  error
)
if qc, ok := s.inner.(driver.StmtQueryContext); ok {
rows, err = qc.QueryContext(ctx, args)
} else {
rows, err = s.inner.Query(namedToValues(args))
}
if err != nil {
return nil, err
}
return &translatingRows{inner: rows}, nil
}

// ExecContext implements driver.StmtExecContext.
func (s *translatingStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
if ec, ok := s.inner.(driver.StmtExecContext); ok {
return ec.ExecContext(ctx, args)
}
return s.inner.Exec(namedToValues(args))
}

// translatingRows wraps driver.Rows and converts RFC3339 text values to time.Time
// so that SQLite TEXT timestamp columns can be scanned into time.Time fields.
type translatingRows struct {
inner driver.Rows
}

func (r *translatingRows) Columns() []string { return r.inner.Columns() }
func (r *translatingRows) Close() error      { return r.inner.Close() }

func (r *translatingRows) Next(dest []driver.Value) error {
if err := r.inner.Next(dest); err != nil {
return err
}
for i, v := range dest {
if s, ok := v.(string); ok {
if t, err := time.Parse(time.RFC3339, s); err == nil {
dest[i] = t
}
}
}
return nil
}

// HasNextResultSet delegates to inner if supported.
func (r *translatingRows) HasNextResultSet() bool {
if nrs, ok := r.inner.(driver.RowsNextResultSet); ok {
return nrs.HasNextResultSet()
}
return false
}

// NextResultSet delegates to inner if supported.
func (r *translatingRows) NextResultSet() error {
if nrs, ok := r.inner.(driver.RowsNextResultSet); ok {
return nrs.NextResultSet()
}
return nil
}

func namedToValues(args []driver.NamedValue) []driver.Value {
vals := make([]driver.Value, len(args))
for i, arg := range args {
vals[i] = arg.Value
}
return vals
}
