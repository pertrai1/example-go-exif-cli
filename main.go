package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
    "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/barasher/go-exiftool"
)

// Convert DMS (Degrees, Minutes, Seconds) coordinates to decimal degrees
func convertDMSToDecimal(dms string, direction string) (float64, error) {
	parts := strings.FieldsFunc(dms, func(r rune) bool {
		return r == ' ' || r == 'd' || r == 'e' || r == 'g' || r == '\'' || r == '"'
	})

	degrees, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}

	minutes, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}

	decimal := degrees + minutes/60 + seconds/3600

	if direction == "S" || direction == "W" {
		decimal = -decimal
	}
	return decimal, nil
}

// Perform reverse geocoding using the provided longitude and latitude
func reverseGeocode(longitude, latitude float64) (string, error) {
	// Format the location JSON
	locationJSON := fmt.Sprintf(`{"x": %f, "y": %f}`, longitude, latitude)

	// Define the ArcGIS reverse geocode API URL
	baseURL := "https://geocode.arcgis.com/arcgis/rest/services/World/GeocodeServer/reverseGeocode"

	// Encode the location JSON as URL query parameter
	params := fmt.Sprintf("?location=%s&f=json&langCode=en", url.QueryEscape(locationJSON))

	// Send HTTP GET request to the ArcGIS API
	response, err := http.Get(baseURL + params)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	// Format the response body for readability
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
	writer.Write([]string{"Filename", "Raw Latitude", "Raw Longitude", "Formatted Latitude", "Formatted Longitude", "Reverse Geocode Response"})

	for _, file := range files {
		filePath := filepath.Join(dirPath, file.Name())
		if !file.IsDir() && isSupportedImageFile(file.Name()) {
			fileMetadatas := et.ExtractMetadata(filePath)
			for _, fileInfo := range fileMetadatas {
				if fileInfo.Err != nil {
					log.Printf("Error concerning %s: %v", fileInfo.File, fileInfo.Err)
					continue
				}

				rawLatitude := safeString(fileInfo.Fields, "GPSLatitude")
				rawLongitude := safeString(fileInfo.Fields, "GPSLongitude")
				latDirection := safeString(fileInfo.Fields, "GPSLatitudeRef")
				lonDirection := safeString(fileInfo.Fields, "GPSLongitudeRef")
				formattedLatitude, err := convertDMSToDecimal(rawLatitude, latDirection)
				if err != nil {
					log.Printf("Error converting latitude for %s: %v", fileInfo.File, err)
					continue
				}
				formattedLongitude, err := convertDMSToDecimal(rawLongitude, lonDirection)
				if err != nil {
					log.Printf("Error converting longitude for %s: %v", fileInfo.File, err)
					continue
				}
				apiResponse, err := reverseGeocode(formattedLongitude, formattedLatitude)
				if err != nil {
					log.Printf("Reverse geocoding failed for %s: %v", fileInfo.File, err)
					continue
				}
				writer.Write([]string{file.Name(), rawLatitude, rawLongitude, fmt.Sprintf("%f", formattedLatitude), fmt.Sprintf("%f", formattedLongitude), apiResponse})
			}
		}
	}
}

func isSupportedImageFile(fileName string) bool {
	lowerFileName := strings.ToLower(fileName)
	return strings.HasSuffix(lowerFileName, ".jpg") || strings.HasSuffix(lowerFileName, ".jpeg") || strings.HasSuffix(lowerFileName, ".tif") || strings.HasSuffix(lowerFileName, ".tiff")
}

func safeString(fields map[string]interface{}, key string) string {
	if val, ok := fields[key]; ok && val != nil {
		return fmt.Sprint(val)
	}
	return ""
}
