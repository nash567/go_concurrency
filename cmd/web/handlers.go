package main

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/messageService/data"
	"github.com/phpdave11/gofpdf"
	"github.com/phpdave11/gofpdf/contrib/gofpdi"
)

func (app *Config) HomePage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "home.page.gohtml", nil)
}

func (app *Config) LoginPage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Welcome to")
	app.render(w, r, "login.page.gohtml", nil)
}

func (app *Config) PostLoginPage(w http.ResponseWriter, r *http.Request) {
	_ = app.Session.RenewToken(r.Context())
	err := r.ParseForm()
	if err != nil {
		app.ErrLog.Println(err)
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")

	user, err := app.Models.User.GetByEmail(email)

	if err != nil {
		app.Session.Put(r.Context(), "error", "Invalid user")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	validPassword, err := user.PasswordMatches(password)

	if err != nil {
		app.Session.Put(r.Context(), "error", "Invalid user")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if !validPassword {

		msg := Message{
			To:      email,
			Subject: "Failed log in attempt",
			Data:    "Invalid login attempt",
		}
		app.SendEmail(msg)
		app.Session.Put(r.Context(), "error", "Invalid user")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.Session.Put(r.Context(), "userID", user.ID)
	app.Session.Put(r.Context(), "user", user)
	app.Session.Put(r.Context(), "flash", "succesfull log in")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Config) Logout(w http.ResponseWriter, r *http.Request) {
	_ = app.Session.Destroy(r.Context())
	_ = app.Session.RenewToken(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)

}

func (app *Config) RegisterPage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "register.page.gohtml", nil)
}

func (app *Config) PostRegisterPage(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.ErrLog.Println(err)
	}
	u := data.User{
		Email:     r.Form.Get("email"),
		FirstName: r.Form.Get("first-name"),
		LastName:  r.Form.Get("last-name"),
		Password:  r.Form.Get("password"),
		Active:    0,
		IsAdmin:   0,
	}
	_, err = u.Insert(u)

	if err != nil {
		app.ErrLog.Println(err)
		app.Session.Put(r.Context(), "error", "Unable to create User")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
	}
	url := fmt.Sprintf("http://localhost/activate?email=%s", u.Email)
	signedURL := GenerateTokenFromString(url)
	app.InfoLog.Println(signedURL)
	msg := Message{
		To:       u.Email,
		Subject:  "Activate your ccount",
		Template: "confirmation-email",
		Data:     template.HTML(signedURL),
	}
	app.SendEmail(msg)
	app.Session.Put(r.Context(), "flash", "COnfirmation email sent.Check you email")
	http.Redirect(w, r, "/login", http.StatusSeeOther)

}

func (app *Config) ActivateAccount(w http.ResponseWriter, r *http.Request) {
	url := r.RequestURI
	testURL := fmt.Sprintf("http://localhost%s", url)
	okay := VerifyToken(testURL)
	if !okay {
		app.Session.Put(r.Context(), "error", "Invalid token")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	u, err := app.Models.User.GetByEmail(r.URL.Query().Get("email"))
	if err != nil {
		app.Session.Put(r.Context(), "error", "no user found")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	u.Active = 1
	err = u.Update()
	if err != nil {
		app.Session.Put(r.Context(), "error", "Unable to update User")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	app.Session.Put(r.Context(), "flash", "Account activated. You can now log in")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *Config) ChooseSubscription(w http.ResponseWriter, r *http.Request) {

	plans, err := app.Models.Plan.GetAll()
	if err != nil {
		app.ErrLog.Println(err)
	}

	dataMap := make(map[string]any)
	dataMap["plans"] = plans
	app.render(w, r, "plans.page.gohtml", &TemplateData{
		Data: dataMap,
	})
}

func (app *Config) SubscribeToPlan(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	planId, err := strconv.Atoi(id)
	if err != nil {
		app.ErrLog.Println("Error getting plan:", err)
	}
	plan, err := app.Models.Plan.GetOne(planId)
	if err != nil {
		app.Session.Put(r.Context(), "error", "unable to find Plan")
		http.Redirect(w, r, "members/plan", http.StatusSeeOther)
		return
	}
	user, ok := app.Session.Get(r.Context(), "user").(data.User)
	if !ok {
		app.Session.Put(r.Context(), "error", "Log in first")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.wait.Add(1)
	go func() {
		defer app.wait.Done()
		invoice, err := app.getInvoice(user, plan)

		if err != nil {
			app.ErrorChan <- err
		}
		msg := Message{
			To:       user.Email,
			Subject:  "your invoice",
			Data:     invoice,
			Template: "invoice",
		}

		app.SendEmail(msg)
	}()
	app.wait.Add(1)
	go func() {
		defer app.wait.Done()
		pdf := app.generateManual(user, plan)
		err := pdf.OutputFileAndClose(fmt.Sprintf("./tmp/%d_manual.pdf", user.ID))
		if err != nil {
			app.ErrorChan <- err
			return
		}
		msg := Message{
			To:      user.Email,
			Subject: "your manual",
			AttachmentsMap: map[string]string{
				"Manual.pdf": fmt.Sprintf("./tmp/%d_manual.pdf", user.ID),
			},
			Template: "invoice",
		}

		app.SendEmail(msg)

		app.ErrorChan <- errors.New("new custom error")
	}()
	err = app.Models.Plan.SubscribeUserToPlan(user, *plan)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Error Subscribing to plan")
		http.Redirect(w, r, "/members/plan", http.StatusSeeOther)
		return
	}
	u, err := app.Models.User.GetOne(user.ID)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Error getting user from database")
		http.Redirect(w, r, "/members/plan", http.StatusSeeOther)
		return
	}
	app.Session.Put(r.Context(), "user", u)

	app.Session.Put(r.Context(), "flash", "Subscribed!")
	http.Redirect(w, r, "/membs/plan", http.StatusSeeOther)

}

func (app *Config) generateManual(u data.User, plan *data.Plan) *gofpdf.Fpdf {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.SetMargins(10, 13, 10)
	importer := gofpdi.NewImporter()

	time.Sleep(5 * time.Second)
	t := importer.ImportPage(pdf, "../../pdf/manual", 1, "/MediaBox")
	pdf.AddPage()

	importer.UseImportedTemplate(pdf, t, 0, 0, 215.9, 0)
	pdf.SetX(75)
	pdf.SetY(120)
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 4, fmt.Sprintf("%s %s", u.FirstName, u.LastName), "", "C", false)
	pdf.Ln(5)

	pdf.MultiCell(0, 4, fmt.Sprintf("%s User Guide", plan.PlanName), "", "C", false)
	return pdf

}
func (app *Config) getInvoice(u data.User, plan *data.Plan) (string, error) {
	return plan.PlanAmountFormatted, nil
}
