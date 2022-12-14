package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"learn-golang/internal/config"
	"learn-golang/internal/driver"
	"learn-golang/internal/forms"
	"learn-golang/internal/helpers"
	"learn-golang/internal/models"
	"learn-golang/internal/render"
	"learn-golang/internal/repository"
	"learn-golang/internal/repository/dbrepo"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Repo the repository used by handlers
var Repo *Repository

// Repository is the repository type
type Repository struct {
	App *config.AppConfig
	DB  repository.DatabaseRepo
}

// NewRepo creates a new repository
func NewRepo(a *config.AppConfig, db *driver.DB) *Repository {
	return &Repository{
		App: a,
		DB:  dbrepo.NewPostgresDBRepo(db.SQL, a),
	}
}

// NewTestRepo creates a new repository
func NewTestRepo(a *config.AppConfig) *Repository {
	return &Repository{
		App: a,
		DB:  dbrepo.NewTestingsDBRepo(a),
	}
}

// NewHandlers sets the repository for the handlers
func NewHandlers(r *Repository) {
	Repo = r
}

// Home is the home page handler
func (rp *Repository) Home(w http.ResponseWriter, r *http.Request) {
	_ = render.Template(w, r, "home.page.tmpl", &models.TemplateData{})
}

// About is the about page handler
func (rp *Repository) About(w http.ResponseWriter, r *http.Request) {
	_ = render.Template(w, r, "about.page.tmpl", &models.TemplateData{})
}

