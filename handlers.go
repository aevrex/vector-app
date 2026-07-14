package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type PageData struct {
	Title       string
	Active      string
	Departments []string
	User        *User
}

type DropdownData struct {
	Department string
	Groups     []string
	Sets       []string
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
	User        *User
	Saved       bool
	Deleted     bool
	Error       string
	Values      TemplateForm
	Templates   []TemplateItem
	SearchQuery string
}

func (app *App) handleHome(w http.ResponseWriter, r *http.Request) {
	user := app.requireUser(w, r)
	if user == nil {
		return
	}

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
		User:        user,
	})
}

func (app *App) handleGroups(w http.ResponseWriter, r *http.Request) {
	if app.requireUser(w, r) == nil {
		return
	}

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
	if app.requireUser(w, r) == nil {
		return
	}

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
	if app.requireUser(w, r) == nil {
		return
	}

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
	user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

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
		User:        user,
		Saved:       r.URL.Query().Get("saved") == "1",
		Deleted:     r.URL.Query().Get("deleted") == "1",
		Templates:   templates,
		SearchQuery: searchQuery,
	})
}

func (app *App) handleManageTemplateList(w http.ResponseWriter, r *http.Request) {
	if app.requireAdmin(w, r) == nil {
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

func (app *App) handleNewGroups(w http.ResponseWriter, r *http.Request) {
	if app.requireAdmin(w, r) == nil {
		return
	}

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
	if app.requireAdmin(w, r) == nil {
		return
	}

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
	user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

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
		app.renderNewTemplateError(w, user, values, "Please fill in department, group, set, title, and body.")
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
	if app.requireAdmin(w, r) == nil {
		return
	}

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
	if app.requireAdmin(w, r) == nil {
		return
	}

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

func (app *App) renderNewTemplateError(w http.ResponseWriter, user *User, values TemplateForm, message string) {
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
		User:        user,
		Error:       message,
		Values:      values,
		Templates:   templates,
	})
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
