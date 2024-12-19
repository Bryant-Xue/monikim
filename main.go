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
	ParamRedirects       map[string]string `yaml:"param_redirects"`
	RefererRestriction   bool              `yaml:"referer_restriction"`
	AllowedReferers      []string          `yaml:"allowed_referers"`
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

// validateReferer checks if the referer is allowed
func validateReferer(r *http.Request, allowedReferers []string) bool {
	if len(allowedReferers) == 0 {
		return true
	}
	referer := r.Referer()
	for _, allowed := range allowedReferers {
		if referer == allowed {
			return true
		}
	}
	return false
}

// serveFavicon serves the favicon if requested
func serveFavicon(w http.ResponseWriter, r *http.Request, faviconPath string) {
	http.ServeFile(w, r, faviconPath)
}

// serveRandomImage serves a random image or redirects to its URL based on the config
func serveRandomImage(w http.ResponseWriter, r *http.Request, config *Config, verbose bool) {
	if config.RefererRestriction && !validateReferer(r, config.AllowedReferers) {
		http.Error(w, "\u8bbf\u95ee\u88ab\u62d2\u7edd", http.StatusForbidden)
		return
	}

	handleCORS(w, r, config)
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	w.Header().Set("Expires", "0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Surrogate-Control", "no-store")

	files, err := os.ReadDir(config.ImageDir)
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

	if config.Mode == "redir" {
		http.Redirect(w, r, "/"+filepath.Join(config.ImageDir, selectedFile.Name()), http.StatusFound)
	} else {
		http.ServeFile(w, r, filepath.Join(config.ImageDir, selectedFile.Name()))
	}
}

func main() {
	verbose := flag.Bool("v", false, "\u8f93\u51fa\u8be6\u7ec6\u65e5\u5fd7")
	flag.Parse()

	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("\u52a0\u8f7d\u914d\u7f6e\u5931\u8d25: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveRandomImage(w, r, config, *verbose)
	})

	log.Printf("\u670d\u52a1\u5668\u6b63\u5728\u7aef\u53e3 %s \u542f\u52a8...", config.Port)
	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		log.Fatalf("\u670d\u52a1\u5668\u542f\u52a8\u5931\u8d25: %v", err)
	}
}
