package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
)

func doTheThings(){
	defer func(){
    if r := recover(); r != nil {
			if abort == true {
				monitorOutput.unshift("Process aborted.")
			} else {
				monitorOutput.unshift("ERROR: " + r.(string))
			}
			cleanup()
		}
  }()
	locked = true
	loadConfig()
  connectToDb()
	if mode == "getData" {
		downloadData()
	} else if mode == "sendAds" {
		sendAds(0)
	} else if mode == "getThenSend" {
		downloadData()
		sendAds(0)
	} else {
		panic(errors.New("Mode not set!"))
	}
	cleanup()
}

func cleanup(){
	sendCounter = 0
	locked = false
	abort = false
	monitorOutput.unshift("AdPusher has finished.")
	monitorOutput.unshift("AdPusher is ready.")
	monitorOutput.unshift("PAUSE")
}

func loadConfig(initialize ...bool) {
	configFile, err := os.Open("adPusherConfig.json")
	if err != nil {
		panic(errors.New("Config file not found."))
	}
	var holder interface{}
	err = json.NewDecoder(configFile).Decode(&holder)
	if err != nil {
		panic(errors.New("Config file malformed."))
	}
	if len(initialize) == 1 && initialize[0] == true{
		parseConfig(holder, true)
	} else {
		parseConfig(holder)
	}
}

func parseConfig(holder interface{}, initialize ...bool) {
	defer func() {
		if r := recover(); r != nil {
			panic(errors.New("Required fields missing from config file."))
		}
	}()
	if len(initialize) == 1 && initialize[0] == true{
		common := holder.(map[string]interface{})["common"].(map[string]interface{})
		configVars["OS_csv_endpoint"] = common["OS_csv_endpoint"].(string)
		configVars["OS_ad_endpoint"] = common["OS_ad_endpoint"].(string)
		configVars["PI_stripe_endpoint"] = common["PI_stripe_endpoint"].(string)
		configVars["cs_esp"] = common["cs_esp"].(string)
		configVars["cs_containertype"] = common["cs_containertype"].(string)
		configVars["CSV_file_location"] = common["CSV_file_location"].(string)
		configVars["database_credentials"] = common["database_credentials"].(string)
		configVars["send_threads"] = common["send_threads"].(string)
		configVars["localhost"] = common["localhost"].(string)
		configVars["serverRoot"] = common["serverRoot"].(string)
	} else {
		specific := holder.(map[string]interface{})[prop].(map[string]interface{})
		configVars["OS_APIKey"] = specific["OS_APIKey"].(string)
		configVars["OS_AppID"] = specific["OS_AppID"].(string)
		configVars["cs_stripeid"] = specific["cs_stripeid"].(string)
		configVars["cs_token"] = specific["cs_token"].(string)
		configVars["cs_page"] = specific["cs_page"].(string)
		configVars["cs_sendid"] = strconv.FormatInt(time.Now().Unix(), 10)
	}
}

func connectToDb() {
	fmt.Println(configVars["database_credentials"])
	database, err := sql.Open("mysql", configVars["database_credentials"])
	if err != nil {
		panic(errors.New("Could not connect to database."))
	}
	db = *database
}

func downloadData() {
	monitorOutput.unshift("Attempting to fetch url for " + prop + " csv from OneSignal.")
	csv_file_url := getFileURL()
	monitorOutput.unshift("Successfully fetched url for " + prop + " csv from OneSignal. Attempting to download data file.")
	getFile(csv_file_url, 0)
	monitorOutput.unshift("Successfully downloaded OneSignal data file for " + prop + ".")
	parseFile()
	monitorOutput.unshift("OneSignal data for " + prop + " copied to database.")
}

func getFileURL() string {
	oSargs := OSargs{
		App_id:       configVars["OS_AppID"],
		Extra_fields: []string{"ip"},
	}
	OSargsJSON, err := json.Marshal(oSargs)

	svcPostRequest, err := http.NewRequest("POST", configVars["OS_csv_endpoint"], strings.NewReader(string(OSargsJSON)))
	svcPostRequest.Header.Set("Authorization", "Basic "+configVars["OS_APIKey"])
	svcPostRequest.Header.Set("Content-Type", "application/json")

	dataUrlResponse, err := myClient.Do(svcPostRequest)
	defer dataUrlResponse.Body.Close()
	var jsonResponseMap map[string]string
	err = json.NewDecoder(dataUrlResponse.Body).Decode(&jsonResponseMap)
	if err != nil {
		panic(errors.New("Could not fetch OneSignal data file url."))
	}
	return jsonResponseMap["csv_file_url"]
}

