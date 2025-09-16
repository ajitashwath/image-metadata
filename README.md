# Image Metadata Service
A REST API service for storing, retrieving, and searching image metadata using Redis as the backend storage and caching layer.

## Features
- **Fast Storage & Retrieval**: Store and retrieve image metadata with Redis hash structures
- **Advanced Search**: Search images by tags, format, and size with intelligent caching
- **Performance Optimized**: Built-in search result caching with automatic cache invalidation
- **Statistics Dashboard**: Real-time service statistics including memory usage
- **RESTful API**: Clean HTTP endpoints for all operations

## Prerequisites
- Go 1.25.1 or higher
- Redis server running on `localhost:6379`

## Installation

1. Clone the repository:
```bash
git clone https://github.com/ajitashwath/image-metadata.git
cd image-metadata
```

2. Install dependencies:
```bash
go mod tidy
```

3. Start Redis server (in WSL):
```bash
redis-server
```

4. Run the service:
```bash
go run main.go
```

The service will start on `http://localhost:8080`

## API Endpoints

### Store Image Metadata
**POST** `/images`

Store metadata for a new image.

**Request Body:**
```json
{
  "id": "img001",
  "name": "sunset.jpg",
  "tags": ["nature", "sunset", "landscape"],
  "size": 1024000,
  "format": "jpg",
  "width": 1920,
  "height": 1080
}
```

**Response:**
```json
{
  "status": "stored"
}
```

### Get Image by ID
**GET** `/images/{id}`

Retrieve metadata for a specific image.

**Response:**
```json
{
  "id": "img001",
  "name": "sunset.jpg",
  "tags": ["nature", "sunset", "landscape"],
  "size": 1024000,
  "format": "jpg",
  "width": 1920,
  "height": 1080
}
```

### Search Images
**GET** `/search?q={query}`

Search for images using various criteria.

**Query Formats:**
- `tag:nature` - Search by tag
- `format:jpg` - Search by format
- `size:>500000` - Search by size (greater than)
- `size:<2000000` - Search by size (less than)

**Response:**
```json
{
  "query": "tag:nature",
  "count": 2,
  "results": [
    {
      "id": "img001",
      "name": "sunset.jpg",
      "tags": ["nature", "sunset"],
      "size": 1024000,
      "format": "jpg",
      "width": 1920,
      "height": 1080
    }
  ]
}
```

### Service Statistics
**GET** `/stats`

Get service statistics and Redis memory usage.

**Response:**
```json
{
  "total_images": 150,
  "cached_searches": 25,
  "redis_memory_usage": "2.5M"
}
```

## Usage Examples

### Store Image Metadata
```bash
curl -X POST http://localhost:8080/images \
  -H "Content-Type: application/json" \
  -d '{
    "id": "img001",
    "name": "mountain.jpg",
    "tags": ["nature", "mountain", "landscape"],
    "size": 2048000,
    "format": "jpg",
    "width": 2560,
    "height": 1440
  }'
```

### Search by Tag
```bash
curl "http://localhost:8080/search?q=tag:nature"
```

### Search by Format
```bash
curl "http://localhost:8080/search?q=format:png"
```

### Search by Size
```bash
# Images larger than 1MB
curl "http://localhost:8080/search?q=size:>1000000"

# Images smaller than 500KB
curl "http://localhost:8080/search?q=size:<500000"
```

### Get Specific Image
```bash
curl http://localhost:8080/images/img001
```

### Get Service Statistics
```bash
curl http://localhost:8080/stats
```

## Architecture

### Data Storage
- **Image Metadata**: Stored as Redis hashes with key pattern `image:{id}`
- **Tag Indexes**: Redis sets with key pattern `tag:{tagname}` containing image IDs
- **Format Indexes**: Redis sets with key pattern `format:{format}` containing image IDs

### Caching Strategy
- Search results are cached for 5 minutes using key pattern `search:{query}`
- Cache is automatically invalidated when new images with matching tags/formats are stored
- Cache hits and misses are logged for monitoring

### Performance Features
- **Indexed Searches**: Fast `O(1)` lookups for tags and formats using Redis sets
- **Intelligent Caching**: Search results cached with automatic invalidation
- **Bulk Operations**: Efficient batch processing for search results
- **Memory Monitoring**: Real-time Redis memory usage tracking

## Configuration

The service uses the following default Redis configuration:
- **Host**: `localhost`
- **Port**: `6379`
- **Database**: `0`

To modify these settings, update the `NewService()` function in `main.go`.

## Error Handling

The API returns appropriate HTTP status codes:
- `200 OK` - Successful operations
- `400 Bad Request` - Invalid request format or query
- `404 Not Found` - Image not found
- `500 Internal Server Error` - Server-side errors

## Dependencies
- **gorilla/mux**: HTTP router for REST endpoints
- **go-redis/redis/v8**: Redis client for data storage
- **Standard Library**: JSON encoding, HTTP handling, logging

## Development

### Running Tests
```bash
go test ./...
```

### Building for Production
```bash
go build -o image-service main.go
./image-service
```

## Monitoring
The service provides built-in monitoring through:
- Request logging for cache hits/misses
- Redis memory usage statistics
- Image count and cached search metrics

Access monitoring data via the `/stats` endpoint or check application logs.
