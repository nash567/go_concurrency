package main

import (
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/messageService/data"
)

var pathTOTemplate = "./cmd/web/templates"

type TemplateData struct {
	StringMap     map[string]string
	IntMap        map[string]int
	FloatMap      map[string]float64
	Data          map[string]any
	Flash         string
	Warning       string
	Error         string
	Authenticated bool
	Now           time.Time
	User          *data.User
}

func (app *Config) render(w http.ResponseWriter, r *http.Request, t string, td *TemplateData) {
	partials := []string{
		fmt.Sprintf("%s/base.layout.gohtml", pathTOTemplate),
		fmt.Sprintf("%s/header.partial.gohtml", pathTOTemplate),
		fmt.Sprintf("%s/navbar.partial.gohtml", pathTOTemplate),
		fmt.Sprintf("%s/footer.partial.gohtml", pathTOTemplate),
		fmt.Sprintf("%s/alerts.partial.gohtml", pathTOTemplate),
	}

	var templateSlice []string
	templateSlice = append(templateSlice, fmt.Sprintf("%s/%s", pathTOTemplate, t))
	templateSlice = append(templateSlice, partials...)
	if td == nil {
		td = &TemplateData{}
	}

	tmpl, err := template.ParseFiles(templateSlice...)
	if err != nil {
		app.ErrLog.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, app.addDefaultData(td, r)); err != nil {
		app.ErrLog.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *Config) addDefaultData(td *TemplateData, r *http.Request) *TemplateData {
	td.Flash = app.Session.PopString(r.Context(), "flash")
	td.Warning = app.Session.PopString(r.Context(), "warning")
	td.Error = app.Session.PopString(r.Context(), "error")
	if app.IsAuthenticated(r) {
		td.Authenticated = true
		user, ok := app.Session.Get(r.Context(), "user").(data.User)
		if !ok {
			app.ErrLog.Println("cant get user from session")
		} else {
			td.User = &user
		}
	}
	td.Now = time.Now()

	return td
}

func (app *Config) IsAuthenticated(r *http.Request) bool {
	return app.Session.Exists(r.Context(), "userID")

}
