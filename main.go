package main

import (
    "bytes"
    "encoding/csv"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"

    "github.com/barasher/go-exiftool"
)

func reverseGeocode(longitude, latitude float64) (string, error) {
    baseURL := "https://geocode.arcgis.com/arcgis/rest/services/World/GeocodeServer/reverseGeocode"
    params := fmt.Sprintf("?location=%f,%f&outSR=4326&f=json", longitude, latitude)
    response, err := http.Get(baseURL + params)
    if err != nil {
        return "", err
    }
    defer response.Body.Close()

    body, err := ioutil.ReadAll(response.Body)
    if err != nil {
        return "", err
    }

    var prettyJSON bytes.Buffer
    if err := json.Indent(&prettyJSON, body, "", "\t"); err != nil {
        return "", err
    }

    return prettyJSON.String(), nil
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: go run main.go <path_to_directory>")
        os.Exit(1)
    }
    dirPath := os.Args[1]

    files, err := ioutil.ReadDir(dirPath)
    if err != nil {
        log.Fatal(err)
    }

    et, err := exiftool.NewExiftool()
    if err != nil {
        log.Fatalf("Error when initializing: %v", err)
    }
    defer et.Close()

    csvFile, err := os.Create("output.csv")
    if err != nil {
        log.Fatalf("Failed creating file: %s", err)
    }
    defer csvFile.Close()

    writer := csv.NewWriter(csvFile)
    defer writer.Flush()
    writer.Write([]string{"Filename", "Reverse Geocode Response"})

    for _, file := range files {
        filePath := filepath.Join(dirPath, file.Name())
        fileExt := strings.ToLower(filepath.Ext(file.Name()))
        if !file.IsDir() && (fileExt == ".jpg" || fileExt == ".jpeg" || fileExt == ".tif" || fileExt == ".tiff") {
            fileMetadatas := et.ExtractMetadata(filePath)
            for _, fileInfo := range fileMetadatas {
                if fileInfo.Err != nil {
                    log.Printf("Error concerning %s: %v", fileInfo.File, fileInfo.Err)
                    continue
                }

                latitude, _ := strconv.ParseFloat(safeString(fileInfo.Fields, "GPSLatitude"), 64)
                longitude, _ := strconv.ParseFloat(safeString(fileInfo.Fields, "GPSLongitude"), 64)
                apiResponse, err := reverseGeocode(longitude, latitude)
                if err != nil {
                    log.Printf("Reverse geocoding failed for %s: %v", fileInfo.File, err)
                    continue
                }
                writer.Write([]string{file.Name(), apiResponse})
            }
        }
    }
}

func safeString(fields map[string]interface{}, key string) string {
    if val, ok := fields[key]; ok && val != nil {
        return fmt.Sprint(val)
    }
    return ""
}

