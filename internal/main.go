package main

import (
	"database/sql"
	"log"

	"github.com/leogsouza/linnet/internal/service"
)

const (
	databaseURL = "postgresql://root@127.0.0.1:26257?sslmode=disable"
	port        = 3000
)

func main() {
	db, err := sql.Open("pgx", databaseURL)

	if err != nil {
		log.Fatalf("could not open db connection: %v\n", err)
		return
	}

	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("could not ping to db: %v\n", err)
		return
	}

	codec := branca.newBranca("my-secret-string-key")

	log.Println(codec)

	s := service.Service{
		DB:    db,
		Codec: codec,
	}
}
