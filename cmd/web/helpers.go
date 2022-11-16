package main

func (app *Config) SendEmail(msg Message) {
	app.wait.Add(1)
	app.Mailer.MailerChan <- msg
}