// Reservation renders the make a reservation page and display form
func (rp *Repository) Reservation(w http.ResponseWriter, r *http.Request) {
	res, ok := rp.App.Session.Get(r.Context(), "reservation").(models.Reservation)
	if !ok {
		rp.App.Session.Put(r.Context(), "error", "can't get reservation from session")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	room, err := rp.DB.GetRoomById(res.RoomID)
	if err != nil {
		rp.App.Session.Put(r.Context(), "error", "can't find room")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	res.Room = room

	rp.App.Session.Put(r.Context(), "reservation", res)

	sd := res.StartDate.Format("2006-01-02")
	ed := res.EndDate.Format("2006-01-02")

	stringMap := make(map[string]string)
	stringMap["start_date"] = sd
	stringMap["end_date"] = ed

	data := make(map[string]any)
	data["reservation"] = res

	_ = render.Template(
		w, r, "make-reservation.page.tmpl", &models.TemplateData{
			Form:      forms.New(nil),
			Data:      data,
			StringMap: stringMap,
		},
	)
}

// PostReservation handles the posting of a reservation form
func (rp *Repository) PostReservation(w http.ResponseWriter, r *http.Request) {
	reservation, ok := rp.App.Session.Get(r.Context(), "reservation").(models.Reservation)
	if !ok {
		helpers.ServerError(w, errors.New("cannot get from session"))
		return
	}

	err := r.ParseForm()
	if err != nil {
		helpers.ServerError(w, err)
		return
	}

	reservation.FirstName = r.Form.Get("first_name")
	reservation.LastName = r.Form.Get("last_name")
	reservation.Phone = r.Form.Get("phone")
	reservation.Email = r.Form.Get("email")

	form := forms.New(r.PostForm)

	form.Required("first_name", "last_name", "email")
	form.MinLength("first_name", 3)
	form.IsEmail("email")

	if !form.Valid() {
		data := make(map[string]any)
		data["reservation"] = reservation

		_ = render.Template(
			w, r, "make-reservation.page.tmpl", &models.TemplateData{
				Form: form,
				Data: data,
			},
		)
		return
	}

	newReservationID, err := rp.DB.InsertReservation(reservation)
	if err != nil {
		helpers.ServerError(w, err)
	}

	restriction := models.RoomRestriction{
		StartDate:     reservation.StartDate,
		EndDate:       reservation.EndDate,
		RoomID:        reservation.RoomID,
		ReservationID: newReservationID,
		RestrictionID: 1,
	}

	err = rp.DB.InsertRoomRestriction(restriction)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}

	// send notification
	htmlMessage := fmt.Sprintf(
		`
	<strong>Reservation Confirmation</strong><br>
    Dear %s, <br>
    This is confirm your reservation from %s to %s.
`, reservation.FirstName, reservation.StartDate.Format("2006-01-02"), reservation.EndDate.Format("2006-01-02"),
	)

	msg := models.MailData{
		To:      reservation.Email,
		From:    "me@here.com",
		Subject: "Reservation Confirmation",
		Content: htmlMessage,
	}

	rp.App.MailChan <- msg

	// send notification to proper owner
	htmlMessage = fmt.Sprintf(
		`
	<strong>Reservation Notification</strong><br>
    A reservation has been made for %s from %s to %s.
`, reservation.Room.RoomName, reservation.StartDate.Format("2006-01-02"), reservation.EndDate.Format("2006-01-02"),
	)

	msg = models.MailData{
		To:       "me@here.com",
		From:     "me@here.com",
		Subject:  "Reservation Notification",
		Content:  htmlMessage,
		Template: "basic.html",
	}

	rp.App.MailChan <- msg

	rp.App.Session.Put(r.Context(), "reservation", reservation)

	http.Redirect(w, r, "/reservation-summary", http.StatusSeeOther)
}

// Generals renders the room page
func (rp *Repository) Generals(w http.ResponseWriter, r *http.Request) {
	_ = render.Template(w, r, "generals.page.tmpl", &models.TemplateData{})
}

// Majors renders the room page
func (rp *Repository) Majors(w http.ResponseWriter, r *http.Request) {
	_ = render.Template(w, r, "majors.page.tmpl", &models.TemplateData{})
}

// Availability renders the search availability page
func (rp *Repository) Availability(w http.ResponseWriter, r *http.Request) {
	_ = render.Template(w, r, "search-availability.page.tmpl", &models.TemplateData{})
}

// PostAvailability renders the search availability page
func (rp *Repository) PostAvailability(w http.ResponseWriter, r *http.Request) {
	start := r.Form.Get("start")
	end := r.Form.Get("end")

	layout := "2006-01-02"
	startDate, err := time.Parse(layout, start)
	if err != nil {
		helpers.ServerError(w, err)
	}
	endDate, err := time.Parse(layout, end)
	if err != nil {
		helpers.ServerError(w, err)
	}

	rooms, err := rp.DB.SearchAvailabilityForAllRooms(startDate, endDate)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}

	if len(rooms) == 0 {
		// no availability
		rp.App.Session.Put(r.Context(), "error", "No availability")
		http.Redirect(w, r, "/search-availability", http.StatusSeeOther)
		return
	}

	data := make(map[string]any)
	data["rooms"] = rooms

	res := models.Reservation{
		StartDate: startDate,
		EndDate:   endDate,
	}

	rp.App.Session.Put(r.Context(), "reservation", res)

	_ = render.Template(
		w, r, "choose-room.page.tmpl", &models.TemplateData{
			Data: data,
		},
	)
}