func getFile(url string, attempts int) {
	monitorOutput.unshift("WAIT", true)
	attempts++

	localFile, err := os.Create(configVars["CSV_file_location"])
	defer localFile.Close()

	remoteFile, err := http.Get(url)
	defer remoteFile.Body.Close()

	io.Copy(localFile, remoteFile.Body)
	fileInfo, err := os.Stat(configVars["CSV_file_location"])

	if abort == true {
		panic(errors.New("Download aborted."))
	}

	if attempts > 30 || err != nil {
		panic(errors.New("Could not download OneSignal data file."))
	} else if fileInfo.Size() < 1000 {
		time.Sleep(time.Second * 10)
		getFile(url, attempts)
	}
}

func parseFile() {
	monitorOutput.unshift("Attempting to copy OneSignal data to database.")
	openedFile, err := os.Open(configVars["CSV_file_location"])
	openedGZ, err := gzip.NewReader(openedFile)
	csvReader := csv.NewReader(openedGZ)
	csvHeader, err := csvReader.Read()
	csvFields, err := csvReader.ReadAll()
	if err != nil {
		panic(errors.New("Failed to open OneSignal data file."))
	}
	var csvKeys = make(map[string]int)

	for i := 0; i < len(csvHeader); i++ {
		switch csvHeader[i] {
		case "id":
			csvKeys["id"] = i
		case "ip":
			csvKeys["ip"] = i
		case "device_model":
			csvKeys["device_model"] = i
		case "invalid_identifier":
			csvKeys["invalid_identifier"] = i
		case "device_type":
			csvKeys["device_type"] = i
		case "device_os":
			csvKeys["device_os"] = i
		}
	}
	db.Query("TRUNCATE OSusers")

	for i := 0; i < len(csvFields); i++ {
		if csvFields[i][csvKeys["invalid_identifier"]] == "f" {
			OSid := csvFields[i][csvKeys["id"]]
			ip := csvFields[i][csvKeys["ip"]]
			device_model := csvFields[i][csvKeys["device_model"]]
			device_type := csvFields[i][csvKeys["device_type"]]
			device_os := csvFields[i][csvKeys["device_os"]]
			insertQuery := "INSERT INTO OSusers (OSid, IP, device_model, device_type, device_os) VALUES ('" + OSid + "','" + ip + "','" + device_model + "','" + device_type + "','" + device_os + "')"
			row, err := db.Query(insertQuery)
			if err != nil {
				panic( errors.New("Error writing to database.") )
			}
			row.Close()
		}
	}
}

func sendAds(tryNum int) func(int) {
	monitorOutput.unshift("Retreiving records from database.")

	rowsPerThread, numThreads := countRecords()

	var wg sync.WaitGroup
	wg.Add(numThreads)
	monitorOutput.unshift("Starting ad send.")
	for i := 0; i < numThreads; i++ {
		go sendThread(rowsPerThread, i, &wg)
	}
	wg.Wait()

	if len(errLog) > 0 {
		tryNum++
		if tryNum >= 10 {
			panic(errors.New("Could not send all ads due to errors. Please check database."))
		}
		monitorOutput.unshift("Errors on sends: " + strings.Join(errLog, ",") + " - Retrying ("+strconv.Itoa(tryNum)+").")
		errLog = errLog[:0]
		return sendAds(tryNum)
	}
	wereSentPhrase := " ads were sent."
	if sendCounter == 1 {
		wereSentPhrase = " ad was sent."
	}
	monitorOutput.unshift("Send finished. " + strconv.Itoa(sendCounter) + wereSentPhrase)
	return nil
}

func sendThread(rowsPerThread int, i int, wg *sync.WaitGroup) {
	defer wg.Done()
	getListQuery := "SELECT OSid, device_model, device_type, device_os, IP FROM OSusers WHERE lastSent < DATE_SUB(CURRENT_TIMESTAMP, INTERVAL 1 DAY) OR lastSent IS NULL LIMIT " + strconv.Itoa(rowsPerThread) + " OFFSET " + strconv.Itoa(rowsPerThread*i)
	rows, err := db.Query(getListQuery)
	if err != nil {
		panic(errors.New("Could not retreive records from database on thread " + strconv.Itoa(i) + "."))
	}
	for rows.Next() {
		if abort == true{
			panic(errors.New("Send aborted."))
		}
		var uR userRecord
		rows.Scan(&uR.OSid, &uR.Device_model, &uR.Device_type, &uR.Device_os, &uR.IP)
		adData, err := fetchAdData(uR)
		if err == nil {
			sendResponse, err := sendAd(uR, adData)
			if err == nil {
				recordSend(sendResponse, uR)
			} else {
				errLog = append(errLog, "Failed to send ad for "+uR.OSid)
			}
		} else {
			errLog = append(errLog, err.Error())
		}
	}
}

