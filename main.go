package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
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

	// Debugging: Print original direction and calculated decimal value
	fmt.Printf("Original direction: %s\n", direction)
	fmt.Printf("Decimal value before direction adjustment: %f\n", decimal)

	if strings.Contains(strings.ToUpper(direction), "S") || strings.Contains(strings.ToUpper(direction), "W") {
		decimal = -decimal
	}

	// Debugging: Print adjusted decimal value
	fmt.Printf("Decimal value after direction adjustment: %f\n", decimal)

	return math.Round(decimal*1e6) / 1e6, nil
}

// Determine the sign of the longitude based on Exif metadata
func determineLongitudeSign(fields map[string]interface{}) int {
	if dir, ok := fields["GPSLongitudeRef"].(string); ok {
		fmt.Println("Longitude Direction:", dir) // Debugging output
		if strings.ToUpper(dir) == "W" {
			return -1 // West direction implies negative longitude
		}
	}
	return 1 // Default to positive longitude if direction is not explicitly specified
}

// Perform reverse geocoding using the provided latitude and longitude
func reverseGeocode(latitude, longitude float64) (string, error) {
	// Format the location JSON with latitude and longitude
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
				lonSign := determineLongitudeSign(fileInfo.Fields)

				// Adjust longitude sign directly
				formattedLongitude *= float64(lonSign)

				apiResponse, err := reverseGeocode(formattedLatitude, formattedLongitude)
				if err != nil {
					log.Printf("Error during reverse geocoding for %s: %v", fileInfo.File, err)
					continue
				}

				// Create a formatted string for longitude with the negative sign
				lonStr := strconv.FormatFloat(math.Abs(formattedLongitude), 'f', -1, 64)
				if lonSign == -1 {
					lonStr = "-" + lonStr
				}

				writer.Write([]string{fileInfo.File, rawLatitude, rawLongitude, strconv.FormatFloat(formattedLatitude, 'f', -1, 64), lonStr, apiResponse})
			}
		}
	}
}

// Check if the file is a supported image file based on its extension
func isSupportedImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".tiff" || ext == ".bmp"
}

// Safely retrieve a string value from a map, returning an empty string if the key is not found or the value is not a string
func safeString(fields map[string]interface{}, key string) string {
	if val, ok := fields[key].(string); ok {
		return val
	}
	return ""
}
