package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"
)

type App struct {
	router *mux.Router
	db     *sql.DB
}

func NewApp() *App {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "vector.db"
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(1)

	app := &App{
		router: mux.NewRouter(),
		db:     db,
	}

	if err := app.setupDB(); err != nil {
		log.Fatal(err)
	}

	app.routes()

	return app
}

func (app *App) routes() {
	app.router.HandleFunc("/signup", app.handleSignupPage).Methods("GET")
	app.router.HandleFunc("/signup", app.handleSignup).Methods("POST")
	app.router.HandleFunc("/login", app.handleLoginPage).Methods("GET")
	app.router.HandleFunc("/login", app.handleLogin).Methods("POST")
	app.router.HandleFunc("/logout", app.handleLogout).Methods("POST")
	app.router.HandleFunc("/admin", app.handleAdmin).Methods("GET")
	app.router.HandleFunc("/admin/role", app.handleSetRole).Methods("POST")

	app.router.HandleFunc("/", app.handleHome).Methods("GET")
	app.router.HandleFunc("/manage", app.handleManageTemplates).Methods("GET")
	app.router.HandleFunc("/manage/groups", app.handleNewGroups).Methods("GET")
	app.router.HandleFunc("/manage/sets", app.handleNewSets).Methods("GET")
	app.router.HandleFunc("/manage/templates", app.handleManageTemplateList).Methods("GET")
	app.router.HandleFunc("/new", app.handleLegacyNewTemplate).Methods("GET")
	app.router.HandleFunc("/new/groups", app.handleNewGroups).Methods("GET")
	app.router.HandleFunc("/new/sets", app.handleNewSets).Methods("GET")
	app.router.HandleFunc("/groups", app.handleGroups).Methods("GET")
	app.router.HandleFunc("/sets", app.handleSets).Methods("GET")
	app.router.HandleFunc("/templates", app.handleTemplates).Methods("GET")
	app.router.HandleFunc("/templates", app.handleCreateTemplate).Methods("POST")
	app.router.HandleFunc("/templates/{id:[0-9]+}", app.handleDeleteTemplate).Methods("DELETE")
	app.router.HandleFunc("/templates/{id:[0-9]+}/delete", app.handleDeleteTemplateFallback).Methods("POST")
}

func (app *App) Run() {
	log.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", app.router))
}

func main() {
	app := NewApp()
	defer app.db.Close()

	app.Run()
}
