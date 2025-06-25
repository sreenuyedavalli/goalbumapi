package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
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

// LoginRequest represents a login request
type LoginRequest struct {
	Password string `json:"password"`
}

// PayPalPaymentRequest represents a PayPal payment completion request
type PayPalPaymentRequest struct {
	OrderID        string                 `json:"orderID"`
	AlbumID        string                 `json:"albumID"`
	AlbumTitle     string                 `json:"albumTitle"`
	AlbumArtist    string                 `json:"albumArtist"`
	Amount         float64                `json:"amount"`
	PayerID        string                 `json:"payerID"`
	PaymentDetails map[string]interface{} `json:"paymentDetails"`
}

// PayPalPaymentResponse represents a PayPal payment response
type PayPalPaymentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	OrderID string `json:"orderID,omitempty"`
}

// Cache for Discogs albums with prices
var (
	discogsCache     []album
	discogsCacheTime time.Time
	cacheMutex       sync.RWMutex
	cacheDuration    = 1 * time.Hour // Cache duration
)

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

// DiscogsService interface for fetching collections
type DiscogsService interface {
	FetchCollection(db Database, page, perPage int) ([]album, int, error)
}

// RealDiscogsService implements DiscogsService
type RealDiscogsService struct{}

func (r *RealDiscogsService) FetchCollection(db Database, page, perPage int) ([]album, int, error) {
	return fetchDiscogsCollection(db, page, perPage)
}

// WikipediaResponse represents the response from Wikipedia API
type WikipediaResponse struct {
	Query struct {
		Pages map[string]struct {
			PageID  int    `json:"pageid"`
			Title   string `json:"title"`
			Extract string `json:"extract"`
		} `json:"pages"`
	} `json:"query"`
}

// WikipediaSearchResponse represents the search response from Wikipedia API
type WikipediaSearchResponse struct {
	Query struct {
		Search []struct {
			Title   string `json:"title"`
			Snippet string `json:"snippet"`
		} `json:"search"`
	} `json:"query"`
}

// fetchWikipediaDescription fetches album description from Wikipedia
func fetchWikipediaDescription(albumTitle, artist string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// First, search for the album
	searchQuery := fmt.Sprintf("%s %s album", albumTitle, artist)
	searchURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&format=json&list=search&srsearch=%s&srlimit=1",
		url.QueryEscape(searchQuery))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating search request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making search request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading search response: %v", err)
	}

	var searchResp WikipediaSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("error parsing search response: %v", err)
	}

	if len(searchResp.Query.Search) == 0 {
		return "", fmt.Errorf("no Wikipedia article found")
	}

	// Get the page content using the search result title
	pageTitle := searchResp.Query.Search[0].Title
	contentURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&format=json&prop=extracts&exintro=true&explaintext=true&titles=%s",
		url.QueryEscape(pageTitle))

	req, err = http.NewRequest("GET", contentURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating content request: %v", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making content request: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading content response: %v", err)
	}

	var wikiResp WikipediaResponse
	if err := json.Unmarshal(body, &wikiResp); err != nil {
		return "", fmt.Errorf("error parsing content response: %v", err)
	}

	// Extract the description from the first page
	for _, page := range wikiResp.Query.Pages {
		if page.Extract != "" {
			// Limit to first 200 characters and add ellipsis if longer
			description := page.Extract
			if len(description) > 200 {
				description = description[:200] + "..."
			}
			return description, nil
		}
	}

	return "", fmt.Errorf("no description found")
}

// getAlbumDescriptionHandler handles requests for album descriptions
func getAlbumDescriptionHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		albumTitle := c.Query("title")
		artist := c.Query("artist")

		if albumTitle == "" || artist == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Album title and artist are required"})
			return
		}

		description, err := fetchWikipediaDescription(albumTitle, artist)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No description found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"description": description})
	}
}

