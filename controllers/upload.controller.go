package controllers

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"lorraxs/fivem_cdn_server/config"
	"net/http"
	"os"
	"path"
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
	c.Router.HandleFunc("/upload", c.Upload).Methods("POST")
}

func removedExt(f string) string {
	return strings.TrimSuffix(f, filepath.Ext(f))
}

func (c *UploadController) Upload(w http.ResponseWriter, r *http.Request) {
	// max total size 20mb
	r.ParseMultipartForm(200 << 20)

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

		// Create the WebP file
		webpFilePath := path.Join(c.Config.App.UploadPath, strings.TrimSuffix(h.Filename, ext)+".webp")
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

	case ".webp", ".jpg", ".jpeg":
		// Save the file directly
		saveFilePath := path.Join(c.Config.App.UploadPath, h.Filename)
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
	w.Write([]byte("File uploaded successfully"))
}
