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
// Only the methods needed for the tests are implemented
// You can expand this as needed for more coverage

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
	// Find existing album and update price
	for i, a := range m.albums {
		if a.ID == albumID {
			m.albums[i].Price = price
			return nil
		}
	}
	// If album not found, add it (simulating the ON CONFLICT behavior)
	m.albums = append(m.albums, album{
		ID:     albumID,
		Title:  "Unknown Album",
		Artist: "Unknown Artist",
		Year:   "Unknown",
		Price:  price,
	})
	return nil
}

func (m *MockDatabase) GetAlbumCount() (int, error) {
	return len(m.albums), nil
}

// Add a mock DiscogsService for testing

type MockDiscogsService struct{}

func (m *MockDiscogsService) FetchCollection(db Database, page, perPage int) ([]album, int, error) {
	if mockDB, ok := db.(*MockDatabase); ok {
		return mockDB.albums, len(mockDB.albums), nil
	}
	return nil, 0, nil
}

// Update setupTestRouter to use the mock DiscogsService
func setupTestRouter() (*gin.Engine, *MockDatabase) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mockDB := &MockDatabase{
		albums: []album{
			{ID: "1", Title: "Test Album 1", Artist: "Artist 1", Year: "2020", Price: 29.99},
			{ID: "2", Title: "Test Album 2", Artist: "Artist 2", Year: "2021", Price: 39.99},
		},
	}
	mockDiscogs := &MockDiscogsService{}

	router.GET("/albums", getAlbums(mockDB, mockDiscogs))
	router.GET("/albums/:id/price", func(c *gin.Context) {
		id := c.Param("id")
		price, _ := mockDB.GetAlbumPrice(id)
		c.JSON(http.StatusOK, gin.H{"price": price})
	})
	router.PUT("/albums/:id/price", updateAlbumPrice(mockDB))

	return router, mockDB
}

func TestGetAlbums(t *testing.T) {
	router, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/albums", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestGetAlbumPrice(t *testing.T) {
	router, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/albums/1/price", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var response map[string]float64
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, 29.99, response["price"])
}

func TestGetAlbumPriceNotFound(t *testing.T) {
	router, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/albums/999/price", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var response map[string]float64
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, 0.0, response["price"])
}

func TestUpdateAlbumPrice(t *testing.T) {
	router, mockDB := setupTestRouter()

	priceUpdate := map[string]float64{"price": 99.99}
	jsonData, _ := json.Marshal(priceUpdate)
	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// Check that the price was updated in the mock DB
	price, _ := mockDB.GetAlbumPrice("1")
	assert.Equal(t, 99.99, price)
}

func TestUpdateAlbumPriceMultipleTimes(t *testing.T) {
	router, mockDB := setupTestRouter()

	// First update
	priceUpdate1 := map[string]float64{"price": 50.00}
	jsonData1, _ := json.Marshal(priceUpdate1)
	req1, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	assert.Equal(t, 200, w1.Code)
	price1, _ := mockDB.GetAlbumPrice("1")
	assert.Equal(t, 50.00, price1)

	// Second update
	priceUpdate2 := map[string]float64{"price": 75.50}
	jsonData2, _ := json.Marshal(priceUpdate2)
	req2, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, 200, w2.Code)
	price2, _ := mockDB.GetAlbumPrice("1")
	assert.Equal(t, 75.50, price2)
}

func TestUpdateAlbumPriceToZero(t *testing.T) {
	router, mockDB := setupTestRouter()

	priceUpdate := map[string]float64{"price": 0.0}
	jsonData, _ := json.Marshal(priceUpdate)
	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	price, _ := mockDB.GetAlbumPrice("1")
	assert.Equal(t, 0.0, price)
}

func TestUpdateAlbumPriceNewAlbum(t *testing.T) {
	router, mockDB := setupTestRouter()

	priceUpdate := map[string]float64{"price": 25.99}
	jsonData, _ := json.Marshal(priceUpdate)
	req, _ := http.NewRequest("PUT", "/albums/999/price", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// Check that the new album was added with the price
	price, _ := mockDB.GetAlbumPrice("999")
	assert.Equal(t, 25.99, price)

	// Verify the album was added to the collection
	albums, _ := mockDB.GetAlbumCount()
	assert.Equal(t, 3, albums) // Original 2 + new 1
}

func TestUpdateAlbumPriceInvalidJSON(t *testing.T) {
	router, _ := setupTestRouter()

	// Send invalid JSON
	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestUpdateAlbumPriceMissingPrice(t *testing.T) {
	router, mockDB := setupTestRouter()

	// Send JSON without price field
	priceUpdate := map[string]string{"other_field": "value"}
	jsonData, _ := json.Marshal(priceUpdate)
	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Current implementation allows missing price field and defaults to 0
	assert.Equal(t, 200, w.Code)
	price, _ := mockDB.GetAlbumPrice("1")
	assert.Equal(t, 0.0, price)
}

func TestUpdateAlbumPriceNegativeValue(t *testing.T) {
	router, mockDB := setupTestRouter()

	priceUpdate := map[string]float64{"price": -10.0}
	jsonData, _ := json.Marshal(priceUpdate)
	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The current implementation allows negative prices, but we can test it works
	assert.Equal(t, 200, w.Code)
	price, _ := mockDB.GetAlbumPrice("1")
	assert.Equal(t, -10.0, price)
}

func TestUpdateAlbumPriceLargeValue(t *testing.T) {
	router, mockDB := setupTestRouter()

	priceUpdate := map[string]float64{"price": 999999.99}
	jsonData, _ := json.Marshal(priceUpdate)
	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	price, _ := mockDB.GetAlbumPrice("1")
	assert.Equal(t, 999999.99, price)
}

func TestUpdateAlbumPriceDecimalPrecision(t *testing.T) {
	router, mockDB := setupTestRouter()

	priceUpdate := map[string]float64{"price": 29.999}
	jsonData, _ := json.Marshal(priceUpdate)
	req, _ := http.NewRequest("PUT", "/albums/1/price", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	price, _ := mockDB.GetAlbumPrice("1")
	assert.Equal(t, 29.999, price)
}
