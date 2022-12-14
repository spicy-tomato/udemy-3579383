package dbrepo

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"learn-golang/internal/models"
	"time"
)

func (*postgresDBRepo) AllUsers() bool {
	return true
}

// InsertReservation inserts a reservation into the database
func (rp *postgresDBRepo) InsertReservation(m models.Reservation) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var newId int
	stmt := `
        INSERT INTO reservations 
            (first_name, last_name, email, phone, start_date, end_date, room_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning id
    `

	err := rp.DB.QueryRowContext(
		ctx, stmt,
		m.FirstName, m.LastName, m.Email, m.Phone, m.StartDate, m.EndDate, m.RoomID,
		time.Now(), time.Now(),
	).Scan(&newId)

	if err != nil {
		return 0, err
	}

	return newId, nil
}

// InsertRoomRestriction inserts a new room restriction into the database
func (rp *postgresDBRepo) InsertRoomRestriction(m models.RoomRestriction) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `
        INSERT INTO room_restrictions 
            (start_date, end_date, room_id, reservation_id, created_at, updated_at, restriction_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `

	_, err := rp.DB.ExecContext(
		ctx, stmt,
		m.StartDate, m.EndDate, m.RoomID, m.ReservationID, time.Now(), time.Now(), m.RestrictionID,
	)

	if err != nil {
		return err
	}

	return nil
}

// SearchAvailabilityByDatesByRoomID returns true if availability exists for roomID, and false otherwise
func (rp *postgresDBRepo) SearchAvailabilityByDatesByRoomID(start, end time.Time, roomID int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var numRows int

	query := `
        SELECT COUNT(id) 
        FROM room_restrictions 
        WHERE 
            room_id = $1 AND
            $2 < end_date AND $3 > start_date
    `

	row := rp.DB.QueryRowContext(
		ctx, query,
		roomID, start, end,
	)
	err := row.Scan(&numRows)
	if err != nil {
		return false, err
	}

	return numRows == 0, nil
}

// SearchAvailabilityForAllRooms returns a slice of available rooms, if any, for given date range
func (rp *postgresDBRepo) SearchAvailabilityForAllRooms(start, end time.Time) (rooms []models.Room, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
        SELECT r.id, r.room_name
        FROM rooms r 
        WHERE 
            r.id NOT IN
                (SELECT room_id from room_restrictions rr WHERE $1 < rr.end_date AND $2 > rr.start_date)
    `

	rows, err := rp.DB.QueryContext(
		ctx, query,
		start, end,
	)
	if err != nil {
		return
	}

	for rows.Next() {
		var room models.Room
		err = rows.Scan(&room.ID, &room.RoomName)
		if err != nil {
			return
		}

		rooms = append(rooms, room)
	}

	if err = rows.Err(); err != nil {
		return
	}

	return
}

// GetRoomById gets a room by id
func (rp *postgresDBRepo) GetRoomById(id int) (models.Room, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var room models.Room
	query := `
        SELECT r.id, r.room_name, r.created_at, r.updated_at
        FROM rooms r
        WHERE r.id = $1
    `

	row := rp.DB.QueryRowContext(ctx, query, id)
	err := row.Scan(&room.ID, &room.RoomName, &room.CreatedAt, &room.UpdatedAt)

	if err != nil {
		return room, err
	}

	return room, nil
}

// GetUserById returns a user by ID
func (rp *postgresDBRepo) GetUserById(id int) (models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
        SELECT id, first_name, last_name, email, password, access_level, created_at, updated_at
        FROM users
        WHERE id = $1
    `

	row := rp.DB.QueryRowContext(ctx, query, id)

	var u models.User
	err := row.Scan(&u, &u.FirstName, &u.LastName, &u.Email, &u.Password, &u.AccessLevel, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return u, err
	}

	return u, nil
}

// UpdateUser updates a user in the database
func (rp *postgresDBRepo) UpdateUser(u models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
        UPDATE users SET first_name = $1, last_name = $2, email = $3, access_level = $4, updated_at = $5
    `

	_, err := rp.DB.ExecContext(
		ctx, query,
		u.FirstName, u.LastName, u.Email, u.AccessLevel, time.Now(),
	)
	if err != nil {
		return err
	}

	return nil
}

// Authenticate authenticates a user
func (rp *postgresDBRepo) Authenticate(email, testPassword string) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var id int
	var hashedPassword string

	query := "SELECT id, password FROM users WHERE email = $1"

	row := rp.DB.QueryRowContext(ctx, query, email)
	err := row.Scan(&id, &hashedPassword)
	if err != nil {
		return id, "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(testPassword))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return 0, "", errors.New("incorrect password")
	}
	if err != nil {
		return 0, "", err
	}

	return id, hashedPassword, nil
}