func countRecords() (int, int) {
	countQuery := "SELECT COUNT(*) FROM OSusers WHERE lastSent < DATE_SUB(CURRENT_TIMESTAMP, INTERVAL 1 DAY) OR lastSent IS NULL"
	numRowsRow := db.QueryRow(countQuery)
	type numStruct struct {
		NumRows int
	}
	var numRowsObj numStruct
	numRowsRow.Scan(&numRowsObj.NumRows)
	numThreads64, _ := strconv.ParseFloat(configVars["send_threads"], 64)
	rowsPerThread := int(math.Ceil(float64(numRowsObj.NumRows) / numThreads64))
	numThreads := int(numThreads64)
	return rowsPerThread, numThreads
}

func fetchAdData(uR userRecord) (interface{}, error) {
	getVars := url.Values{
		"cs_esp":           []string{"powerinbox_web"},
		"cs_containertype": []string{"web"},
		"cs_stripeid":      []string{configVars["cs_stripeid"]},
		"cs_token":         []string{configVars["cs_token"]},
		"cs_page":          []string{configVars["cs_page"]},
		"cs_sendid":				[]string{configVars["cs_sendid"]},
		"cs_email":         []string{uR.OSid},
		"cs_devicemodel":   []string{uR.Device_model},
		"cs_devicetype":    []string{uR.Device_type},
		"cs_osversion":     []string{uR.Device_os},
		"cs_ip":            []string{uR.IP},
	}
	powInbEndpoint := configVars["PI_stripe_endpoint"] + getVars.Encode()

	adResponse, err := myClient.Get(powInbEndpoint)
	var adData interface{}
	err = json.NewDecoder(adResponse.Body).Decode(&adData)
	defer adResponse.Body.Close()
	if adData == nil {
		err = errors.New("Failed to load ad for " + uR.OSid + ".")
	}
	return adData, err
}

func sendAd(uR userRecord, adData interface{}) (http.Response, error) {
	dataBase := adData.(map[string]interface{})["items"].([]interface{})[0].(map[string]interface{})

	imgURL := dataBase["imageurl"].(string)

	adObject := AdObject{
		App_id:             configVars["OS_AppID"],
		Include_player_ids: [1]string{uR.OSid},
		Contents:           map[string]string{"en": dataBase["subtitle"].(string)},
		Headings:           map[string]string{"en": "[AD] " + dataBase["title"].(string)},
		Url:                dataBase["redirecturl"].(string),
		Small_icon:         imgURL,
		Large_icon:         imgURL,
		Adm_small_icon:     imgURL,
		Adm_large_icon:     imgURL,
		Chrome_web_icon:    imgURL,
	}

	adObjectJSON, err := json.Marshal(adObject)

	sendAdsRequest, err := http.NewRequest("POST", configVars["OS_ad_endpoint"], strings.NewReader(string(adObjectJSON)))
	sendAdsRequest.Header.Set("Authorization", "Basic "+configVars["OS_APIKey"])
	sendAdsRequest.Header.Set("Content-Type", "application/json")

	sendResponse, err := myClient.Do(sendAdsRequest)

	return *sendResponse, err
}

func recordSend(sendResponse http.Response, uR userRecord) {
	defer sendResponse.Body.Close()
	var target interface{}
	err := json.NewDecoder(sendResponse.Body).Decode(&target)
	if err != nil || target == nil {
		errLog = append(errLog, "Error saving record of send for "+uR.OSid)
	} else {
		adSendId := target.(map[string]interface{})["id"].(string)
		sentQuery := "UPDATE OSusers SET lastSent = NOW(), sentData = '" + adSendId + "' WHERE OSid = '" + uR.OSid + "'"
		row, err := db.Query(sentQuery)
		if err == nil {
			row.Close()
			if sendCounter%50 == 0 {
				monitorOutput.unshift(strconv.Itoa(sendCounter))
			}
			sendCounter++
		} else {
			errLog = append(errLog, "Error saving record of send for "+uR.OSid)
		}
	}
}
