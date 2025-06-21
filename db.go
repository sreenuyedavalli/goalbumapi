package main

import (
	"database/sql"
)

type Database interface {
	GetAlbumPrice(albumID string) (float64, error)
	SetAlbumPrice(albumID string, price float64) error
	GetAlbumCount() (int, error)
}

type PostgresDB struct {
	db *sql.DB
}

func (p *PostgresDB) GetAlbumPrice(albumID string) (float64, error) {
	var price float64
	err := p.db.QueryRow("SELECT price FROM album_prices WHERE album_id = $1 ORDER BY updated_at DESC LIMIT 1", albumID).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return price, err
}

func (p *PostgresDB) SetAlbumPrice(albumID string, price float64) error {
	_, err := p.db.Exec(`
		INSERT INTO album_prices (album_id, price, updated_at) 
		VALUES ($1, $2, NOW()) 
		ON CONFLICT (album_id) 
		DO UPDATE SET 
			price = EXCLUDED.price, 
			updated_at = NOW()
	`, albumID, price)
	return err
}

func (p *PostgresDB) GetAlbumCount() (int, error) {
	var count int
	err := p.db.QueryRow("SELECT COUNT(DISTINCT album_id) FROM album_prices").Scan(&count)
	return count, err
}
