package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

type ImageMetadata struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Tags   []string `json:"tags"`
	Size   int64    `json:"size"`
	Format string   `json:"format"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
}

type Service struct {
	redis *redis.Client
	ctx   context.Context
}

func NewService() *Service {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	return &Service{
		redis: rdb,
		ctx:   context.Background(),
	}
}

func (s *Service) invalidateSearchCaches(tags []string, format string) {
	for _, tag := range tags {
		pattern := fmt.Sprintf("search:tag:%s*", tag)
		keys, _ := s.redis.Keys(s.ctx, pattern).Result()
		if len(keys) > 0 {
			s.redis.Del(s.ctx, keys...)
		}
	}

	pattern := fmt.Sprintf("search:format:%s*", format)
	keys, _ := s.redis.Keys(s.ctx, pattern).Result()
	if len(keys) > 0 {
		s.redis.Del(s.ctx, keys...)
	}
}

func (s *Service) StoreImage(img *ImageMetadata) error {
	key := fmt.Sprintf("image:%s", img.ID)
	data := map[string]interface{}{
		"name":   img.Name,
		"tags":   strings.Join(img.Tags, ","),
		"size":   img.Size,
		"format": img.Format,
		"width":  img.Width,
		"height": img.Height,
	}
	err := s.redis.HMSet(s.ctx, key, data).Err()
	if err != nil {
		return err
	}

	for _, tag := range img.Tags {
		s.redis.SAdd(s.ctx, fmt.Sprintf("tag:%s", tag), img.ID)
	}

	s.redis.SAdd(s.ctx, fmt.Sprintf("format:%s", img.Format), img.ID)
	s.invalidateSearchCaches(img.Tags, img.Format)
	return nil
}

func (s *Service) GetImage(id string) (*ImageMetadata, error) {
	key := fmt.Sprintf("image:%s", id)

	data, err := s.redis.HGetAll(s.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("image not found")
	}

	size, _ := strconv.ParseInt(data["size"], 10, 64)
	width, _ := strconv.Atoi(data["width"])
	height, _ := strconv.Atoi(data["height"])

	var tags []string
	if data["tags"] != "" {
		tags = strings.Split(data["tags"], ",")
	}

	return &ImageMetadata{
		ID:     id,
		Name:   data["name"],
		Tags:   tags,
		Size:   size,
		Format: data["format"],
		Width:  width,
		Height: height,
	}, nil
}

func (s *Service) filterBySize(sizeLimit int64, greater bool) ([]string, error) {
	var matchingIDs []string
	keys, err := s.redis.Keys(s.ctx, "image:*").Result()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		sizeStr, err := s.redis.HGet(s.ctx, key, "size").Result()
		if err != nil {
			continue
		}
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			continue
		}

		if (greater && size > sizeLimit) || (!greater && size < sizeLimit) {
			id := strings.TrimPrefix(key, "image:")
			matchingIDs = append(matchingIDs, id)
		}
	}
	return matchingIDs, nil
}

func (s *Service) SearchImages(query string) ([]string, error) {
	cacheKey := fmt.Sprintf("search:%s", query)
	cached, err := s.redis.Get(s.ctx, cacheKey).Result()
	if err == nil {
		var ids []string
		json.Unmarshal([]byte(cached), &ids)
		log.Printf("Cache hit for query: %s", query)
		return ids, nil
	}

	parts := strings.SplitN(query, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid Query Format")
	}

	searchType, value := parts[0], parts[1]
	var ids []string

	switch searchType {
	case "tag":
		ids, err = s.redis.SMembers(s.ctx, fmt.Sprintf("tag:%s", value)).Result()
	case "format":
		ids, err = s.redis.SMembers(s.ctx, fmt.Sprintf("format:%s", value)).Result()
	case "size":
		// Simple size filter
		if strings.HasPrefix(value, ">") {
			minSize, _ := strconv.ParseInt(value[1:], 10, 64)
			ids, err = s.filterBySize(minSize, true)
		} else if strings.HasPrefix(value, "<") {
			maxSize, _ := strconv.ParseInt(value[1:], 10, 64)
			ids, err = s.filterBySize(maxSize, false)
		}
	default:
		return nil, fmt.Errorf("Unsupported Search Type")
	}
	if err != nil {
		return nil, err
	}

	jsonData, _ := json.Marshal(ids)
	s.redis.SetEX(s.ctx, cacheKey, jsonData, 5*time.Minute)
	log.Printf("Cache miss for query: %s, Cached %d results", query, len(ids))
	return ids, nil
}

func (s *Service) handleStoreImage(w http.ResponseWriter, r *http.Request) {
	var img ImageMetadata
	if err := json.NewDecoder(r.Body).Decode(&img); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.StoreImage(&img); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stored"})
}

func (s *Service) handleGetImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	img, err := s.GetImage(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(img)
}

func (s *Service) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' required", http.StatusBadRequest)
		return
	}
	ids, err := s.SearchImages(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var images []ImageMetadata
	for _, id := range ids {
		if img, err := s.GetImage(id); err == nil {
			images = append(images, *img)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   query,
		"count":   len(images),
		"results": images,
	})
}

func (s *Service) getMemoryUsage() string {
	info, err := s.redis.Info(s.ctx, "memory").Result()
	if err != nil {
		return "unknown"
	}
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Used_memory_human:") {
			return strings.TrimPrefix(line, "Used_memory_human:")
		}
	}
	return "unknown"
}

func (s *Service) handleStats(w http.ResponseWriter, r *http.Request) {
	imageKeys, _ := s.redis.Keys(s.ctx, "image:*").Result()
	cacheKeys, _ := s.redis.Keys(s.ctx, "search:*").Result()

	stats := map[string]interface{}{
		"total_images":       len(imageKeys),
		"cached_searches":    len(cacheKeys),
		"redis_memory_usage": s.getMemoryUsage(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func main() {
	service := NewService()
	r := mux.NewRouter()

	r.HandleFunc("/images", service.handleStoreImage).Methods("POST")
	r.HandleFunc("/images/{id}", service.handleGetImage).Methods("GET")
	r.HandleFunc("/search", service.handleSearch).Methods("GET")
	r.HandleFunc("/stats", service.handleStats).Methods("GET")

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `
Image Metadata Service

API Endpoints:
- POST /images - Store image metadata
- GET /images/{id} - Get image by ID  
- GET /search?q={query} - Search images
- GET /stats - Service statistics

Example Usage:

1. Store an image:
curl -X POST http://localhost:8080/images \
  -H "Content-Type: application/json" \
  -d '{"id":"img1", "name":"sunset.jpg", "tags":["nature","sunset"], "size":1024000, "format":"jpg", "width":1920, "height":1080}'

2. Search by tag:
curl "http://localhost:8080/search?q=tag:nature"

3. Search by format:
curl "http://localhost:8080/search?q=format:jpg"

4. Search by size:
curl "http://localhost:8080/search?q=size:>500000"

5. Get specific image:
curl http://localhost:8080/images/img1

6. Get stats:
curl http://localhost:8080/stats
`)
	})

	fmt.Println("Image Metadata Service starting on :8080")
	fmt.Println("Make sure Redis is running on localhost:6379")
	log.Fatal(http.ListenAndServe(":8080", r))
}
