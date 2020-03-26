package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/hako/branca"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/joho/godotenv"
	"github.com/leogsouza/linnet/internal/handler"
	"github.com/leogsouza/linnet/internal/service"
)

func init() {
	// loads values from .env into the system
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
}

func main() {

	var (
		port        = env("PORT", "3000")
		origin      = env("ORIGIN", "http://localhost:"+port)
		databaseURL = env("DATABASE_URL", "postgres://root@127.0.0.1:26257/linnet?sslmode=disable")
		brancaKey   = env("BRANCA_KEY", "")
	)

	db, err := sql.Open("pgx", databaseURL)

	if err != nil {
		log.Fatalf("could not open db connection: %v\n", err)
		return
	}

	defer db.Close()

	err = dbMigration(db)
	if err != nil {
		log.Fatalf("could not proecess the db migration: %v\n", err)
		return
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("could not ping to db: %v\n", err)
		return
	}

	codec := branca.NewBranca(brancaKey)
	codec.SetTTL(uint32(service.TokenLifeSpan.Seconds()))

	s := service.New(db, codec, origin)

	h := handler.New(s)

	log.Printf("accepting connections on port %s", port)
	if err = http.ListenAndServe(":"+port, h); err != nil {
		log.Fatalf("could not start server: %v\n", err)
	}

}

func env(key, fallbackValue string) string {
	s := os.Getenv(key)
	if s == "" {
		return fallbackValue
	}

	return s
}

func dbMigration(db *sql.DB) error {

	driver, err := cockroachdb.WithInstance(db, &cockroachdb.Config{})
	if err != nil {
		log.Fatalf("could not get db driver instance: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations",
		"linnet",
		driver)
	log.Println("Initiation migration")
	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Println("Executing migration")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal(err)
		return err
	}
	return nil
}