// initDB initializes the database connection and creates necessary tables
func initDB() (Database, error) {
	// Get database configuration from environment variables
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Validate required environment variables
	if dbHost == "" || dbPort == "" || dbUser == "" || dbPassword == "" || dbName == "" {
		return nil, fmt.Errorf("missing required database environment variables: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME")
	}

	// Create connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	// Create album_prices table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS album_prices (
			album_id VARCHAR(255) PRIMARY KEY,
			price DECIMAL(10,2) NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating album_prices table: %v", err)
	}

	// Create orders table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			order_id VARCHAR(255) UNIQUE NOT NULL,
			album_id VARCHAR(255) NOT NULL,
			album_title VARCHAR(500) NOT NULL,
			album_artist VARCHAR(500) NOT NULL,
			amount DECIMAL(10,2) NOT NULL,
			payer_id VARCHAR(255) NOT NULL,
			payment_status VARCHAR(50) DEFAULT 'completed',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating orders table: %v", err)
	}

	// Create album_of_month_signups table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS album_of_month_signups (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			signup_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating album_of_month_signups table: %v", err)
	}

	return &PostgresDB{db: db}, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getAlbumPrice retrieves the price for an album from the database
func getAlbumPrice(db Database, albumID string) (float64, error) {
	return db.GetAlbumPrice(albumID)
}

// updateAlbumPrice updates the price for an album in the database
func updateAlbumPrice(db Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var update PriceUpdate

		if err := c.BindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price data"})
			return
		}

		// Update price in database
		err := db.SetAlbumPrice(id, update.Price)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update price"})
			return
		}

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
}

// fetchDiscogsCollection fetches albums from Discogs API
func fetchDiscogsCollection(db Database, page int, perPage int) ([]album, int, error) {
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
		albumID := strconv.Itoa(release.ID)

		// Get stored price from database
		price, err := db.GetAlbumPrice(albumID)
		if err != nil {
			fmt.Printf("Error getting price for album %s: %v\n", albumID, err)
			price = 0
		}

		albums = append(albums, album{
			ID:          albumID,
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

// Update getAlbums to accept both Database and DiscogsService
func getAlbums(db Database, discogs DiscogsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json")

		// Get pagination parameters from query string
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "12"))
		sortBy := c.DefaultQuery("sort_by", "none")
		sortDirection := c.DefaultQuery("direction", "desc")

		if page < 1 {
			page = 1
		}
		if perPage < 1 {
			perPage = 12
		}

		albums, totalItems, err := discogs.FetchCollection(db, page, perPage)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Sort albums if requested
		if sortBy != "none" {
			switch sortBy {
			case "title":
				sort.Slice(albums, func(i, j int) bool {
					if sortDirection == "asc" {
						return albums[i].Title < albums[j].Title
					}
					return albums[i].Title > albums[j].Title
				})
			case "artist":
				sort.Slice(albums, func(i, j int) bool {
					if sortDirection == "asc" {
						return albums[i].Artist < albums[j].Artist
					}
					return albums[i].Artist > albums[j].Artist
				})
			case "year":
				sort.Slice(albums, func(i, j int) bool {
					yearI, _ := strconv.Atoi(albums[i].Year)
					yearJ, _ := strconv.Atoi(albums[j].Year)
					if sortDirection == "asc" {
						return yearI < yearJ
					}
					return yearI > yearJ
				})
			case "price":
				sort.Slice(albums, func(i, j int) bool {
					if sortDirection == "asc" {
						return albums[i].Price < albums[j].Price
					}
					return albums[i].Price > albums[j].Price
				})
			}
		}

		// Calculate pagination info
		totalPages := (totalItems + perPage - 1) / perPage
		if totalPages < 1 {
			totalPages = 1
		}

		// Ensure page is within bounds
		if page > totalPages {
			page = totalPages
		}

		// Create response
		response := gin.H{
			"albums": albums,
			"pagination": gin.H{
				"current_page": page,
				"total_pages":  totalPages,
				"total_items":  totalItems,
				"per_page":     perPage,
			},
			"sort": gin.H{
				"by":        sortBy,
				"direction": sortDirection,
			},
		}

		c.JSON(http.StatusOK, response)
	}
}

// Authentication middleware
func requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		authenticated := session.Get("authenticated")

		if authenticated == nil || authenticated != true {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Login handler
func loginHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginReq LoginRequest
		if err := c.BindJSON(&loginReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Get admin password from environment variable
		adminPassword := os.Getenv("ADMIN_PASSWORD")
		if adminPassword == "" {
			// Default password if not set (for development)
			adminPassword = "admin123"
			fmt.Printf("ADMIN_PASSWORD not set, using default password\n")
		} else {
			fmt.Printf("ADMIN_PASSWORD is set, length: %d\n", len(adminPassword))
		}

		// Debug: Log only non-sensitive information
		fmt.Printf("Login attempt - Received password length: %d\n", len(loginReq.Password))

		// Check password
		if loginReq.Password == adminPassword {
			session := sessions.Default(c)
			session.Set("authenticated", true)
			session.Save()

			fmt.Printf("Login successful\n")
			c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
		} else {
			fmt.Printf("Login failed - password mismatch\n")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		}
	}
}