type jsonResponse struct {
	Ok        bool   `json:"ok"`
	Message   string `json:"message"`
	RoomID    string `json:"room_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// AvailabilityJSON handles request for availability and send JSON response
func (rp *Repository) AvailabilityJSON(w http.ResponseWriter, r *http.Request) {
	sd := r.Form.Get("start")
	ed := r.Form.Get("end")

	layout := "2006-01-02"
	startDate, _ := time.Parse(layout, sd)
	endDate, _ := time.Parse(layout, ed)

	roomID, _ := strconv.Atoi(r.Form.Get("room_id"))

	available, _ := rp.DB.SearchAvailabilityByDatesByRoomID(startDate, endDate, roomID)
	resp := jsonResponse{
		Ok:        available,
		Message:   "",
		StartDate: sd,
		EndDate:   ed,
		RoomID:    strconv.Itoa(roomID),
	}

	out, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		helpers.ServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

// Contact renders the search availability page
func (rp *Repository) Contact(w http.ResponseWriter, r *http.Request) {
	_ = render.Template(w, r, "contact.page.tmpl", &models.TemplateData{})
}

// ReservationSummary displays the reservation summary page
func (rp *Repository) ReservationSummary(w http.ResponseWriter, r *http.Request) {
	reservation, ok := rp.App.Session.Get(r.Context(), "reservation").(models.Reservation)
	if !ok {
		rp.App.ErrorLog.Println("Can't get error from session")
		rp.App.Session.Put(r.Context(), "error", "Can't get reservation from session")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	rp.App.Session.Remove(r.Context(), "reservation")

	data := make(map[string]any)
	data["reservation"] = reservation

	sd := reservation.StartDate.Format("2006-01-02")
	ed := reservation.StartDate.Format("2006-01-02")
	stringMap := make(map[string]string)
	stringMap["start_date"] = sd
	stringMap["end_date"] = ed

	_ = render.Template(
		w, r, "reservation-summary.page.tmpl", &models.TemplateData{
			Data:      data,
			StringMap: stringMap,
		},
	)
}

// ChooseRoom displays list of available rooms
func (rp *Repository) ChooseRoom(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.Atoi(chi.URLParam(r, "id"))

	if err != nil {
		helpers.ServerError(w, err)
		return
	}

	rp.App.Session.Get(r.Context(), "reservation")

	res, ok := rp.App.Session.Get(r.Context(), "reservation").(models.Reservation)
	if !ok {
		helpers.ServerError(w, err)
		return
	}

	res.RoomID = roomID

	rp.App.Session.Put(r.Context(), "reservation", res)

	http.Redirect(w, r, "/make-reservation", http.StatusSeeOther)
}

// BookRoom takes URL parameters, builds a session variable, and takes user to make res screen
func (rp *Repository) BookRoom(w http.ResponseWriter, r *http.Request) {
	roomID, _ := strconv.Atoi(r.URL.Query().Get("id"))
	sd := r.URL.Query().Get("s")
	ed := r.URL.Query().Get("e")

	layout := "2006-01-02"
	startDate, _ := time.Parse(layout, sd)
	endDate, _ := time.Parse(layout, ed)

	var res models.Reservation

	room, err := rp.DB.GetRoomById(roomID)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}

	res.Room = room
	res.RoomID = roomID
	res.StartDate = startDate
	res.EndDate = endDate

	rp.App.Session.Put(r.Context(), "reservation", res)

	http.Redirect(w, r, "/make-reservation", http.StatusSeeOther)
}

func (rp *Repository) ShowLogin(w http.ResponseWriter, r *http.Request) {
	_ = render.Template(
		w, r, "login.page.tmpl", &models.TemplateData{
			Form: forms.New(nil),
		},
	)
}

// PostShowLogin handles logging the user in
func (rp *Repository) PostShowLogin(w http.ResponseWriter, r *http.Request) {
	_ = rp.App.Session.RenewToken(r.Context())

	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}

	form := forms.New(r.PostForm)
	form.Required("email", "password")
	form.IsEmail("email")

	if !form.Valid() {
		_ = render.Template(
			w, r, "login.page.tmpl", &models.TemplateData{
				Form: form,
			},
		)
		return
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")

	id, _, err := rp.DB.Authenticate(email, password)
	if err != nil {
		log.Println(err)
		rp.App.Session.Put(r.Context(), "error", "Invalid login credentials")
		http.Redirect(w, r, "/user/login", http.StatusSeeOther)
		return
	}

	rp.App.Session.Put(r.Context(), "user_id", id)
	rp.App.Session.Put(r.Context(), "flash", "Logged in successfully")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (rp *Repository) Logout(w http.ResponseWriter, r *http.Request) {
	_ = rp.App.Session.Destroy(r.Context())
	_ = rp.App.Session.RenewToken(r.Context())

	http.Redirect(w, r, "/user/login", http.StatusSeeOther)
}

func (rp *Repository) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	_ = render.Template(w, r, "admin-dashboard.page.tmpl", &models.TemplateData{})
}
