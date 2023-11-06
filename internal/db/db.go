// Package db contains the database access routines for the app
package db

import (
	"database/sql"
	"fmt"
	"os"

	// import the sqlite db driver
	_ "github.com/mattn/go-sqlite3"
	"www.github.com/spirozh/httpspy/internal/data"
)

const dbName = "./httpspy.db"

// DB encapsulates the db for the app
type DB struct {
	sql         *sql.DB
	allRequests []data.Request
}

// OpenDb gets a database connection,  ensures that the tables exist, and loads the existing requests into memory
func OpenDb() DB {
	var db DB
	_, err := os.Stat(dbName)
	dbFileExists := os.IsNotExist(err)

	sql, err := sql.Open("sqlite3", dbName)
	if err != nil {
		panic(err)
	}

	db.sql = sql

	if dbFileExists {
		db.ensureTables()
	}

	db.allRequests = []data.Request{}
	db.loadRequests()

	return db
}

// Close closes the database and panics if there is an error.  If the database is already closed, it does nothing
func (db *DB) Close() {
	if db.sql == nil {
		return
	}

	err := db.sql.Close()
	if err != nil {
		panic(err)
	}
	fmt.Println("db closed")

	db.sql = nil
}

func (db *DB) ensureTables() {
	// create tables
	_, err := db.sql.Exec(`
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

func (db *DB) loadRequests() {
	res, err := db.sql.Query(`
		select * from requests
	`)
	if err != nil {
		panic("could not load tables")
	}

	for res.Next() {
		var req data.Request
		err := res.Scan(&req.ID, &req.Method, &req.URL, &req.Headers, &req.Body, &req.Timestamp)
		if err != nil {
			panic(fmt.Errorf("failed to scan: %w", err))
		}
		db.allRequests = append(db.allRequests, req)
	}
}

// GetRequests returns a list of all the requests
func (db *DB) GetRequests() (requests []data.Request) {
	return db.allRequests
}

// WriteRequest writes a request to the DB, puts the id into the request, and returns any error
func (db *DB) WriteRequest(req *data.Request) error {
	res, err := db.sql.Exec(`
		insert into requests(method, url, headers, body, timestamp) values ($1, $2, $3, $4, $5) returning id
	`, req.Method, req.URL, req.Headers, req.Body, req.Timestamp)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	req.ID = id
	db.allRequests = append(db.allRequests, *req)

	return err
}

// DeleteRequests deletes all requests from the DB, returns any error
func (db *DB) DeleteRequests() error {
	_, err := db.sql.Exec("delete from requests")
	db.allRequests = []data.Request{}
	return err
}
