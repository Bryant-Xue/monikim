package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
)

// Config represents the configuration for the server
type Config struct {
	Port                 string            `yaml:"port"`
	ImageDir             string            `yaml:"image_dir"`
	AllowedExtensions    []string          `yaml:"allowed_extensions"`
	DisableFileTypeCheck bool              `yaml:"disable_file_type_check"`
	FaviconPath          string            `yaml:"favicon_path"`
	CorsEnabled          bool              `yaml:"cors_enabled"`
	AllowedOrigins       []string          `yaml:"allowed_origins"`
	AllowedMethods       []string          `yaml:"allowed_methods"`
	AllowedHeaders       []string          `yaml:"allowed_headers"`
	Mode                 string            `yaml:"mode"`
	RefererCheckEnabled  bool              `yaml:"referer_check_enabled"`
	AllowedReferers      []string          `yaml:"allowed_referers"`
	ParamSourceMapping   map[string]string `yaml:"param_source_mapping"`
}

// loadConfig loads configuration from the specified YAML file
func loadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("\u65e0\u6cd5\u8bfb\u53d6\u914d\u7f6e\u6587\u4ef6: %v", err)
	}
	defer file.Close()

	var config Config
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("\u89e3\u6790\u914d\u7f6e\u6587\u4ef6\u51fa\u9519: %v", err)
	}

	return &config, nil
}

// handleCORS sets the appropriate CORS headers based on the config
func handleCORS(w http.ResponseWriter, r *http.Request, config *Config) {
	if config.CorsEnabled {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if len(config.AllowedOrigins) > 0 {
			origin := r.Header.Get("Origin")
			for _, allowedOrigin := range config.AllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
					break
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		if len(config.AllowedMethods) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", fmt.Sprintf("%s", config.AllowedMethods))
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if len(config.AllowedHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", fmt.Sprintf("%s", config.AllowedHeaders))
		}
	}
}

// isValidExtension checks if the file extension is valid
func isValidExtension(fileName string, allowedExtensions []string) bool {
	ext := filepath.Ext(fileName)
	for _, allowed := range allowedExtensions {
		if ext == allowed {
			return true
		}
	}
	return false
}

// serveImageRedirect redirects to the image URL instead of serving it directly
func serveImageRedirect(w http.ResponseWriter, imagePath string) {
	http.Redirect(w, &http.Request{}, imagePath, http.StatusFound)
}

// serveImageFile serves the specified image file
func serveImageFile(w http.ResponseWriter, imagePath string) {
	http.ServeFile(w, &http.Request{}, imagePath)
}

// contains checks if a slice contains a given element
func contains(slice []string, item string) bool {
	for _, element := range slice {
		if element == item {
			return true
		}
	}
	return false
}

// handleImageRequest processes the image request logic
func handleImageRequest(w http.ResponseWriter, r *http.Request, config *Config, imageDir string) {
	files, err := os.ReadDir(imageDir)
	if err != nil {
		http.Error(w, "\u65e0\u6cd5\u8bfb\u53d6\u56fe\u7247\u76ee\u5f55", http.StatusInternalServerError)
		return
	}

	var validFiles []os.DirEntry
	for _, file := range files {
		if !file.IsDir() && (config.DisableFileTypeCheck || isValidExtension(file.Name(), config.AllowedExtensions)) {
			validFiles = append(validFiles, file)
		}
	}

	if len(validFiles) == 0 {
		http.Error(w, "\u6ca1\u6709\u627e\u5230\u6709\u6548\u7684\u56fe\u7247", http.StatusNotFound)
		return
	}

	rand.Seed(time.Now().UnixNano())
	selectedFile := validFiles[rand.Intn(len(validFiles))]
	imagePath := filepath.Join(imageDir, selectedFile.Name())
	if config.Mode == "redir" {
		serveImageRedirect(w, imagePath)
	} else {
		serveImageFile(w, imagePath)
	}
}

// main is the entry point of the application
func main() {
	flag.Parse()

	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("\u52a0\u8f7d\u914d\u7f6e\u5931\u8d25: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		param := r.URL.Query().Get("source")
		imageDir := config.ImageDir
		if customDir, exists := config.ParamSourceMapping[param]; exists {
			imageDir = customDir
		}
		referer := r.Referer()
		if config.RefererCheckEnabled && !contains(config.AllowedReferers, referer) {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		handleImageRequest(w, r, config, imageDir)
	})

	log.Printf("\u670d\u52a1\u5668\u6b63\u5728\u7aef\u53e3 %s \u542f\u52a8...", config.Port)
	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		log.Fatalf("\u670d\u52a1\u5668\u542f\u52a8\u5931\u8d25: %v", err)
	}
}
