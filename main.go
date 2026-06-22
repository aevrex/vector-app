package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
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
	Active      string
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

type ManageTemplatesListData struct {
	Templates   []TemplateItem
	SearchQuery string
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

type ManageTemplateData struct {
	Title       string
	Active      string
	Departments []string
	Saved       bool
	Deleted     bool
	Error       string
	Values      TemplateForm
	Templates   []TemplateItem
	SearchQuery string
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

// Handlers
func (app *App) handleHome(w http.ResponseWriter, r *http.Request) {
	departments, err := app.listDepartments()
	if err != nil {
		http.Error(w, "Could not load departments", http.StatusInternalServerError)
		log.Println("load departments:", err)
		return
	}

	app.render(w, "home.html", PageData{
		Title:       "Vector Library",
		Active:      "browse",
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

func (app *App) handleLegacyNewTemplate(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/manage", http.StatusSeeOther)
}

func (app *App) handleManageTemplates(w http.ResponseWriter, r *http.Request) {
	departments, err := app.listDepartments()
	if err != nil {
		http.Error(w, "Could not load departments", http.StatusInternalServerError)
		log.Println("load departments:", err)
		return
	}

	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	templates, err := app.listManagedTemplates(searchQuery)
	if err != nil {
		http.Error(w, "Could not load templates", http.StatusInternalServerError)
		log.Println("load managed templates:", err)
		return
	}

	app.render(w, "manage.html", ManageTemplateData{
		Title:       "Manage Templates",
		Active:      "manage",
		Departments: departments,
		Saved:       r.URL.Query().Get("saved") == "1",
		Deleted:     r.URL.Query().Get("deleted") == "1",
		Templates:   templates,
		SearchQuery: searchQuery,
	})
}

func (app *App) handleManageTemplateList(w http.ResponseWriter, r *http.Request) {
	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))

	templates, err := app.listManagedTemplates(searchQuery)
	if err != nil {
		http.Error(w, "Could not load templates", http.StatusInternalServerError)
		log.Println("load managed templates:", err)
		return
	}

	app.renderPartial(w, "manage_templates.html", ManageTemplatesListData{
		Templates:   templates,
		SearchQuery: searchQuery,
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

	http.Redirect(w, r, "/manage?saved=1", http.StatusSeeOther)
}

func (app *App) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := templateID(r)
	if err != nil {
		http.Error(w, "Invalid template id", http.StatusBadRequest)
		return
	}

	if err := app.deleteTemplate(id); err != nil {
		http.Error(w, "Could not delete template", http.StatusInternalServerError)
		log.Println("delete template:", err)
		return
	}

	searchQuery := strings.TrimSpace(r.URL.Query().Get("q"))
	templates, err := app.listManagedTemplates(searchQuery)
	if err != nil {
		http.Error(w, "Could not load templates", http.StatusInternalServerError)
		log.Println("load managed templates:", err)
		return
	}

	app.renderPartial(w, "manage_templates.html", ManageTemplatesListData{
		Templates:   templates,
		SearchQuery: searchQuery,
	})
}

func (app *App) handleDeleteTemplateFallback(w http.ResponseWriter, r *http.Request) {
	id, err := templateID(r)
	if err != nil {
		http.Error(w, "Invalid template id", http.StatusBadRequest)
		return
	}

	if err := app.deleteTemplate(id); err != nil {
		http.Error(w, "Could not delete template", http.StatusInternalServerError)
		log.Println("delete template:", err)
		return
	}

	http.Redirect(w, r, "/manage?deleted=1", http.StatusSeeOther)
}

// Rendering htmx content
func (app *App) render(w http.ResponseWriter, page string, data any) {
	files := []string{
		"templates/index.html",
		"templates/" + page,
		"templates/partials/manage_templates.html",
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

	templates, err := app.listManagedTemplates("")
	if err != nil {
		http.Error(w, "Could not load templates", http.StatusInternalServerError)
		log.Println("load managed templates:", err)
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	app.render(w, "manage.html", ManageTemplateData{
		Title:       "Manage Templates",
		Active:      "manage",
		Departments: departments,
		Error:       message,
		Values:      values,
		Templates:   templates,
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

func (app *App) listManagedTemplates(searchQuery string) ([]TemplateItem, error) {
	searchQuery = strings.TrimSpace(searchQuery)

	var (
		rows *sql.Rows
		err  error
	)

	if searchQuery == "" {
		rows, err = app.db.Query(`
			SELECT id, department, group_name, set_name, title, body
			FROM templates
			ORDER BY department, group_name, set_name, title;
		`)
	} else {
		likeQuery := "%" + searchQuery + "%"
		rows, err = app.db.Query(`
			SELECT id, department, group_name, set_name, title, body
			FROM templates
			WHERE department LIKE ?
				OR group_name LIKE ?
				OR set_name LIKE ?
				OR title LIKE ?
				OR body LIKE ?
			ORDER BY department, group_name, set_name, title;
		`, likeQuery, likeQuery, likeQuery, likeQuery, likeQuery)
	}
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

func (app *App) deleteTemplate(id int) error {
	_, err := app.db.Exec(`
		DELETE FROM templates
		WHERE id = ?;
	`, id)
	return err
}

func pickValue(existing string, newValue string) string {
	newValue = strings.TrimSpace(newValue)
	if newValue != "" {
		return newValue
	}

	return strings.TrimSpace(existing)
}

func templateID(r *http.Request) (int, error) {
	return strconv.Atoi(mux.Vars(r)["id"])
}

func main() {
	app := NewApp()
	defer app.db.Close()

	app.Run()
}
