package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"
)

type App struct {
	router *mux.Router
	db     *sql.DB
}

type PageData struct {
	Title       string
	Departments []string
}

type DropdownData struct {
	Department string
	Groups     []string
	Sets       []string
}

type TemplateItem struct {
	ID         int
	Department string
	Group      string
	Set        string
	Title      string
	Body       string
}

type TemplatesData struct {
	Templates []TemplateItem
}

type TemplateForm struct {
	Department    string
	NewDepartment string
	Group         string
	NewGroup      string
	Set           string
	NewSet        string
	Title         string
	Body          string
}

type NewTemplateData struct {
	Title       string
	Departments []string
	Saved       bool
	Error       string
	Values      TemplateForm
}

func NewApp() *App {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "vector.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

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
	app.router.HandleFunc("/", app.handleHome).Methods("GET")
	app.router.HandleFunc("/new", app.handleNewTemplate).Methods("GET")
	app.router.HandleFunc("/new/groups", app.handleNewGroups).Methods("GET")
	app.router.HandleFunc("/new/sets", app.handleNewSets).Methods("GET")
	app.router.HandleFunc("/groups", app.handleGroups).Methods("GET")
	app.router.HandleFunc("/sets", app.handleSets).Methods("GET")
	app.router.HandleFunc("/templates", app.handleTemplates).Methods("GET")
	app.router.HandleFunc("/templates", app.handleCreateTemplate).Methods("POST")
}

func (app *App) Run() {
	log.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", app.router))
}

// Handlers
func (app *App) handleHome(w http.ResponseWriter, r *http.Request) {
	departments, err := app.listDepartments()
	if err != nil {
		http.Error(w, "Could not load departments", http.StatusInternalServerError)
		log.Println("load departments:", err)
		return
	}

	app.render(w, "home.html", PageData{
		Title:       "Vector",
		Departments: departments,
	})
}

func (app *App) handleGroups(w http.ResponseWriter, r *http.Request) {
	department := r.URL.Query().Get("department")

	groups, err := app.listGroups(department)
	if err != nil {
		http.Error(w, "Could not load groups", http.StatusInternalServerError)
		log.Println("load groups:", err)
		return
	}

	app.renderPartial(w, "groups.html", DropdownData{
		Department: department,
		Groups:     groups,
	})
}

func (app *App) handleSets(w http.ResponseWriter, r *http.Request) {
	department := r.URL.Query().Get("department")
	group := r.URL.Query().Get("group")

	sets, err := app.listSets(department, group)
	if err != nil {
		http.Error(w, "Could not load sets", http.StatusInternalServerError)
		log.Println("load sets:", err)
		return
	}

	app.renderPartial(w, "sets.html", DropdownData{
		Department: department,
		Sets:       sets,
	})
}

func (app *App) handleTemplates(w http.ResponseWriter, r *http.Request) {
	department := r.URL.Query().Get("department")
	group := r.URL.Query().Get("group")
	set := r.URL.Query().Get("set")

	templates, err := app.listTemplates(department, group, set)
	if err != nil {
		http.Error(w, "Could not load templates", http.StatusInternalServerError)
		log.Println("load templates:", err)
		return
	}

	app.renderPartial(w, "templates.html", TemplatesData{
		Templates: templates,
	})
}

func (app *App) handleNewTemplate(w http.ResponseWriter, r *http.Request) {
	departments, err := app.listDepartments()
	if err != nil {
		http.Error(w, "Could not load departments", http.StatusInternalServerError)
		log.Println("load departments:", err)
		return
	}

	app.render(w, "new.html", NewTemplateData{
		Title:       "Add Template",
		Departments: departments,
		Saved:       r.URL.Query().Get("saved") == "1",
	})
}

func (app *App) handleNewGroups(w http.ResponseWriter, r *http.Request) {
	department := r.URL.Query().Get("department")

	groups, err := app.listGroups(department)
	if err != nil {
		http.Error(w, "Could not load groups", http.StatusInternalServerError)
		log.Println("load groups:", err)
		return
	}

	app.renderPartial(w, "new_groups.html", DropdownData{
		Department: department,
		Groups:     groups,
	})
}

func (app *App) handleNewSets(w http.ResponseWriter, r *http.Request) {
	department := r.URL.Query().Get("department")
	group := r.URL.Query().Get("group")

	sets, err := app.listSets(department, group)
	if err != nil {
		http.Error(w, "Could not load sets", http.StatusInternalServerError)
		log.Println("load sets:", err)
		return
	}

	app.renderPartial(w, "new_sets.html", DropdownData{
		Department: department,
		Sets:       sets,
	})
}

