package httpspy

import (
	"database/sql"
	"errors"
	"os"

	// import the sqlite db driver
	_ "github.com/mattn/go-sqlite3"
)

const dbName = "./httpspy.db"

// DB encapsulates the db for the app
type DB struct {
	db *sql.DB
}

// OpenDb gets a database connection and ensures that the tables
func OpenDb() DB {
	dbExists := doesDbExist()

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		panic(err)
	}

	if dbExists {
		ensureTables(db)
	}

	return DB{db}
}

// Close closes the database and panics if there is an error.  If the database is already closed, it does nothing
func (db DB) Close() {
	if db.db == nil {
		return
	}

	err := db.db.Close()
	if err != nil {
		panic(err)
	}

	db.db = nil
}

func doesDbExist() bool {
	_, err := os.Stat(dbName)
	return os.IsNotExist(err)
}

func ensureTables(db *sql.DB) {
	// create tables
	_, err := db.Exec(`
		create table requests (
			id        integer not null primary key,
			method    text,
			url       text,
			headers   text,
			body      text,
			timestamp datetime
		)
	`)
	if err != nil {
		panic(err)
	}
}

// GetRequests returns a list of all the requests which have a URL that matches the url parameter
func (db DB) GetRequests(url string) (requests []Request, err error) {
	rows, err := db.db.Query(`
		select id, method, url, headers, body, timestamp from requests where ''=$1 or url=$1 order by timestamp desc
	`, url)
	if err != nil {
		return requests, err
	}
	defer rows.Close()

	for rows.Next() {
		var r Request

		rowErr := rows.Scan(&r.ID, &r.Method, &r.URL, &r.Headers, &r.Body, &r.Timestamp)
		err = errors.Join(err, rowErr)

		requests = append(requests, r)
	}
	return requests, err
}

// WriteRequest writes a request to the DB, returns the id of the new record and any error
func (db DB) WriteRequest(req Request) (int64, error) {
	res, err := db.db.Exec(`
		insert into requests(method, url, headers, body, timestamp) values ($1, $2, $3, $4, $5) returning id
	`, req.Method, req.URL, req.Headers, req.Body, req.Timestamp)
	if err != nil {
		return -1, err
	}

	return res.LastInsertId()
}
