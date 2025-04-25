package controllers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"lorraxs/fivem_cdn_server/config"
	"lorraxs/fivem_cdn_server/utils"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chai2010/webp"
	"github.com/gorilla/mux"
	"github.com/mazen160/go-random"
)

type ImageFile struct {
	Name     string
	FullPath string
	MimeType string
	Bytes    []byte
}

type BufferPayload struct {
	Name  string `json:"name"`
	Bytes []byte `json:"bytes"`
}

type UploadResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	FileName string `json:"fileName"`
	Url      string `json:"url"`
}

type UploadManifestCollectionItemTexture struct {
	TextureId string `json:"textureId"`
	Name      string `json:"name"`
	Url       string `json:"url"`
	Size      int    `json:"size"`
	Hash      string `json:"hash"`
}

type UploadManifestCollectionItem struct {
	CollectionName string                                `json:"collectionName"`
	Gender         string                                `json:"gender"`
	ComponentType  string                                `json:"componentType"`
	ComponentId    string                                `json:"componentId"`
	DrawableId     string                                `json:"drawableId"`
	Name           string                                `json:"name"`
	Url            string                                `json:"url"`
	Textures       []UploadManifestCollectionItemTexture `json:"textures"`
}

type UploadManifestCollection struct {
	CollectionName string                         `json:"collectionName"`
	Items          []UploadManifestCollectionItem `json:"items"`
}

type UploadManifestResponse struct {
	CollectionNum int                        `json:"collectionNum"`
	Collections   []UploadManifestCollection `json:"collections"`
}

type UploadController struct {
	ctx            context.Context
	Router         *mux.Router
	Config         *config.Config
	DB             *sql.DB
	CachedClothing []*ClothingItem
}

type ClothingItem struct {
	CollectionName string  `json:"cn"`
	Gender         int     `json:"g"`
	ComponentType  string  `json:"ct"`
	ComponentId    int     `json:"ci"`
	DrawableId     int     `json:"di"`
	TextureId      int     `json:"ti"`
	Size           int     `json:"s"`
	Hash           string  `json:"h"`
	Price          float64 `json:"p"`
}

type ClothingPrice struct {
	Hash  string  `json:"hash"`
	Price float64 `json:"price"`
}

func NewUploadController() *UploadController {
	return &UploadController{}
}

