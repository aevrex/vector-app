package main

import (
	"database/sql"
	"strings"
)

type TemplateItem struct {
	ID         int
	Department string
	Group      string
	Set        string
	Title      string
	Body       string
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
	if err != nil {
		return err
	}
	return migrateAuth(app.db)
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
