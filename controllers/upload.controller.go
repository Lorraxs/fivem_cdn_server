package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"lorraxs/fivem_cdn_server/config"
	"lorraxs/fivem_cdn_server/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/gorilla/mux"
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
	ctx    context.Context
	Router *mux.Router
	Config *config.Config
}

func NewUploadController() *UploadController {
	return &UploadController{}
}

func (c *UploadController) Init(ctx context.Context, router *mux.Router) {
	c.ctx = ctx
	c.Router = router
	c.Config = config.GetConfig()
	c.Router.HandleFunc("/static/{file}", c.DeleteStaticFile).Methods("DELETE")
	c.Router.HandleFunc("/upload", c.Upload).Methods("POST")
	c.Router.HandleFunc("/upload/manifest", c.GetUploadManifest).Methods("GET")
	c.Router.HandleFunc("/upload-buffer", c.UploadBuffer).Methods("POST")
	c.Router.HandleFunc("/static/{file}", c.GetStaticFile).Methods("GET")
}

func (c *UploadController) GetUploadManifest(w http.ResponseWriter, r *http.Request) {

	collection := r.URL.Query().Get("collection")

	response := UploadManifestResponse{
		CollectionNum: 0,
		Collections:   []UploadManifestCollection{},
	}

	files, err := os.ReadDir(c.Config.App.UploadPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

		/* item := UploadManifestCollectionItem{
			CollectionName: collectionName,
			Gender:         gender,
			ComponentType:  componentType,
			ComponentId:    componentId,
			DrawableId:     drawableId,
			TextureId:      textureId,
			Name:           name,
			Url:            path.Join(c.Config.App.BaseUrl, "static", file.Name()),
		}

		collection.Items = append(collection.Items, item) */

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