func (c *UploadController) Init(ctx context.Context, router *mux.Router, db *sql.DB) {
	c.DB = db
	c.ctx = ctx
	c.Router = router
	c.Config = config.GetConfig()
	clothing, err := c.GetClothing("null", true)
	if err != nil {
		fmt.Println("Error getting clothing:", err)
		return
	}
	c.CachedClothing = clothing.([]*ClothingItem)
	c.Router.HandleFunc("/static/{file}", c.DeleteStaticFile).Methods("DELETE")
	c.Router.HandleFunc("/upload", c.Upload).Methods("POST")
	c.Router.HandleFunc("/upload/manifest", c.GetUploadManifest).Methods("GET")
	c.Router.HandleFunc("/upload/clothing/flush_cache", func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("Secret")
		if secret != c.Config.App.Secret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		clothing, err := c.GetClothing("null", true)
		if err != nil {
			http.Error(w, "Error getting clothing", http.StatusInternalServerError)
			return
		}
		c.CachedClothing = clothing.([]*ClothingItem)
	}).Methods("GET")
	c.Router.HandleFunc("/upload/clothing/update_price", func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("Secret")
		if secret != c.Config.App.Secret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		hash := r.URL.Query().Get("hash")
		price := r.URL.Query().Get("price")
		if hash == "" || price == "" {
			http.Error(w, "Missing required parameters", http.StatusBadRequest)
			return
		}
		priceFloat, err := strconv.ParseFloat(price, 64)
		if err != nil {
			http.Error(w, "Invalid price parameter", http.StatusBadRequest)
			return
		}
		_, err = c.DB.Exec("INSERT INTO texture_prices (hash, price) VALUES (?, ?) ON DUPLICATE KEY UPDATE price = ?", hash, priceFloat, priceFloat)
		if err != nil {
			http.Error(w, "Error updating price", http.StatusInternalServerError)
			return
		}
		clothing, err := c.GetClothing("null", true)
		if err != nil {
			http.Error(w, "Error getting clothing", http.StatusInternalServerError)
			return
		}
		c.CachedClothing = clothing.([]*ClothingItem)
		response := struct {
			Success bool `json:"success"`
			Data    any  `json:"data"`
		}{
			Success: true,
			Data:    "Price updated successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}).Methods("POST")

	c.Router.HandleFunc("/upload/clothing/random", func(w http.ResponseWriter, r *http.Request) {
		componentType := r.URL.Query().Get("componentType")
		componentId := r.URL.Query().Get("componentId")
		rate := r.URL.Query().Get("rate")
		fmt.Printf("componentType: %s, componentId: %s, rate: %s\n", componentType, componentId, rate)
		response := struct {
			Success bool `json:"success"`
			Data    any  `json:"data"`
		}{
			Success: false,
		}
		if componentType == "" || componentId == "" || rate == "" {
			http.Error(w, "Missing required parameters", http.StatusBadRequest)
			return
		}
		rateInt, err := strconv.Atoi(rate)
		if err != nil {
			http.Error(w, "Invalid rate parameter", http.StatusBadRequest)
			return
		}
		componentIdInt, err := strconv.Atoi(componentId)
		if err != nil {
			http.Error(w, "Invalid componentId parameter", http.StatusBadRequest)
			return
		}
		rd1, err := random.IntRange(0, 100)
		if err != nil {
			http.Error(w, "Error generating random number", http.StatusInternalServerError)
			return
		}
		if rd1 > rateInt {
			response.Success = false
			response.Data = "Rate not met"
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		var item *ClothingItem
		var items []*ClothingItem
		totalPrice := 0.0
		minPrice := math.MaxFloat64
		maxPrice := 0.0
		for _, clothingItem := range c.CachedClothing {
			if clothingItem.ComponentType == componentType && clothingItem.ComponentId == componentIdInt {
				if clothingItem.Price > 0 {
					items = append(items, clothingItem)
					totalPrice += clothingItem.Price
					if clothingItem.Price < minPrice {
						minPrice = clothingItem.Price
					}
					if clothingItem.Price > maxPrice {
						maxPrice = clothingItem.Price
					}
				}
			}
		}
		minMax := maxPrice - minPrice
		rd, err := random.IntRange(0, int(totalPrice))
		if err != nil {
			http.Error(w, "Error generating random number", http.StatusInternalServerError)
			return
		}
		curRate := 0.0
		for _, clothingItem := range items {
			if math.Abs(minMax-clothingItem.Price)+curRate >= float64(rd) {
				item = clothingItem
				break
			}
			curRate += math.Abs(minMax - clothingItem.Price)
		}
		if item == nil {
			http.Error(w, "Item not found", http.StatusNotFound)
			return
		}
		response.Success = true
		response.Data = item
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	}).Methods("GET")

	c.Router.HandleFunc("/upload/clothing/{hash}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		hash := vars["hash"]
		var item *ClothingItem
		for _, clothingItem := range c.CachedClothing {
			if clothingItem.Hash == hash {
				item = clothingItem
				break
			}
		}
		if item == nil {
			http.Error(w, "Item not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(item)
	}).Methods("GET")

	c.Router.HandleFunc("/upload-buffer", c.UploadBuffer).Methods("POST")
	c.Router.HandleFunc("/static/{file}", c.GetStaticFile).Methods("GET")
}

func generateTextureHash(
	collectionName string,
	componentType string,
	componentId string,
	drawableId string,
	textureId string,
	gender string,
) string {
	// Tạo chuỗi kết hợp các thuộc tính của texture
	if collectionName == "" {
		collectionName = "default"
	}
	textureString := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		collectionName,
		componentType,
		componentId,
		drawableId,
		textureId,
		strings.TrimSpace(gender), // Loại bỏ khoảng trắng nếu gender rỗng
	)
	// Tạo hash SHA-256 từ chuỗi
	hasher := sha256.New()
	hasher.Write([]byte(textureString))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	// Lấy 16 ký tự đầu tiên của hash
	if len(hashString) > 16 {
		return hashString[:16]
	}
	return hashString
}

func (c *UploadController) GetClothingPrices() ([]ClothingPrice, error) {
	response := []ClothingPrice{}
	rows, err := c.DB.Query("SELECT hash, price FROM texture_prices")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ClothingPrice
		err := rows.Scan(&item.Hash, &item.Price)
		if err != nil {
			return nil, err
		}
		response = append(response, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *UploadController) GetClothingPrice(hash string) (float64, error) {
	var price float64
	err := c.DB.QueryRow("SELECT price FROM texture_prices WHERE hash = ?", hash).Scan(&price)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil // Không tìm thấy giá
		}
		return 0, err // Lỗi khác
	}
	return price, nil
}

func mustParseInt(s string) int {
	// Base 10, 64-bit (có thể thay bằng 32 nếu cần)
	n, _ := strconv.ParseInt(s, 10, 64)
	return int(n)
}

func (c *UploadController) GetClothing(collection string, returnSet bool) (interface{}, error) {
	files, err := os.ReadDir(c.Config.App.UploadPath)
	if err != nil {
		return nil, err
	}
	if returnSet {
		prices, err := c.GetClothingPrices()
		if err != nil {
			return nil, err
		}
		priceMap := make(map[string]float64)
		for _, price := range prices {
			priceMap[price.Hash] = price.Price
		}
		// Create a map to store the clothing items
		response := []*ClothingItem{}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			// filename Male_Apt01_male_component_11_6_1.webp

			ext := strings.ToLower(filepath.Ext(file.Name()))
			if ext != ".webp" {
				continue
			}
			// Get the file name without the extension
			name := removedExt(file.Name())

			// Split the file name into parts
			parts := strings.Split(name, "-")
			if len(parts) < 6 {
				continue
			}

			textureId := parts[len(parts)-1]
			drawableId := parts[len(parts)-2]
			componentId := parts[len(parts)-3]
			componentType := parts[len(parts)-4]
			gender := parts[len(parts)-5]
			collectionName := parts[len(parts)-6]

			if collection != "null" && collection != collectionName {
				continue
			}

			fi, err := file.Info()

			if err != nil {
				continue
			}

			if fi.Size() < 1024*4 {
				if componentType != "prop" && componentId != "2" && componentId != "7" {
					continue
				}
				if fi.Size() == 3904 {
					continue
				}
				if componentType == "component" && componentId == "7" && fi.Size() < 1024 {
					continue
				}
			}
			price := 0.0
			hash := generateTextureHash(collectionName, componentType, componentId, drawableId, textureId, gender)
			if priceValue, ok := priceMap[hash]; ok {
				price = priceValue
			}
			intGender := 0
			if gender == "0" {
				intGender = 0
			} else {
				intGender = 1
			}
			item := ClothingItem{
				CollectionName: collectionName,
				Gender:         intGender,
				ComponentType:  componentType,
				ComponentId:    mustParseInt(componentId),
				DrawableId:     mustParseInt(drawableId),
				TextureId:      mustParseInt(textureId),
				Size:           int(fi.Size()),
				Hash:           hash,
				Price:          price,
			}
			response = append(response, &item)
		}
		return response, nil
	} else {

		response := UploadManifestResponse{
			CollectionNum: 0,
			Collections:   []UploadManifestCollection{},
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			// filename Male_Apt01_male_component_11_6_1.webp

			ext := strings.ToLower(filepath.Ext(file.Name()))
			if ext != ".webp" {
				continue
			}
			// Get the file name without the extension
			name := removedExt(file.Name())

			// Split the file name into parts
			parts := strings.Split(name, "-")
			if len(parts) < 6 {
				continue
			}

			textureId := parts[len(parts)-1]
			drawableId := parts[len(parts)-2]
			componentId := parts[len(parts)-3]
			componentType := parts[len(parts)-4]
			gender := parts[len(parts)-5]
			collectionName := parts[len(parts)-6]

			if collection != "null" && collection != collectionName {
				continue
			}

			fi, err := file.Info()

			if err != nil {
				continue
			}

			if fi.Size() < 1024*4 {
				if componentType != "prop" && componentId != "2" && componentId != "7" {
					continue
				}
				if fi.Size() == 3904 {
					continue
				}
				if componentType == "component" && componentId == "7" && fi.Size() < 1024 {
					continue
				}
			}

			// Check if the collection already exists
			var collection *UploadManifestCollection
			for i := range response.Collections {
				if response.Collections[i].CollectionName == collectionName {
					collection = &response.Collections[i]
					break
				}
			}

			if collection == nil {
				collection = &UploadManifestCollection{
					CollectionName: collectionName,
					Items:          []UploadManifestCollectionItem{},
				}
				response.Collections = append(response.Collections, *collection)
				response.CollectionNum++
			}

			// Check if the item already exists
			var item *UploadManifestCollectionItem
			for i := range collection.Items {
				if collection.Items[i].ComponentId == componentId && collection.Items[i].DrawableId == drawableId && collection.Items[i].Gender == gender && collection.Items[i].ComponentType == componentType {
					item = &collection.Items[i]
					break
				}
			}

			url := utils.JoinURL(c.Config.App.BaseUrl, "static", file.Name())

			texture := UploadManifestCollectionItemTexture{
				TextureId: textureId,
				Name:      name,
				Url:       url,
				Size:      int(fi.Size()),
				Hash:      generateTextureHash(collectionName, componentType, componentId, drawableId, textureId, gender),
			}

			if item == nil {
				item = &UploadManifestCollectionItem{
					CollectionName: collectionName,
					Gender:         gender,
					ComponentType:  componentType,
					ComponentId:    componentId,
					DrawableId:     drawableId,
					Name:           name,
					Url:            url,
					Textures:       []UploadManifestCollectionItemTexture{texture},
				}
				collection.Items = append(collection.Items, *item)
			}

			item.Textures = append(item.Textures, texture)

		}
		return response, nil
	}

}

