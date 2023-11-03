package httpspy

import (
	"database/sql"
	"errors"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// DbName is the name of sqlite database file
const DbName = "./httpspy.db"

func openDb() *sql.DB {
	db, err := sql.Open("sqlite3", DbName)
	if err != nil {
		panic(err)
	}

	ensureTables(db)

	return db
}

func doesDbExist() bool {
	_, err := os.Stat(DbName)
	return os.IsNotExist(err)
}

func ensureTables(db *sql.DB) {
	// create tables
	_, err := db.Exec(`
		create table if not exists requests (
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

func getRequests(db *sql.DB, url string) (requests []Request, err error) {
	rows, err := db.Query(`
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

func writeRequest(db *sql.DB, req *Request) {
	res, err := db.Exec(`
		insert into requests(method, url, headers, body, timestamp) values ($1, $2, $3, $4, $5) returning id
	`, req.Method, req.URL, req.Headers, req.Body, req.Timestamp)
	if err != nil {
		req.dbErrorChan <- err
		req.idChan <- -1
		return
	}

	id, err := res.LastInsertId()
	req.dbErrorChan <- err
	req.idChan <- id
}