func (app *App) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Could not read form", http.StatusBadRequest)
		return
	}

	values := TemplateForm{
		Department:    r.FormValue("department"),
		NewDepartment: r.FormValue("department_new"),
		Group:         r.FormValue("group"),
		NewGroup:      r.FormValue("group_new"),
		Set:           r.FormValue("set"),
		NewSet:        r.FormValue("set_new"),
		Title:         strings.TrimSpace(r.FormValue("title")),
		Body:          strings.TrimSpace(r.FormValue("body")),
	}

	department := pickValue(values.Department, values.NewDepartment)
	group := pickValue(values.Group, values.NewGroup)
	set := pickValue(values.Set, values.NewSet)

	if department == "" || group == "" || set == "" || values.Title == "" || values.Body == "" {
		app.renderNewTemplateError(w, values, "Please fill in department, group, set, title, and body.")
		return
	}

	err = app.createTemplate(department, group, set, values.Title, values.Body)
	if err != nil {
		http.Error(w, "Could not save template", http.StatusInternalServerError)
		log.Println("save template:", err)
		return
	}

	http.Redirect(w, r, "/new?saved=1", http.StatusSeeOther)
}

// Rendering htmx content
func (app *App) render(w http.ResponseWriter, page string, data any) {
	files := []string{
		"templates/index.html",
		"templates/" + page,
	}

	views, err := template.ParseFiles(files...)
	if err != nil {
		http.Error(w, "Template parse error", http.StatusInternalServerError)
		log.Println("template parse error:", err)
		return
	}

	err = views.ExecuteTemplate(w, "index", data)
	if err != nil {
		http.Error(w, "Template render error", http.StatusInternalServerError)
		log.Println("template render error:", err)
		return
	}
}

func (app *App) renderPartial(w http.ResponseWriter, page string, data any) {
	views, err := template.ParseFiles("templates/partials/" + page)
	if err != nil {
		http.Error(w, "Template parse error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	err = views.ExecuteTemplate(w, page, data)
	if err != nil {
		http.Error(w, "Template render error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

func (app *App) setupDB() error {
	_, err := app.db.Exec(`
		CREATE TABLE IF NOT EXISTS templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			department TEXT NOT NULL,
			group_name TEXT NOT NULL,
			set_name TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL
		);
	`)
	return err
}

func (app *App) renderNewTemplateError(w http.ResponseWriter, values TemplateForm, message string) {
	departments, err := app.listDepartments()
	if err != nil {
		http.Error(w, "Could not load departments", http.StatusInternalServerError)
		log.Println("load departments:", err)
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	app.render(w, "new.html", NewTemplateData{
		Title:       "Add Template",
		Departments: departments,
		Error:       message,
		Values:      values,
	})
}

func (app *App) listDepartments() ([]string, error) {
	rows, err := app.db.Query(`
		SELECT DISTINCT department
		FROM templates
		ORDER BY department;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var departments []string
	for rows.Next() {
		var department string
		if err := rows.Scan(&department); err != nil {
			return nil, err
		}
		departments = append(departments, department)
	}

	return departments, rows.Err()
}

func (app *App) listGroups(department string) ([]string, error) {
	rows, err := app.db.Query(`
		SELECT DISTINCT group_name
		FROM templates
		WHERE department = ?
		ORDER BY group_name;
	`, department)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var group string
		if err := rows.Scan(&group); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}

	return groups, rows.Err()
}

func (app *App) listSets(department string, group string) ([]string, error) {
	rows, err := app.db.Query(`
		SELECT DISTINCT set_name
		FROM templates
		WHERE department = ? AND group_name = ?
		ORDER BY set_name;
	`, department, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sets []string
	for rows.Next() {
		var set string
		if err := rows.Scan(&set); err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}

	return sets, rows.Err()
}

func (app *App) listTemplates(department string, group string, set string) ([]TemplateItem, error) {
	rows, err := app.db.Query(`
		SELECT id, department, group_name, set_name, title, body
		FROM templates
		WHERE department = ? AND group_name = ? AND set_name = ?
		ORDER BY title;
	`, department, group, set)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []TemplateItem
	for rows.Next() {
		var item TemplateItem
		if err := rows.Scan(&item.ID, &item.Department, &item.Group, &item.Set, &item.Title, &item.Body); err != nil {
			return nil, err
		}
		templates = append(templates, item)
	}

	return templates, rows.Err()
}

func (app *App) createTemplate(department string, group string, set string, title string, body string) error {
	_, err := app.db.Exec(`
		INSERT INTO templates (department, group_name, set_name, title, body)
		VALUES (?, ?, ?, ?, ?);
	`, department, group, set, title, body)
	return err
}

func pickValue(existing string, newValue string) string {
	newValue = strings.TrimSpace(newValue)
	if newValue != "" {
		return newValue
	}

	return strings.TrimSpace(existing)
}

func main() {
	app := NewApp()
	defer app.db.Close()

	app.Run()
}
