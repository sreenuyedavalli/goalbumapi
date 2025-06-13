package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
)

// DiscogsRelease represents a release from Discogs API
type DiscogsRelease struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Year        int     `json:"year"`
	Price       float64 `json:"price"`
	Thumb       string  `json:"thumb"`
	ResourceURL string  `json:"resource_url"`
}

// DiscogsResponse represents the response from Discogs API
type DiscogsResponse struct {
	Pagination struct {
		Page    int `json:"page"`
		Pages   int `json:"pages"`
		PerPage int `json:"per_page"`
		Items   int `json:"items"`
	} `json:"pagination"`
	Releases []struct {
		ID        int `json:"id"`
		BasicInfo struct {
			Title       string `json:"title"`
			Year        int    `json:"year"`
			ResourceURL string `json:"resource_url"`
			Thumb       string `json:"thumb"`
			Artists     []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"basic_information"`
	} `json:"releases"`
}

// PriceUpdate represents a price update request
type PriceUpdate struct {
	Price float64 `json:"price"`
}

// Cache for Discogs albums with prices
var (
	discogsCache     []album
	discogsCacheTime time.Time
	cacheMutex       sync.RWMutex
	cacheDuration    = 1 * time.Hour // Cache duration
	priceMutex       sync.RWMutex
	albumPrices      = make(map[string]float64) // Map of album ID to price
)

// updateAlbumPrice updates the price for an album
func updateAlbumPrice(c *gin.Context) {
	id := c.Param("id")
	var update PriceUpdate

	if err := c.BindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price data"})
		return
	}

	// Update price in memory
	priceMutex.Lock()
	albumPrices[id] = update.Price
	priceMutex.Unlock()

	// Update price in cache if album exists
	cacheMutex.Lock()
	for i, album := range discogsCache {
		if album.ID == id {
			discogsCache[i].Price = update.Price
			break
		}
	}
	cacheMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{"message": "Price updated successfully"})
}

// fetchDiscogsCollection fetches albums from Discogs API
func fetchDiscogsCollection(page int, perPage int) ([]album, int, error) {
	// Get Discogs token from environment variable
	discogsToken := os.Getenv("DISCOGS_TOKEN")
	if discogsToken == "" {
		return nil, 0, fmt.Errorf("DISCOGS_TOKEN environment variable not set")
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Fetch the requested page directly
	url := fmt.Sprintf("https://api.discogs.com/users/sreenu1/collection/folders/0/releases?page=%d&per_page=%d", page, perPage)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("User-Agent", "AlbumCollectionApp/1.0")
	req.Header.Add("Authorization", fmt.Sprintf("Discogs token=%s", discogsToken))

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("discogs API returned status %d: %s", resp.StatusCode, string(body))
	}

	var discogsResp DiscogsResponse
	if err := json.Unmarshal(body, &discogsResp); err != nil {
		return nil, 0, fmt.Errorf("error parsing response: %v", err)
	}

	// Convert Discogs releases to our album format
	var albums []album
	for _, release := range discogsResp.Releases {
		// Get the artist from the artists array if available
		artist := "Unknown Artist"
		if len(release.BasicInfo.Artists) > 0 {
			artist = release.BasicInfo.Artists[0].Name
		}

		// Get the label from the labels array if available
		label := "Unknown Label"
		if len(release.BasicInfo.Labels) > 0 {
			label = release.BasicInfo.Labels[0].Name
		}

		title := release.BasicInfo.Title

		// Get stored price if exists
		priceMutex.RLock()
		price := albumPrices[strconv.Itoa(release.ID)]
		priceMutex.RUnlock()

		albums = append(albums, album{
			ID:          strconv.Itoa(release.ID),
			Title:       title,
			Artist:      artist,
			Year:        strconv.Itoa(release.BasicInfo.Year),
			Price:       price,
			Thumb:       release.BasicInfo.Thumb,
			ResourceURL: release.BasicInfo.ResourceURL,
			Label:       label,
		})
	}

	// Update cache with the current page's data
	cacheMutex.Lock()
	if len(discogsCache) == 0 {
		// Initialize cache if empty
		discogsCache = make([]album, discogsResp.Pagination.Items)
	}

	// Update the cache with the current page's data
	start := (page - 1) * perPage
	end := start + len(albums)
	if end > len(discogsCache) {
		end = len(discogsCache)
	}
	copy(discogsCache[start:end], albums)
	discogsCacheTime = time.Now()
	cacheMutex.Unlock()

	// Debug logging
	fmt.Printf("Page %d: Retrieved %d albums, Total items: %d\n",
		page, len(albums), discogsResp.Pagination.Items)

	return albums, discogsResp.Pagination.Items, nil
}

// postAlbums adds an album from JSON received in the request body.
func postAlbums(c *gin.Context) {
	var newAlbum album

	// Call BindJSON to bind the received JSON to
	// newAlbum.
	if err := c.BindJSON(&newAlbum); err != nil {
		return
	}

	// Add the new album to the slice.
	albums = append(albums, newAlbum)
	c.IndentedJSON(http.StatusCreated, newAlbum)
}

// getAlbumByID locates the album whose ID value matches the id
// parameter sent by the client, then returns that album as a response.
func getAlbumByID(c *gin.Context) {
	id := c.Param("id")

	// Loop over the list of albums, looking for
	// an album whose ID value matches the parameter.
	for _, a := range albums {
		if a.ID == id {
			c.IndentedJSON(http.StatusOK, a)
			return
		}
	}
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
}

// album represents data about a record album.
type album struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Year        string  `json:"year"`
	Price       float64 `json:"price"`
	Thumb       string  `json:"thumb"`
	ResourceURL string  `json:"resource_url"`
	Label       string  `json:"label"`
}

// getAlbums responds with the list of all albums as JSON.
func getAlbums(c *gin.Context) {
	c.Header("Content-Type", "application/json")

	// Get pagination parameters from query string
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "12"))
	sortBy := c.DefaultQuery("sort_by", "none")

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 12
	}

	// Fetch albums from Discogs
	discogsAlbums, totalItems, err := fetchDiscogsCollection(page, perPage)
	if err != nil {
		// Log the error for debugging
		fmt.Printf("Error fetching albums from Discogs: %v\n", err)

		// Return appropriate error message to client
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Sort albums if requested
	switch sortBy {
	case "label":
		// Sort albums by label
		sort.Slice(discogsAlbums, func(i, j int) bool {
			return discogsAlbums[i].Label < discogsAlbums[j].Label
		})
	case "year":
		// Sort albums by year (descending)
		sort.Slice(discogsAlbums, func(i, j int) bool {
			yearI, _ := strconv.Atoi(discogsAlbums[i].Year)
			yearJ, _ := strconv.Atoi(discogsAlbums[j].Year)
			return yearI > yearJ // Descending order
		})
	}

	// Calculate pagination metadata
	totalPages := (totalItems + perPage - 1) / perPage

	// Return paginated response
	c.JSON(http.StatusOK, gin.H{
		"albums": discogsAlbums,
		"pagination": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total_items":  totalItems,
			"total_pages":  totalPages,
		},
		"sort_by": sortBy,
	})
}

// albums slice to seed record album data.
var albums = []album{
	{ID: "1", Title: "Blue Train", Artist: "John Coltrane", Year: "1977", Price: 56.99},
	{ID: "2", Title: "Jeru", Artist: "Gerry Mulligan", Year: "1987", Price: 17.99},
	{ID: "3", Title: "Sarah Vaughan and Clifford Brown", Artist: "Sarah Vaughan", Year: "1997", Price: 39.99},
}

func main() {
	router := gin.Default()
	// Serve frontend static files
	router.Use(static.Serve("/", static.LocalFile("./views/js", true)))

	// Setup route group for the API
	api := router.Group("/api")
	{
		api.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})

		// Add price update endpoint
		api.PUT("/albums/:id/price", updateAlbumPrice)
	}

	router.GET("/albums", getAlbums)
	router.GET("/albums/:id", getAlbumByID)
	router.POST("/albums", postAlbums)

	// Serve admin page
	router.StaticFile("/admin", "./views/js/admin.html")

	router.Run("localhost:3000")
}
