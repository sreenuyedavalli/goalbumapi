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

// TestGetAlbums tests the GET /albums endpoint
func TestGetAlbums(t *testing.T) {
	router := SetupRouter()
	
	// Create a request to GET /albums
	req, err := http.NewRequest("GET", "/albums", nil)
	assert.NoError(t, err)
	
	// Create a response recorder
	w := httptest.NewRecorder()
	
	// Perform the request
	router.ServeHTTP(w, req)
	
	// Check status code
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Parse the response
	var response []album
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	
	// Check that we got the expected number of albums
	assert.Equal(t, 3, len(response))
	
	// Check the first album
	assert.Equal(t, "1", response[0].ID)
	assert.Equal(t, "Blue Train", response[0].Title)
	assert.Equal(t, "John Coltrane", response[0].Artist)
	assert.Equal(t, "1977", response[0].Year)
	assert.Equal(t, 56.99, response[0].Price)
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