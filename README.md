# Album Collection Manager

A web application for managing your Discogs album collection and prices. The application fetches your album collection from Discogs and allows you to manage prices for each album.

## Features

- **Discogs Integration**: Automatically fetches your album collection from Discogs
- **Price Management**: Set and update prices for your albums
- **PayPal Integration**: Accept payments for albums with PayPal buttons
- **Sorting**: Sort albums by title, artist, year, or price
- **Pagination**: Browse through your collection with paginated results
- **Responsive Design**: Works on both desktop and mobile devices
- **Admin Interface**: Secure interface for managing album prices with authentication
- **Session Management**: Secure login/logout functionality for admin access

## Prerequisites

- Go 1.21 or later
- PostgreSQL database
- Discogs API token
- Environment variables configured

## Environment Variables

Create a `.env` file in the root directory with the following variables:

```env
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=albumdb

# Discogs API Configuration
DISCOGS_TOKEN=your_discogs_token

# Admin Authentication
ADMIN_PASSWORD=your_secure_admin_password

# Server Configuration
PORT=3000
```

## PayPal Configuration

The application includes PayPal integration for accepting payments. Currently, it uses PayPal's sandbox environment for testing.

### PayPal Client ID

To use PayPal in production, you'll need to:

1. Create a PayPal Developer account at [developer.paypal.com](https://developer.paypal.com)
2. Create a PayPal app to get your Client ID
3. Update the PayPal SDK script in `views/js/index.html`:
   ```html
   <script src="https://www.paypal.com/sdk/js?client-id=YOUR_CLIENT_ID&currency=USD"></script>
   ```

### Testing PayPal Integration

- The current implementation uses PayPal's sandbox environment
- Test payments can be made using PayPal's sandbox accounts
- All orders are stored in the database for tracking

### Production Considerations

For production deployment, consider:
- Verifying payments with PayPal's API
- Implementing webhook handling for payment notifications
- Adding inventory management
- Setting up email notifications for orders
- Implementing shipping integration

## Database Setup

1. Create a PostgreSQL database named `albumdb`
2. The application will automatically create the required tables:
   ```sql
   CREATE TABLE IF NOT EXISTS album_prices (
       album_id VARCHAR(255) PRIMARY KEY,
       price DECIMAL(10,2) NOT NULL,
       updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
   );

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
   );
   ```

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/goalbumapi.git
   cd goalbumapi
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   go build
   ```

4. Run the application:
   ```bash
   ./goalbumapi
   ```

The application will be available at `http://localhost:3000`

## API Endpoints

### Public Endpoints

- `GET /` - Public album collection view
- `GET /albums` - Get paginated list of albums
  - Query Parameters:
    - `page` (default: 1) - Page number
    - `per_page` (default: 12) - Items per page
    - `sort_by` (default: none) - Sort field (title, artist, year, price)
    - `direction` (default: desc) - Sort direction (asc, desc)

### Authentication Endpoints

- `GET /login` - Admin login page
- `POST /api/auth/login` - Authenticate admin user
  - Request Body:
    ```json
    {
        "password": "your_admin_password"
    }
    ```
- `POST /api/auth/logout` - Logout admin user
- `GET /api/auth/check` - Check authentication status

### Admin Endpoints (Protected)

- `GET /admin` - Admin interface (requires authentication)
- `PUT /api/admin/albums/:id/price` - Update album price (requires authentication)
  - Request Body:
    ```json
    {
        "price": 29.99
    }
    ```

### PayPal Endpoints

- `POST /api/paypal/complete-payment` - Process completed PayPal payment
  - Request Body:
    ```json
    {
        "orderID": "PAYPAL_ORDER_ID",
        "albumID": "ALBUM_ID",
        "albumTitle": "Album Title",
        "albumArtist": "Artist Name",
        "amount": 29.99,
        "payerID": "PAYPAL_PAYER_ID",
        "paymentDetails": {}
    }
    ```
  - Response:
    ```json
    {
        "success": true,
        "message": "Payment processed successfully",
        "orderID": "PAYPAL_ORDER_ID"
    }
    ```

## Frontend

The application includes three main views:

1. **Public View** (`/`)
   - Displays album collection
   - Supports sorting and pagination
   - Shows album details and prices
   - PayPal payment buttons for albums with prices
   - No authentication required

2. **Login View** (`/login`)
   - Secure login form for admin access
   - Password-based authentication
   - Redirects to admin page on successful login

3. **Admin View** (`/admin`)
   - Secure interface for managing album prices
   - Enhanced sorting and filtering options
   - Price update functionality
   - Logout button
   - Requires authentication

## Security Features

- **Session-based Authentication**: Uses secure cookies for session management
- **Protected Routes**: Admin endpoints require valid authentication
- **Automatic Redirects**: Unauthenticated users are redirected to login
- **Secure Password Storage**: Admin password can be set via environment variable
- **CSRF Protection**: Built-in protection against cross-site request forgery

## Default Credentials

If no `ADMIN_PASSWORD` environment variable is set, the application uses a default password:
- **Default Password**: `admin123`

**⚠️ Security Note**: Always set a strong `ADMIN_PASSWORD` environment variable in production.

## Development

### Project Structure

```
goalbumapi/
├── main.go           # Main application file
├── db.go            # Database interface and implementation
├── go.mod           # Go module file
├── go.sum           # Go module checksum
├── views/           # Frontend templates
│   └── js/          # JavaScript files
│       ├── admin.html
│       ├── index.html
│       └── login.html
└── README.md        # This file
```

### Building for Production

1. Build the application:
   ```bash
   go build -o goalbumapi
   ```

2. Run with environment variables:
   ```bash
   ADMIN_PASSWORD=your_secure_password ./goalbumapi
   ```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [Discogs API](https://www.discogs.com/developers/) for providing the album collection data
- [Gin Web Framework](https://github.com/gin-gonic/gin) for the web server
- [Bootstrap](https://getbootstrap.com/) for the frontend styling
