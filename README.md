# Vinyl Monkey

A web application for managing and displaying your Discogs vinyl collection with pricing capabilities.

## Features

- View your Discogs collection in a responsive grid layout
- Sort albums by record label or release year
- Admin interface for managing album prices
- Pagination support for large collections
- Caching for improved performance
- Modern, responsive UI
- Multi-architecture Docker support (amd64, arm64)

## Prerequisites

- Go 1.16 or higher
- A Discogs API token
- Git
- Docker and Docker Compose (optional)

## Installation

### Option 1: Local Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd goalbumapi
```

2. Install Go dependencies:
```bash
go mod download
```

3. Set up your Discogs API token:
   - Go to [Discogs Developer Settings](https://www.discogs.com/settings/developers)
   - Create a new token
   - Set the token as an environment variable:
```bash
export DISCOGS_TOKEN="your-token-here"
```

### Option 2: Docker Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd goalbumapi
```

2. Set up your Discogs API token:
```bash
export DISCOGS_TOKEN="your-token-here"
```

## Building and Running

### Option 1: Local Build

1. Build the application:
```bash
go build
```

2. Run the application:
```bash
./goalbumapi
```

### Option 2: Docker Build

1. Build and run using Docker Compose:
```bash
docker-compose up --build
```

Or build for specific architectures:
```bash
# Build for amd64
docker buildx build --platform linux/amd64 -t albumapp:latest .

# Build for arm64
docker buildx build --platform linux/arm64 -t albumapp:latest .

# Build for both architectures
docker buildx build --platform linux/amd64,linux/arm64 -t albumapp:latest .
```

The application will start on `http://localhost:3000`

## Usage

### Public View
- Visit `http://localhost:3000` to view your collection
- Use the sort dropdown to organize albums by:
  - Default order
  - Record label
  - Release year
- Navigate through pages using the pagination controls

### Admin View
- Visit `http://localhost:3000/admin` to manage album prices
- Set prices for each album in your collection
- Prices are saved in memory and persist until server restart

## Screenshots

### Public View
<img width="1301" alt="Screenshot 2025-06-12 at 11 34 05 PM" src="https://github.com/user-attachments/assets/adbfa066-880f-46d3-849a-47ffb6ca0659" />

*The main collection view showing albums sorted by default order*

### Admin View
<img width="1342" alt="Screenshot 2025-06-12 at 11 35 21 PM" src="https://github.com/user-attachments/assets/d1aac976-97f3-4f8a-9b0b-86f3b5d1a974" />

*The admin interface for managing album prices*

## API Endpoints

- `GET /albums` - Get paginated list of albums
  - Query parameters:
    - `page` (default: 1)
    - `per_page` (default: 12)
    - `sort_by` (options: none, label, year)
- `PUT /api/albums/:id/price` - Update album price
  - Request body: `{"price": 29.99}`

## Development

### Project Structure
```
goalbumapi/
├── main.go           # Main application code
├── main_test.go      # Test files
├── Dockerfile        # Multi-arch Docker build file
├── docker-compose.yml # Docker Compose configuration
├── views/
│   └── js/
│       ├── index.html    # Public view
│       └── admin.html    # Admin interface
└── README.md
```

### Adding New Features
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
