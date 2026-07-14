package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "file:../../vector.db?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if len(os.Args) == 4 && os.Args[1] == "setrole" {
		res, err := db.Exec("UPDATE users SET role = ? WHERE email = ?", os.Args[3], os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		n, _ := res.RowsAffected()
		fmt.Printf("updated %d row(s)\n", n)
	}

	rows, err := db.Query("SELECT id, email, role FROM users ORDER BY id")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var email, role string
		if err := rows.Scan(&id, &email, &role); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d\t%s\t%s\n", id, email, role)
	}
}
