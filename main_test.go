package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// MockDatabase implements the Database interface for testing
type MockDatabase struct {
	albums []album
}

func (m *MockDatabase) GetAlbumPrice(albumID string) (float64, error) {
	for _, a := range m.albums {
		if a.ID == albumID {
			return a.Price, nil
		}
	}
	return 0, nil
}

func (m *MockDatabase) SetAlbumPrice(albumID string, price float64) error {
	for i, a := range m.albums {
		if a.ID == albumID {
			m.albums[i].Price = price
			return nil
		}
	}
	// If album not found, add it
	m.albums = append(m.albums, album{
		ID:     albumID,
		Title:  "Test Album",
		Artist: "Test Artist",
		Year:   "2023",
		Price:  price,
	})
	return nil
}

func (m *MockDatabase) GetAlbumCount() (int, error) {
	return len(m.albums), nil
}

func (m *MockDatabase) AddAlbumOfMonthSignup(name, email string) error {
	return nil
}

// MockDiscogsService implements the DiscogsService interface for testing
type MockDiscogsService struct {
	albums []album
}

func (m *MockDiscogsService) FetchCollection(db Database, page, perPage int) ([]album, int, error) {
	return m.albums, len(m.albums), nil
}

func TestGetAlbums(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB := &MockDatabase{
		albums: []album{
			{ID: "1", Title: "Test Album", Artist: "Test Artist", Year: "2023", Price: 25.99},
		},
	}

	mockDiscogs := &MockDiscogsService{
		albums: []album{
			{ID: "1", Title: "Test Album", Artist: "Test Artist", Year: "2023", Price: 25.99},
		},
	}

	router := gin.New()
	router.GET("/albums", getAlbums(mockDB, mockDiscogs))

	req, _ := http.NewRequest("GET", "/albums", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "albums")
}

func TestUpdateAlbumPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB := &MockDatabase{
		albums: []album{
			{ID: "1", Title: "Test Album", Artist: "Test Artist", Year: "2023", Price: 25.99},
		},
	}

	router := gin.New()
	router.PUT("/albums/:id/price", updateAlbumPrice(mockDB))

	priceUpdate := map[string]float64{"price": 30.99}
	jsonData, _ := json.Marshal(priceUpdate)

	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the price was updated
	updatedPrice, err := mockDB.GetAlbumPrice("1")
	assert.NoError(t, err)
	assert.Equal(t, 30.99, updatedPrice)
}

func TestGetAlbumPrice(t *testing.T) {
	mockDB := &MockDatabase{
		albums: []album{
			{ID: "1", Title: "Test Album", Artist: "Test Artist", Year: "2023", Price: 25.99},
		},
	}

	price, err := getAlbumPrice(mockDB, "1")
	assert.NoError(t, err)
	assert.Equal(t, 25.99, price)

	// Test non-existent album
	price, err = getAlbumPrice(mockDB, "999")
	assert.NoError(t, err)
	assert.Equal(t, 0.0, price)
}

func TestMockDatabaseMethods(t *testing.T) {
	mockDB := &MockDatabase{}

	// Test GetAlbumCount
	count, err := mockDB.GetAlbumCount()
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Test SetAlbumPrice
	err = mockDB.SetAlbumPrice("1", 25.99)
	assert.NoError(t, err)

	// Test GetAlbumPrice after setting
	price, err := mockDB.GetAlbumPrice("1")
	assert.NoError(t, err)
	assert.Equal(t, 25.99, price)

	// Test GetAlbumCount after adding
	count, err = mockDB.GetAlbumCount()
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Test AddAlbumOfMonthSignup
	err = mockDB.AddAlbumOfMonthSignup("Test User", "test@example.com")
	assert.NoError(t, err)
}

func TestUpdateAlbumPriceInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB := &MockDatabase{}
	router := gin.New()
	router.PUT("/albums/:id/price", updateAlbumPrice(mockDB))

	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAlbumsWithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB := &MockDatabase{}
	mockDiscogs := &MockDiscogsService{
		albums: []album{
			{ID: "1", Title: "Album 1", Artist: "Artist 1", Year: "2023", Price: 25.99},
			{ID: "2", Title: "Album 2", Artist: "Artist 2", Year: "2023", Price: 30.99},
		},
	}

	router := gin.New()
	router.GET("/albums", getAlbums(mockDB, mockDiscogs))

	req, _ := http.NewRequest("GET", "/albums?page=1&per_page=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "albums")
}
