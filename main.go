package main

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	FILES_FOLDER = "files"
	DATA_FOLDER  = "info"
)

type File struct {
	Hash      string
	Extension string
}

func Compress(data []byte) []byte {
	compressed := new(bytes.Buffer)

	writer, err := flate.NewWriter(compressed, flate.BestCompression)
	if err != nil {
		panic(err)
		return nil
	}

	_, err = writer.Write(data)
	if err != nil {
		fmt.Println("Error compressing data:", err)
		return nil
	}

	err = writer.Close()
	if err != nil {
		fmt.Println("Error closing writer:", err)
		return nil
	}

	return compressed.Bytes()
}

func Decompress(compressed []byte) []byte {
	reader := bytes.NewReader(compressed)
	deflateReader := flate.NewReader(reader)
	data, err := io.ReadAll(deflateReader)
	if err != nil {
		fmt.Println("Error decompressing data:", err)
		return nil
	}
	return data
}

type Index struct {
	stat os.FileInfo
	data []byte
}

var index Index = Index{
	stat: nil,
	data: nil,
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	stat, _ := os.Stat("index.html")
	if index.stat.ModTime() != stat.ModTime() {
		index.data = make([]byte, stat.Size())
		index.data, _ = os.ReadFile("index.html")
		index.stat = stat
		fmt.Println("index page was reloaded")
	}
	w.Write(index.data)
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	tempfile, header, err := r.FormFile("upload")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer tempfile.Close()

	hash := sha256.New()
	file := new(bytes.Buffer)
	io.Copy(file, tempfile)

	hash.Write(file.Bytes())
	hash.Write([]byte(time.Now().String()))
	hashSum := fmt.Sprintf("%x", hash.Sum(nil)[:32])

	fileDescription := File{hashSum, header.Filename[strings.Index(header.Filename, "."):]}

	data, _ := os.OpenFile(DATA_FOLDER+"/"+hashSum, os.O_WRONLY|os.O_CREATE, 0666)
	json.NewEncoder(data).Encode(fileDescription)

	f, err := os.OpenFile(FILES_FOLDER+"/"+hashSum, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	compressed := bytes.NewReader(Compress(file.Bytes()))

	io.Copy(f, compressed)
	w.Write([]byte(hashSum))
}

func GetFileHandler(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Path[len("/get/"):]
	file, err := os.Open(FILES_FOLDER + "/" + hash)

	rawFileInfo, _ := os.ReadFile(DATA_FOLDER + "/" + hash)
	fileinfo := File{}
	err = json.Unmarshal(
		rawFileInfo,
		&fileinfo,
	)
	if err != nil {
		panic(err) //please dont
	}

	compressed, _ := io.ReadAll(file)
	decompressed := bytes.NewReader(Decompress(compressed))

	w.Header().Set("Content-Disposition", "attachment; filename="+fileinfo.Hash+"."+fileinfo.Extension)
	w.Header().Set("Content-Type", "application/octet-stream")

	io.Copy(w, decompressed)
}

func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err) // Возвращает true, если файл или папка существуют
}

func main() {
	if !checkFileExists(FILES_FOLDER) {
		os.Mkdir(FILES_FOLDER, 0666)
	}
	if !checkFileExists(DATA_FOLDER) {
		os.Mkdir(DATA_FOLDER, 0666)
	}

	index.stat, _ = os.Stat("index.html")
	index.data = make([]byte, index.stat.Size())
	index.data, _ = os.ReadFile("index.html")

	http.HandleFunc("/", IndexHandler)
	http.HandleFunc("/upload", UploadHandler)
	http.HandleFunc("/get/", GetFileHandler)
	http.ListenAndServe(":8080", nil)
}
