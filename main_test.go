package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// SetupRouter initializes the router for testing
func SetupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Setup route group for the API
	api := router.Group("/api")
	{
		api.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})
	}

	router.GET("/albums", getAlbums)
	router.GET("/albums/:id", getAlbumByID)
	router.POST("/albums", postAlbums)

	return router
}

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) {
	// Set test database environment variables
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASSWORD", "postgres")
	os.Setenv("DB_NAME", "albumdb_test")

	// Initialize database
	err := initDB()
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
}

// setupTestRouter creates a test router with all routes
func setupTestRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/albums", getAlbums)
	router.PUT("/api/albums/:id/price", updateAlbumPrice)
	router.POST("/api/albums", createAlbum)
	return router
}

// TestInitDB tests database initialization
func TestInitDB(t *testing.T) {
	// Test with valid configuration
	err := initDB()
	if err != nil {
		t.Errorf("initDB failed with valid configuration: %v", err)
	}

	// Test with invalid configuration
	os.Setenv("DB_HOST", "invalid_host")
	err = initDB()
	if err == nil {
		t.Error("initDB should fail with invalid configuration")
	}
}

// TestGetAlbums tests the getAlbums endpoint
func TestGetAlbums(t *testing.T) {
	setupTestDB(t)
	router := setupTestRouter()

	tests := []struct {
		name          string
		query         string
		expectedCode  int
		expectedItems int
	}{
		{
			name:          "Default pagination",
			query:         "",
			expectedCode:  http.StatusOK,
			expectedItems: 12,
		},
		{
			name:          "Custom page size",
			query:         "?per_page=5",
			expectedCode:  http.StatusOK,
			expectedItems: 5,
		},
		{
			name:          "Invalid page",
			query:         "?page=0",
			expectedCode:  http.StatusOK,
			expectedItems: 12,
		},
		{
			name:          "Sort by title",
			query:         "?sort_by=title&direction=asc",
			expectedCode:  http.StatusOK,
			expectedItems: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/albums"+tt.query, nil)
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			albums := response["albums"].([]interface{})
			if len(albums) != tt.expectedItems {
				t.Errorf("Expected %d items, got %d", tt.expectedItems, len(albums))
			}
		})
	}
}

// TestUpdateAlbumPrice tests the updateAlbumPrice endpoint
func TestUpdateAlbumPrice(t *testing.T) {
	setupTestDB(t)
	router := setupTestRouter()

	tests := []struct {
		name         string
		albumID      string
		price        float64
		expectedCode int
	}{
		{
			name:         "Valid price update",
			albumID:      "123",
			price:        29.99,
			expectedCode: http.StatusOK,
		},
		{
			name:         "Invalid price",
			albumID:      "123",
			price:        -10.0,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Missing price",
			albumID:      "123",
			price:        0,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priceUpdate := PriceUpdate{Price: tt.price}
			jsonData, _ := json.Marshal(priceUpdate)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/albums/"+tt.albumID+"/price", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}
		})
	}
}

// TestCreateAlbum tests the createAlbum endpoint
func TestCreateAlbum(t *testing.T) {
	setupTestDB(t)
	router := setupTestRouter()

	tests := []struct {
		name         string
		album        CreateAlbumRequest
		expectedCode int
	}{
		{
			name: "Valid album creation",
			album: CreateAlbumRequest{
				Title:  "Test Album",
				Artist: "Test Artist",
				Year:   2024,
				Label:  "Test Label",
				Genres: []string{"Rock"},
				Styles: []string{"Progressive"},
				Tracklist: []struct {
					Position string `json:"position"`
					Title    string `json:"title"`
					Duration string `json:"duration"`
				}{
					{Position: "A1", Title: "Track 1", Duration: "3:45"},
				},
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "Missing required fields",
			album: CreateAlbumRequest{
				Title: "Test Album",
				// Missing required fields
			},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.album)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/albums", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, w.Code)
			}
		})
	}
}

// TestGetAlbumPrice tests the getAlbumPrice function
func TestGetAlbumPrice(t *testing.T) {
	setupTestDB(t)

	tests := []struct {
		name     string
		albumID  string
		expected float64
		hasError bool
	}{
		{
			name:     "Existing album",
			albumID:  "123",
			expected: 29.99,
			hasError: false,
		},
		{
			name:     "Non-existent album",
			albumID:  "999",
			expected: 0,
			hasError: false,
		},
	}

	// First, set up test data
	_, err := db.Exec(`
		INSERT INTO album_prices (album_id, price, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (album_id) DO UPDATE
		SET price = $2, updated_at = CURRENT_TIMESTAMP
	`, "123", 29.99)
	if err != nil {
		t.Fatalf("Failed to set up test data: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := getAlbumPrice(tt.albumID)
			if (err != nil) != tt.hasError {
				t.Errorf("getAlbumPrice() error = %v, hasError %v", err, tt.hasError)
				return
			}
			if price != tt.expected {
				t.Errorf("getAlbumPrice() = %v, want %v", price, tt.expected)
			}
		})
	}
}

