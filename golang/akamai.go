package csg

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"math/rand"
	"os"
	"strings"
	"time"
)

const AkamaiSensorDataDefaultReuseInterval = 1200
const AkamaiSensorSuccess = "success"

type AkamaiSensorData struct {
	ItemList []*AkamaiSensorDataItem `yaml:"item_list"`
}

type AkamaiSensorDataItem struct {
	Site       string `yaml:"site"`
	SensorDest string `yaml:"sensor_dest"`
	SensorData string `yaml:"sensor_data"`
	UserAgent  string
	lastUsed   int64
}

func ParseAkamaiSensorData(filePath string) *AkamaiSensorData {
	data := new(AkamaiSensorData)

	file, err := os.Open(filePath)
	if err != nil {
		panic(err) //if these files don't exist, something is horribly wrong with the execution bundle
	}
	defer file.Close()

	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		panic(err)
	}

	for _, item := range data.ItemList {
		item.lastUsed = 0

		split := strings.Split(item.SensorData, ",")
		if len(split) < 10 {
			// malformed configuration, just fail fast
			panic(fmt.Errorf("AkamaiSensorData: Malformed sensor data: %s", item.SensorData))
		}

		item.UserAgent = split[4]
	}

	return data
}

func (asd *AkamaiSensorData) GetSensorData() (*AkamaiSensorDataItem, error) {
	return asd.GetSensorDataWithReuseInterval(AkamaiSensorDataDefaultReuseInterval)
}

func (asd *AkamaiSensorData) GetSensorDataWithReuseInterval(reuseInterval int64) (*AkamaiSensorDataItem, error) {
	SeedRand()

	currentTime := time.Now().Unix()
	available := make([]int, 0)

	for i, item := range asd.ItemList {
		if (currentTime - item.lastUsed) > reuseInterval {
			available = append(available, i)
		}
	}

	Log.Debugf("Sensor data available: %d/%d", len(available), len(asd.ItemList))

	if len(available) == 0 {
		return nil, fmt.Errorf("AkamaiSensorData: Out of valid sensor data!")
	}

	idx := available[rand.Intn(len(available))]
	Log.Debugf("Found available sensor data at index %d", idx)
	sensorDataItem := asd.ItemList[idx]

	return sensorDataItem, nil
}

func (sdi *AkamaiSensorDataItem) MarkUsed() {
	sdi.lastUsed = time.Now().Unix()
}