func (c *UploadController) GetUploadManifest(w http.ResponseWriter, r *http.Request) {

	collection := r.URL.Query().Get("collection")
	asSet := r.URL.Query().Get("set")
	fromPrice := r.URL.Query().Get("price_from")
	var response interface{}
	var err error
	if asSet == "true" {
		if fromPrice != "" {
			fromPriceFloat, err := strconv.ParseFloat(fromPrice, 64)
			if err != nil {
				http.Error(w, "Invalid price parameter", http.StatusBadRequest)
				return
			}
			response := []*ClothingItem{}
			for _, clothingItem := range c.CachedClothing {
				if clothingItem.Price >= fromPriceFloat {
					response = append(response, clothingItem)
				}
			}
			json.NewEncoder(w).Encode(response)
			return
		} else {
			response = c.CachedClothing
		}
	} else {

		response, err = c.GetClothing(collection, false)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	json.NewEncoder(w).Encode(response)
}

func (c *UploadController) GetStaticFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	file := vars["file"]
	filePath := utils.JoinURL(c.Config.App.UploadPath, file)
	http.ServeFile(w, r, filePath)
}

func (c *UploadController) DeleteStaticFile(w http.ResponseWriter, r *http.Request) {
	secret := r.Header.Get("Secret")
	if secret != c.Config.App.Secret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	file := vars["file"]
	filePath := utils.JoinURL(c.Config.App.UploadPath, file)
	err := os.Remove(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func removedExt(f string) string {
	return strings.TrimSuffix(f, filepath.Ext(f))
}

func (c *UploadController) Upload(w http.ResponseWriter, r *http.Request) {
	// max total size 20mb
	r.ParseMultipartForm(200 << 20)

	removeBackground := r.URL.Query().Get("rmbg")
	fileName := r.URL.Query().Get("name")

	if fileName == "" {
		http.Error(w, "File name is required", http.StatusBadRequest)
		return
	}

	secret := r.Header.Get("Secret")
	fmt.Printf("Secret: %s\n", secret)

	if secret != c.Config.App.Secret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	f, h, err := r.FormFile("file")
	if err != nil {
		fmt.Printf("Error reading file of 'image' form data. Reason: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	response := UploadResponse{
		Success:  true,
		FileName: h.Filename,
		Message:  "File uploaded successfully",
		Url:      utils.JoinURL(c.Config.App.BaseUrl, "static", h.Filename),
	}

	ext := strings.ToLower(filepath.Ext(h.Filename))
	switch ext {
	case ".png":
		// Decode the PNG image
		img, err := png.Decode(f)
		if err != nil {
			errStr := fmt.Sprintf("Error decoding the PNG file. Reason: %s\n", err)
			fmt.Println(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}
		if removeBackground == "true" {
			img, err = utils.RemoveGreenBackground(img)
			if err != nil {
				errStr := fmt.Sprintf("Error removing the green background. Reason: %s\n", err)
				fmt.Println(errStr)
				http.Error(w, errStr, http.StatusInternalServerError)
				return
			}
			/* img, err = utils.TrimImage(img)
			if err != nil {
				errStr := fmt.Sprintf("Error trimming the image. Reason: %s\n", err)
				fmt.Println(errStr)
				http.Error(w, errStr, http.StatusInternalServerError)
				return
			} */
		}
		// Create the WebP file
		webpFilePath := utils.JoinURL(c.Config.App.UploadPath, fileName+".webp")
		webpFile, err := os.Create(webpFilePath)
		if err != nil {
			errStr := fmt.Sprintf("Error in creating the WebP file. Reason: %s\n", err)
			fmt.Println(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}
		defer webpFile.Close()

		// Encode the image to WebP format and save it
		var buf bytes.Buffer
		if err := webp.Encode(&buf, img, nil); err != nil {
			errStr := fmt.Sprintf("Error encoding the image to WebP format. Reason: %s\n", err)
			fmt.Println(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}

		if _, err := webpFile.Write(buf.Bytes()); err != nil {
			errStr := fmt.Sprintf("Error writing the WebP file. Reason: %s\n", err)
			fmt.Println(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}
		response.Url = utils.JoinURL(c.Config.App.BaseUrl, "static", fileName+".webp")

	case ".webp", ".jpg", ".jpeg":
		// Save the file directly
		saveFilePath := utils.JoinURL(c.Config.App.UploadPath, fileName+ext)
		saveFile, err := os.Create(saveFilePath)
		if err != nil {
			errStr := fmt.Sprintf("Error in creating the file. Reason: %s\n", err)
			fmt.Println(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}
		defer saveFile.Close()

		if _, err := f.Seek(0, 0); err != nil {
			errStr := fmt.Sprintf("Error seeking the file. Reason: %s\n", err)
			fmt.Println(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}

		if _, err := io.Copy(saveFile, f); err != nil {
			errStr := fmt.Sprintf("Error saving the file. Reason: %s\n", err)
			fmt.Println(errStr)
			http.Error(w, errStr, http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, "Only PNG, JPG, and WebP files are allowed", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

}

func (c *UploadController) UploadBuffer(w http.ResponseWriter, r *http.Request) {
	// max total size 20mb
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)

	secret := r.Header.Get("Secret")

	if secret != c.Config.App.Secret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fmt.Printf("Body: %s\n", r.Body)

	fileName := r.Header.Get("FileName")

	ext := strings.ToLower(filepath.Ext(fileName))

	response := UploadResponse{
		Success:  true,
		FileName: fileName,
		Message:  "File uploaded successfully",
		Url:      utils.JoinURL(c.Config.App.BaseUrl, "static", fileName),
	}

	// Decode the PNG image
	img, err := png.Decode(r.Body)
	if err != nil {
		errStr := fmt.Sprintf("Error decoding the PNG file. Reason: %s\n", err)
		fmt.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}

	// Create the WebP file
	webpFilePath := utils.JoinURL(c.Config.App.UploadPath, strings.TrimSuffix(fileName, ext)+".webp")
	webpFile, err := os.Create(webpFilePath)
	if err != nil {
		errStr := fmt.Sprintf("Error in creating the WebP file. Reason: %s\n", err)
		fmt.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}
	defer webpFile.Close()

	// Encode the image to WebP format and save it
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, nil); err != nil {
		errStr := fmt.Sprintf("Error encoding the image to WebP format. Reason: %s\n", err)
		fmt.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}

	if _, err := webpFile.Write(buf.Bytes()); err != nil {
		errStr := fmt.Sprintf("Error writing the WebP file. Reason: %s\n", err)
		fmt.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}
	response.Url = utils.JoinURL(c.Config.App.BaseUrl, "static", strings.TrimSuffix(fileName, ext)+".webp")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