// TestFetchDiscogsCollection tests the fetchDiscogsCollection function
func TestFetchDiscogsCollection(t *testing.T) {
	// Set up test Discogs token
	os.Setenv("DISCOGS_TOKEN", "test_token")

	tests := []struct {
		name         string
		page         int
		perPage      int
		expectedCode int
		hasError     bool
	}{
		{
			name:         "Valid request",
			page:         1,
			perPage:      12,
			expectedCode: http.StatusOK,
			hasError:     false,
		},
		{
			name:         "Invalid page",
			page:         0,
			perPage:      12,
			expectedCode: http.StatusBadRequest,
			hasError:     true,
		},
		{
			name:         "Invalid per_page",
			page:         1,
			perPage:      0,
			expectedCode: http.StatusBadRequest,
			hasError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			albums, totalItems, err := fetchDiscogsCollection(tt.page, tt.perPage)
			if (err != nil) != tt.hasError {
				t.Errorf("fetchDiscogsCollection() error = %v, hasError %v", err, tt.hasError)
				return
			}
			if !tt.hasError {
				if len(albums) > tt.perPage {
					t.Errorf("fetchDiscogsCollection() returned too many albums: got %d, want <= %d", len(albums), tt.perPage)
				}
				if totalItems < 0 {
					t.Errorf("fetchDiscogsCollection() returned invalid total items: %d", totalItems)
				}
			}
		})
	}
}

// TestCacheManagement tests the caching functionality
func TestCacheManagement(t *testing.T) {
	// Test cache initialization
	if len(discogsCache) != 0 {
		t.Error("Cache should be empty initially")
	}

	// Test cache update
	testAlbum := album{
		ID:     "123",
		Title:  "Test Album",
		Artist: "Test Artist",
		Year:   "2024",
		Price:  29.99,
	}

	cacheMutex.Lock()
	discogsCache = append(discogsCache, testAlbum)
	discogsCacheTime = time.Now()
	cacheMutex.Unlock()

	// Verify cache update
	cacheMutex.RLock()
	if len(discogsCache) != 1 {
		t.Error("Cache should contain one item")
	}
	if discogsCache[0].ID != testAlbum.ID {
		t.Error("Cache should contain the test album")
	}
	cacheMutex.RUnlock()
}

// TestEnvironmentVariables tests environment variable handling
func TestEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "Existing variable",
			key:      "TEST_VAR",
			value:    "test_value",
			expected: "test_value",
		},
		{
			name:     "Non-existent variable",
			key:      "NON_EXISTENT",
			value:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
			}
			value := os.Getenv(tt.key)
			if value != tt.expected {
				t.Errorf("getEnv() = %v, want %v", value, tt.expected)
			}
		})
	}
}

// TestGetAlbumByID tests the GET /albums/:id endpoint
func TestGetAlbumByID(t *testing.T) {
	router := SetupRouter()

	// Test case 1: Valid album ID
	req, err := http.NewRequest("GET", "/albums/1", nil)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response album
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "1", response.ID)
	assert.Equal(t, "Blue Train", response.Title)

	// Test case 2: Invalid album ID
	req, err = http.NewRequest("GET", "/albums/999", nil)
	assert.NoError(t, err)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errorResponse gin.H
	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)

	assert.Equal(t, "album not found", errorResponse["message"])
}

// TestPostAlbums tests the POST /albums endpoint
func TestPostAlbums(t *testing.T) {
	router := SetupRouter()

	// Create a new album
	newAlbum := album{
		ID:     "4",
		Title:  "Test Album",
		Artist: "Test Artist",
		Year:   "2023",
		Price:  29.99,
	}

	// Convert the album to JSON
	jsonData, err := json.Marshal(newAlbum)
	assert.NoError(t, err)

	// Create a request to POST /albums
	req, err := http.NewRequest("POST", "/albums", bytes.NewBuffer(jsonData))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Check status code
	assert.Equal(t, http.StatusCreated, w.Code)

	// Parse the response
	var response album
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Check that the response matches the input
	assert.Equal(t, newAlbum.ID, response.ID)
	assert.Equal(t, newAlbum.Title, response.Title)
	assert.Equal(t, newAlbum.Artist, response.Artist)
	assert.Equal(t, newAlbum.Year, response.Year)
	assert.Equal(t, newAlbum.Price, response.Price)

	// Verify the album was added to the albums slice
	assert.Equal(t, 4, len(albums))
	assert.Equal(t, "4", albums[3].ID)
	assert.Equal(t, "Test Album", albums[3].Title)
}

// TestAPIEndpoint tests the API endpoint
func TestAPIEndpoint(t *testing.T) {
	router := SetupRouter()

	// Create a request to GET /api/
	req, err := http.NewRequest("GET", "/api/", nil)
	assert.NoError(t, err)

	// Create a response recorder
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Check status code
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response
	var response gin.H
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Check the response
	assert.Equal(t, "pong", response["message"])
}
