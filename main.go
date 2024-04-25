package main

import (
    "encoding/csv"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "path/filepath"

    "github.com/barasher/go-exiftool"
)

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

    // Write CSV header
    writer.Write([]string{"Filename", "Latitude", "LatitudeRef", "Longitude", "LongitudeRef", "Altitude", "AltitudeRef"})

    for _, file := range files {
        filePath := filepath.Join(dirPath, file.Name())
        if !file.IsDir() && (filepath.Ext(file.Name()) == ".jpg" || filepath.Ext(file.Name()) == ".jpeg" || filepath.Ext(file.Name()) == ".tif" || filepath.Ext(file.Name()) == ".tiff") {
            fileInfos := et.ExtractMetadata(filePath)
            for _, fileInfo := range fileInfos {
                if fileInfo.Err != nil {
                    log.Printf("Error concerning %s: %v", fileInfo.File, fileInfo.Err)
                    continue
                }

                latitude := safeString(fileInfo.Fields, "GPSLatitude")
                latitudeRef := safeString(fileInfo.Fields, "GPSLatitudeRef")
                longitude := safeString(fileInfo.Fields, "GPSLongitude")
                longitudeRef := safeString(fileInfo.Fields, "GPSLongitudeRef")
                altitude := safeString(fileInfo.Fields, "GPSAltitude")
                altitudeRef := safeString(fileInfo.Fields, "GPSAltitudeRef")

                writer.Write([]string{file.Name(), latitude, latitudeRef, longitude, longitudeRef, altitude, altitudeRef})
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

