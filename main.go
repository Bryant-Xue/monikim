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
    Port                 string   `yaml:"port"`
    ImageDir             string   `yaml:"image_dir"`
    AllowedExtensions    []string `yaml:"allowed_extensions"`
    DisableFileTypeCheck bool     `yaml:"disable_file_type_check"`
    FaviconPath          string   `yaml:"favicon_path"`
    CorsEnabled          bool     `yaml:"cors_enabled"`
    AllowedOrigins       []string `yaml:"allowed_origins"`
    AllowedMethods       []string `yaml:"allowed_methods"`
    AllowedHeaders       []string `yaml:"allowed_headers"`
}

// loadConfig loads configuration from the specified YAML file
func loadConfig(configPath string) (*Config, error) {
        file, err := os.Open(configPath)
        if err != nil {
                return nil, fmt.Errorf("无法读取配置文件: %v", err)
        }
        defer file.Close()

        var config Config
        if err := yaml.NewDecoder(file).Decode(&config); err != nil {
                return nil, fmt.Errorf("解析配置文件出错: %v", err)
        }

        return &config, nil
}

func handleCORS(w http.ResponseWriter, r *http.Request, config *Config) {
    if config.CorsEnabled {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        if len(config.AllowedOrigins) > 0 {
            for _, origin := range config.AllowedOrigins {
                if origin == "*" || origin == r.Header.Get("Origin") {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
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

var lastFileName string

// serveRandomImage serves a random image from the specified directory
func serveRandomImage(w http.ResponseWriter, r *http.Request, config *Config, verbose bool) {
    // 处理 CORS
    handleCORS(w, r, config)
    w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
    w.Header().Set("Expires", "0")
    w.Header().Set("Pragma", "no-cache")
    w.Header().Set("Surrogate-Control", "no-store")
        startTime := time.Now()
        if verbose {
                // 获取客户端的IP地址
                clientIP := r.Header.Get("X-Forwarded-For")
                if clientIP == "" {
                        clientIP = r.RemoteAddr // 如果没有 X-Forwarded-For，使用RemoteAddr
                }

                log.Printf("开始处理请求...\n")
                log.Printf("请求者IP: %s\n", clientIP)
                log.Printf("请求方法: %s\n", r.Method)
                log.Printf("请求路径: %s\n", r.URL.Path)
        }

        if r.URL.Path == "/favicon.ico" {
                if verbose {
                        log.Println("请求 favicon.ico")
                }
                serveFavicon(w, r, config.FaviconPath)
                // 计算处理耗时
                elapsedTime := time.Since(startTime)
                if verbose {
                        log.Printf("处理耗时: %v\n", elapsedTime)
                }
                return
        }

        files, err := os.ReadDir(config.ImageDir)
        if err != nil {
                http.Error(w, "无法读取图片目录", http.StatusInternalServerError)
                return
        }

        var validFiles []os.DirEntry
        for _, file := range files {
                if file.IsDir() {
                        continue
                }

                if config.DisableFileTypeCheck || isValidExtension(file.Name(), config.AllowedExtensions) {
                        validFiles = append(validFiles, file)
                }
        }

        if len(validFiles) == 0 {
                http.Error(w, "没有找到有效的图片", http.StatusNotFound)
                return
        }

        if len(validFiles) == 1 {
                if verbose {
                        log.Println("只有一张有效图片，直接返回。")
                }
                serveImageFile(w, r, config.ImageDir, validFiles[0].Name())
                lastFileName = validFiles[0].Name()
                return

        }

        rand.Seed(time.Now().UnixNano())
        var randomFile os.DirEntry
        for {
                randomFile = validFiles[rand.Intn(len(validFiles))]
                if randomFile.Name() != lastFileName {
                        if verbose {
                                log.Printf("选中的图片是：%s\n", randomFile.Name())
                        }
                        break
                }
        }

        lastFileName = randomFile.Name()
        if verbose {
                log.Printf("返回图片: %s\n", lastFileName)
        }
        serveImageFile(w, r, config.ImageDir, randomFile.Name())
        elapsedTime := time.Since(startTime)
        if verbose {
                log.Printf("处理耗时: %v\n", elapsedTime)
        }
}

func serveFavicon(w http.ResponseWriter, r *http.Request, faviconPath string) {
        file, err := os.Open(faviconPath)
        if err != nil {
                http.Error(w, "无法打开 favicon 文件", http.StatusInternalServerError)
                return
        }
        defer file.Close()

        http.ServeFile(w, r, faviconPath)
}

// serveImageFile serves the specified image file
func serveImageFile(w http.ResponseWriter, r *http.Request, imageDir, fileName string) {
        imagePath := filepath.Join(imageDir, fileName)

        file, err := os.Open(imagePath)
        if err != nil {
                http.Error(w, "无法打开图片文件", http.StatusInternalServerError)
                return
        }
        defer file.Close()

        http.ServeFile(w, r, imagePath)
}

// main is the entry point of the application
func main() {
        verbose := flag.Bool("v", false, "输出详细日志")
        flag.Parse()

        config, err := loadConfig("config.yaml")
        if err != nil {
                log.Fatalf("加载配置失败: %v", err)
        }

        http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
                serveRandomImage(w, r, config, *verbose)
        })

        log.Printf("服务器正在端口 %s 启动...", config.Port)
        if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
                log.Fatalf("服务器启动失败: %v", err)
        }
}