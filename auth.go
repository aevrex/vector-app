package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const authSchema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY,
	email         TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
	created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
`

const sessionTTL = 7 * 24 * time.Hour

type User struct {
	ID    int64
	Email string
	Role  string
}

func migrateAuth(db *sql.DB) error {
	if _, err := db.Exec(authSchema); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, time.Now().Unix())
	return err
}

// createUser makes the first account an admin so a fresh database is never
// left without one.
func createUser(db *sql.DB, email, passwordHash string) (*User, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return nil, err
	}
	role := "user"
	if count == 0 {
		role = "admin"
	}
	res, err := tx.Exec(`INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)`, email, passwordHash, role)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &User{ID: id, Email: email, Role: role}, nil
}

func userByEmail(db *sql.DB, email string) (*User, string) {
	u := &User{}
	var hash string
	err := db.QueryRow(`SELECT id, email, role, password_hash FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.Role, &hash)
	if err != nil {
		return nil, ""
	}
	return u, hash
}

func userByID(db *sql.DB, id int64) *User {
	u := &User{}
	err := db.QueryRow(`SELECT id, email, role FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Role)
	if err != nil {
		return nil
	}
	return u
}

func listUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(`SELECT id, email, role FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Role); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func setRole(db *sql.DB, userID int64, role string) error {
	_, err := db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, userID)
	return err
}

func createSession(db *sql.DB, userID int64) (string, error) {
	token := rand.Text()
	_, err := db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, time.Now().Add(sessionTTL).Unix())
	if err != nil {
		return "", err
	}
	return token, nil
}

func userBySession(db *sql.DB, token string) *User {
	u := &User{}
	err := db.QueryRow(`
		SELECT u.id, u.email, u.role
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > ?`, token, time.Now().Unix()).
		Scan(&u.ID, &u.Email, &u.Role)
	if err != nil {
		return nil
	}
	return u
}

func deleteSession(db *sql.DB, token string) {
	db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
}

// formError is swapped into the page's #form-error div by htmx. It returns
// 200 because htmx does not swap 4xx responses by default.
func (app *App) formError(w http.ResponseWriter, msg string) {
	fmt.Fprintf(w, `<p class="mt-4 rounded-xl border border-[#ffe5e5] bg-[#ffe5e5] p-3 text-sm font-semibold text-[#ad1d1a] shadow-sm">%s</p>`,
		template.HTMLEscapeString(msg))
}

func (app *App) currentUser(r *http.Request) *User {
	c, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	return userBySession(app.db, c.Value)
}

// requireUser returns the logged-in user, or sends the client to /login and
// returns nil. htmx requests get an HX-Redirect so the full page navigates
// instead of swapping the login page into a fragment.
func (app *App) requireUser(w http.ResponseWriter, r *http.Request) *User {
	u := app.currentUser(r)
	if u != nil {
		return u
	}
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
	} else {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
	return nil
}

// requireAdmin returns the logged-in admin, or responds (login redirect for
// anonymous visitors, 403 for signed-in non-admins) and returns nil.
func (app *App) requireAdmin(w http.ResponseWriter, r *http.Request) *User {
	u := app.requireUser(w, r)
	if u == nil {
		return nil
	}
	if u.Role != "admin" {
		http.Error(w, "Forbidden: admins only.", http.StatusForbidden)
		return nil
	}
	return u
}

func (app *App) startSession(w http.ResponseWriter, userID int64) error {
	token, err := createSession(app.db, userID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure: true, // enable when serving over HTTPS
	})
	return nil
}

// Handlers

func (app *App) handleSignupPage(w http.ResponseWriter, r *http.Request) {
	if app.currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	app.render(w, "signup.html", PageData{Title: "Sign up", Active: "signup"})
}

func (app *App) handleSignup(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	if !strings.Contains(email, "@") {
		app.formError(w, "Enter a valid email address.")
		return
	}
	if len(password) < 8 {
		app.formError(w, "Password must be at least 8 characters.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("signup:", err)
		app.formError(w, "Something went wrong. Try again.")
		return
	}
	user, err := createUser(app.db, email, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			app.formError(w, "That email is already registered.")
		} else {
			log.Println("signup:", err)
			app.formError(w, "Something went wrong. Try again.")
		}
		return
	}
	if err := app.startSession(w, user.ID); err != nil {
		log.Println("signup:", err)
		app.formError(w, "Something went wrong. Try again.")
		return
	}
	w.Header().Set("HX-Redirect", "/")
}

func (app *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if app.currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	app.render(w, "login.html", PageData{Title: "Log in", Active: "login"})
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	u, hash := userByEmail(app.db, email)
	if u == nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		app.formError(w, "Invalid email or password.")
		return
	}
	if err := app.startSession(w, u.ID); err != nil {
		log.Println("login:", err)
		app.formError(w, "Something went wrong. Try again.")
		return
	}
	w.Header().Set("HX-Redirect", "/")
}

func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		deleteSession(app.db, c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("HX-Redirect", "/login")
}

type UserRow struct {
	U    User
	Self bool
}

type AdminPageData struct {
	Title  string
	Active string
	User   *User
	Rows   []UserRow
}

func (app *App) handleAdmin(w http.ResponseWriter, r *http.Request) {
	u := app.requireAdmin(w, r)
	if u == nil {
		return
	}
	users, err := listUsers(app.db)
	if err != nil {
		log.Println("admin:", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	rows := make([]UserRow, len(users))
	for i, x := range users {
		rows[i] = UserRow{U: x, Self: x.ID == u.ID}
	}
	app.render(w, "admin.html", AdminPageData{Title: "Admin", Active: "admin", User: u, Rows: rows})
}

func (app *App) handleSetRole(w http.ResponseWriter, r *http.Request) {
	u := app.requireAdmin(w, r)
	if u == nil {
		return
	}
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	role := r.FormValue("role")
	if err != nil || (role != "user" && role != "admin") {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if id == u.ID {
		http.Error(w, "You cannot change your own role.", http.StatusBadRequest)
		return
	}
	target := userByID(app.db, id)
	if target == nil {
		http.Error(w, "no such user", http.StatusNotFound)
		return
	}
	if err := setRole(app.db, id, role); err != nil {
		log.Println("set role:", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	target.Role = role
	views, err := template.ParseFiles("templates/admin.html")
	if err != nil {
		log.Println("render user_row:", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := views.ExecuteTemplate(w, "user_row", UserRow{U: *target}); err != nil {
		log.Println("render user_row:", err)
	}
}
