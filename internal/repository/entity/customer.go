package entity

import "time"

type CustomerEntity struct {
	ID        int64     `db:"id"`
	Email     string    `db:"email"`
	Name      string    `db:"name"`
	Country   string    `db:"country"`
	CreatedAt time.Time `db:"created_at"`
}