// Logout handler
func logoutHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()

		c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
	}
}

// checkAuthHandler checks if user is authenticated
func checkAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		authenticated := session.Get("authenticated")

		if authenticated == true {
			c.JSON(http.StatusOK, gin.H{"authenticated": true})
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
		}
	}
}

// completePayPalPaymentHandler handles PayPal payment completion
func completePayPalPaymentHandler(db Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var paymentReq PayPalPaymentRequest
		if err := c.BindJSON(&paymentReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment data"})
			return
		}

		// Store the order in the database
		_, err := db.(*PostgresDB).db.Exec(`
			INSERT INTO orders (order_id, album_id, album_title, album_artist, amount, payer_id)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentReq.OrderID, paymentReq.AlbumID, paymentReq.AlbumTitle, paymentReq.AlbumArtist, paymentReq.Amount, paymentReq.PayerID)

		if err != nil {
			fmt.Printf("Error storing order: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
			return
		}

		// Log the payment for debugging
		fmt.Printf("PayPal payment completed and stored:\n")
		fmt.Printf("  Order ID: %s\n", paymentReq.OrderID)
		fmt.Printf("  Album ID: %s\n", paymentReq.AlbumID)
		fmt.Printf("  Album: %s by %s\n", paymentReq.AlbumTitle, paymentReq.AlbumArtist)
		fmt.Printf("  Amount: $%.2f\n", paymentReq.Amount)
		fmt.Printf("  Payer ID: %s\n", paymentReq.PayerID)

		// In a real application, you would also:
		// 1. Verify the payment with PayPal's API
		// 2. Send confirmation emails
		// 3. Update inventory
		// 4. Trigger shipping process

		response := PayPalPaymentResponse{
			Success: true,
			Message: "Payment processed successfully",
			OrderID: paymentReq.OrderID,
		}

		c.JSON(http.StatusOK, response)
	}
}

// AlbumOfMonthSignupRequest represents the sign-up form data
type AlbumOfMonthSignupRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Handler for album of the month sign-up
func albumOfMonthSignupHandler(db Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AlbumOfMonthSignupRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data"})
			return
		}
		if req.Name == "" || req.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name and email are required"})
			return
		}
		err := db.AddAlbumOfMonthSignup(req.Name, req.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save sign-up"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Sign-up successful"})
	}
}

func main() {
	// Initialize database
	db, err := initDB()
	if err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		os.Exit(1)
	}

	router := gin.Default()

	// Setup session middleware
	store := cookie.NewStore([]byte("vinyl-monkey-secret-key"))
	router.Use(sessions.Sessions("vinyl-monkey-session", store))

	// Serve static files using gin's built-in static file serving
	router.Static("/static", "./views/js")
	router.LoadHTMLGlob("./views/js/*.html")

	// Public routes
	router.GET("/albums", getAlbums(db, &RealDiscogsService{}))
	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})
	router.GET("/album-of-month", func(c *gin.Context) {
		c.HTML(http.StatusOK, "album-of-month.html", nil)
	})
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Setup route group for the API
	api := router.Group("/api")
	{
		api.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})

		// Authentication routes
		auth := api.Group("/auth")
		{
			auth.POST("/login", loginHandler())
			auth.POST("/logout", logoutHandler())
			auth.GET("/check", checkAuthHandler())
		}

		// PayPal payment routes
		paypal := api.Group("/paypal")
		{
			paypal.POST("/complete-payment", completePayPalPaymentHandler(db))
		}

		// Wikipedia description route
		api.GET("/album-description", getAlbumDescriptionHandler())

		// Protected admin routes
		admin := api.Group("/admin")
		admin.Use(requireAuth())
		{
			admin.PUT("/albums/:id/price", updateAlbumPrice(db))
		}

		// Album of the month sign-up route
		api.POST("/album-of-month-signup", albumOfMonthSignupHandler(db))
	}

	// Protected admin page
	router.GET("/admin", requireAuth(), func(c *gin.Context) {
		c.HTML(http.StatusOK, "admin.html", nil)
	})

	router.Run(":3000")
}
